import {CommonModule} from '@angular/common';
import {ChangeDetectionStrategy, Component, computed, DestroyRef, effect, ElementRef, inject, signal, ViewChild} from '@angular/core';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';
import {FormsModule} from '@angular/forms';
import {ActivatedRoute, Router, RouterLink} from '@angular/router';
import hljs from 'highlight.js';
import {ApiService} from '../api.service';
import {SessionStateService} from '../session-state.service';
import {ShadowStep} from '../types';

/**
 * ReviewPageComponent provides an interactive inspector for recorded action steps
 * and Playwright script generation and code editing.
 */
@Component({
  standalone: true,
  selector: 'app-review-page',
  imports: [CommonModule, FormsModule, RouterLink],
  templateUrl: './review-page.component.ng.html',
  styleUrl: './review-page.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class ReviewPageComponent {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly api = inject(ApiService);
  private readonly sessionState = inject(SessionStateService);
  private readonly destroyRef = inject(DestroyRef);

  @ViewChild('codeBlock') private codeBlock?: ElementRef<HTMLElement>;

  readonly workspaceId = signal('');
  readonly actionSetId = signal<string | null>(null);
  readonly actionSetName = signal('Untitled Shadow');
  readonly targetApp = signal('Google Ads');
  readonly steps = signal<ShadowStep[]>([]);
  readonly selectedStepIndex = signal<number>(1);
  readonly saving = signal(false);
  readonly error = signal<string | null>(null);
  readonly savedMessage = signal<string | null>(null);

  readonly playwrightScript = signal<string | null>(null);
  readonly generatingScript = signal(false);
  readonly savingScript = signal(false);
  readonly isEditingScript = signal(false);

  // Feedback fields
  readonly includeFeedback = signal(false);
  readonly feedbackError = signal('');
  readonly feedbackLogs = signal('');

  readonly selectedStep = computed(() =>
    this.steps().find((step) => step.stepIndex === this.selectedStepIndex()) ?? this.steps()[0] ?? null
  );

  readonly selectedStepJson = computed(() =>
    this.selectedStep() ? JSON.stringify(this.selectedStep(), null, 2) : ''
  );

  constructor() {
    effect(() => {
      const script = this.playwrightScript();
      if (script && !this.isEditingScript()) {
        setTimeout(() => {
          if (this.codeBlock?.nativeElement) {
            this.codeBlock.nativeElement.removeAttribute('data-highlighted');
            hljs.highlightElement(this.codeBlock.nativeElement);
          }
        }, 50);
      }
    });

    this.route.paramMap
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((params) => {
        const workspaceId = params.get('workspaceId');
        const actionSetId = params.get('actionSetId');

        if (actionSetId) {
          this.actionSetId.set(actionSetId);
          this.loadActionSet(actionSetId);
        } else if (workspaceId) {
          this.workspaceId.set(workspaceId);
          this.loadWorkspaceShadow(workspaceId);
        }
      });
  }

  loadWorkspaceShadow(workspaceId: string): void {
    const cachedSteps = this.sessionState.latestSteps();
    if (cachedSteps.length > 0) {
      this.steps.set(cachedSteps);
      return;
    }

    this.api.getShadowActions(workspaceId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (response) => {
          this.steps.set(response.steps ?? []);
        },
        error: (error) => {
          this.error.set(error?.error?.message ?? 'Failed to load recorded actions');
        }
      });
  }

  loadActionSet(actionSetId: string): void {
    this.api.getActionSet(actionSetId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (item) => {
          this.actionSetName.set(item.name);
          this.targetApp.set(item.target_app || 'Google Ads');
          const rawSteps = item.steps?.['steps'];
          if (Array.isArray(rawSteps)) {
            this.steps.set(rawSteps as ShadowStep[]);
          }
          if (item.playwrightScript) {
            this.playwrightScript.set(item.playwrightScript);
          }
        },
        error: (error) => {
          this.error.set(error?.error?.message ?? 'Failed to load action set');
        }
      });
  }

  selectStep(stepIndex: number): void {
    this.selectedStepIndex.set(stepIndex);
  }

  deleteStep(stepIndex: number): void {
    this.steps.update((current) => {
      const next = current.filter((step) => step.stepIndex !== stepIndex);
      return next.map((step, index) => ({...step, stepIndex: index + 1}));
    });
    if (this.selectedStepIndex() >= stepIndex && this.selectedStepIndex() > 1) {
      this.selectedStepIndex.set(this.selectedStepIndex() - 1);
    }
  }

  removeStep(stepIndex: number): void {
    this.deleteStep(stepIndex);
  }

  moveStep(stepIndex: number, delta: number): void {
    const currentSteps = [...this.steps()];
    const fromIndex = currentSteps.findIndex((step) => step.stepIndex === stepIndex);
    const toIndex = fromIndex + delta;
    if (fromIndex < 0 || toIndex < 0 || toIndex >= currentSteps.length) return;
    const [moved] = currentSteps.splice(fromIndex, 1);
    currentSteps.splice(toIndex, 0, moved);
    const reindexed = currentSteps.map((step, index) => ({...step, stepIndex: index + 1}));
    this.steps.set(reindexed);
    this.selectedStepIndex.set(toIndex + 1);
  }

  stepSummary(step: ShadowStep): string {
    const kind = step.type || 'Action';
    const target = (step.payload?.['selector'] as string) || (step.payload?.['text'] as string) || (step.payload?.['url'] as string) || '';
    return target ? `${kind}: ${target}` : kind;
  }

  stepLocator(step: ShadowStep): string {
    return (step.payload?.['selector'] as string) || (step.payload?.['target'] as string) || 'No selector';
  }

  copyFullJson(): void {
    const jsonText = JSON.stringify(this.steps(), null, 2);
    if (navigator.clipboard) {
      navigator.clipboard.writeText(jsonText);
    }
    this.savedMessage.set('Copied full JSON to clipboard!');
    setTimeout(() => this.savedMessage.set(null), 2500);
  }

  generateScript(): void {
    const actionSetId = this.actionSetId();
    if (!actionSetId) {
      this.error.set('Please save the action set before generating a script.');
      return;
    }

    this.generatingScript.set(true);
    this.error.set(null);

    const feedback = this.includeFeedback() ? {
      error: this.feedbackError() || undefined,
      logs: this.feedbackLogs() || undefined
    } : undefined;

    this.api.generatePlaywrightScript(actionSetId, feedback)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (response) => {
          this.generatingScript.set(false);
          this.playwrightScript.set(response.script);
          this.isEditingScript.set(false);
        },
        error: (error) => {
          this.generatingScript.set(false);
          this.error.set(error?.error?.message ?? 'Failed to generate script');
        }
      });
  }

  toggleScriptEdit(): void {
    this.isEditingScript.update((editing) => !editing);
  }

  onScriptChange(newScript: string): void {
    this.playwrightScript.set(newScript);
  }

  saveScript(): void {
    const actionSetId = this.actionSetId();
    const script = this.playwrightScript();
    if (!actionSetId || script === null) return;

    this.savingScript.set(true);
    this.error.set(null);

    this.api.updateActionSet(actionSetId, {playwrightScript: script})
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.savingScript.set(false);
          this.isEditingScript.set(false);
          this.savedMessage.set('Script saved successfully!');
          setTimeout(() => this.savedMessage.set(null), 3000);
        },
        error: (error) => {
          this.savingScript.set(false);
          this.error.set(error?.error?.message ?? 'Failed to save script');
        }
      });
  }

  saveActionSet(): void {
    const name = this.actionSetName().trim();
    if (!name) {
      this.error.set('Please enter a name for the action set');
      return;
    }

    this.saving.set(true);
    this.error.set(null);
    this.savedMessage.set(null);

    const payload = {
      name,
      targetApp: this.targetApp(),
      steps: {
        steps: this.steps(),
        count: this.steps().length
      }
    };

    const actionSetId = this.actionSetId();
    if (actionSetId) {
      this.api.updateActionSet(actionSetId, payload)
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe({
          next: () => {
            this.saving.set(false);
            this.savedMessage.set('Action set updated successfully!');
            setTimeout(() => this.savedMessage.set(null), 3000);
          },
          error: (error) => {
            this.saving.set(false);
            this.error.set(error?.error?.message ?? 'Failed to update action set');
          }
        });
    } else {
      this.api.createActionSet(payload)
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe({
          next: (response) => {
            this.saving.set(false);
            this.actionSetId.set(response.actionSetId);
            this.savedMessage.set('Action set saved successfully!');
            this.router.navigate(['/action-sets', response.actionSetId]);
          },
          error: (error) => {
            this.saving.set(false);
            this.error.set(error?.error?.message ?? 'Failed to save action set');
          }
        });
    }
  }
}
