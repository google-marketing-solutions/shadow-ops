import {CommonModule} from '@angular/common';
import {ChangeDetectionStrategy, Component, computed, DestroyRef, inject, signal} from '@angular/core';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';
import {Router, RouterLink} from '@angular/router';
import {ApiService} from '../api.service';
import {SessionStateService} from '../session-state.service';
import {ActionSetItem, WorkspaceSummary} from '../types';

/**
 * DashboardPageComponent manages and renders the main dashboard overview,
 * including active workspaces, action sets, and workspace lifecycle actions.
 */
@Component({
  standalone: true,
  selector: 'app-dashboard-page',
  imports: [CommonModule, RouterLink],
  templateUrl: './dashboard-page.component.ng.html',
  styleUrl: './dashboard-page.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class DashboardPageComponent {
  private readonly api = inject(ApiService);
  private readonly router = inject(Router);
  private readonly sessionState = inject(SessionStateService);
  private readonly destroyRef = inject(DestroyRef);

  readonly workspaces = signal<WorkspaceSummary[]>([]);
  readonly actionSets = signal<ActionSetItem[]>([]);
  readonly loading = signal(false);
  readonly creatingWorkspace = signal(false);
  readonly error = signal<string | null>(null);
  readonly runningWorkspaceCount = computed(() => this.workspaces().filter((workspace) => workspace.status !== 'killed').length);
  readonly shadowopsEnabled = computed(() => this.sessionState.shadowopsEnabled());
  readonly topActionSets = computed(() => this.actionSets().slice(0, 4));

  constructor() {
    this.refresh();
  }

  refresh(): void {
    this.loading.set(true);
    this.error.set(null);
    this.api.listWorkspaces()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (response) => this.workspaces.set(response.items ?? []),
        error: (error) => this.error.set(error?.error?.message ?? 'Failed to load workspaces')
      });

    this.api.listActionSets()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (response) => {
          this.actionSets.set(response.items ?? []);
          this.loading.set(false);
        },
        error: (error) => {
          this.error.set(error?.error?.message ?? 'Failed to load action sets');
          this.loading.set(false);
        }
      });
  }

  newWorkspace(): void {
    this.creatingWorkspace.set(true);
    this.api.createWorkspace()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (response) => {
          this.creatingWorkspace.set(false);
          this.router.navigate(['/workspaces', response.workspaceId]);
        },
        error: (error) => {
          this.creatingWorkspace.set(false);
          this.error.set(error?.error?.message ?? 'Failed to create workspace');
        }
      });
  }

  goToWorkspace(workspace: WorkspaceSummary): void {
    this.sessionState.setWorkspace(workspace);
    this.router.navigate(['/workspaces', workspace.workspaceId]);
  }

  statusClass(status: string): string {
    const normalized = (status || '').toLowerCase();
    if (normalized.includes('active') || normalized.includes('record')) return 'is-active';
    if (normalized.includes('kill') || normalized.includes('error')) return 'is-error';
    return 'is-muted';
  }
}
