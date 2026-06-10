import {CommonModule} from '@angular/common';
import {ChangeDetectionStrategy, Component, computed, DestroyRef, inject, signal} from '@angular/core';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';
import {FormsModule} from '@angular/forms';
import {RouterLink} from '@angular/router';
import {ApiService} from '../api.service';
import {ActionSetItem} from '../types';

/**
 * ActionSetsPageComponent manages and displays the library of recorded Action Sets.
 */
@Component({
  standalone: true,
  selector: 'app-action-sets-page',
  imports: [CommonModule, FormsModule, RouterLink],
  templateUrl: './action-sets-page.component.ng.html',
  styleUrl: './action-sets-page.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class ActionSetsPageComponent {
  private readonly api = inject(ApiService);
  private readonly destroyRef = inject(DestroyRef);

  readonly actionSets = signal<ActionSetItem[]>([]);
  readonly query = signal('');
  readonly loading = signal(true);
  readonly activeCount = computed(() => this.actionSets().filter((item) => item.status === 'active').length);
  readonly draftCount = computed(() => this.actionSets().filter((item) => item.status !== 'active').length);
  readonly lastEditedAt = computed(() => this.actionSets()[0]?.updated_at ?? null);

  readonly filtered = computed(() => {
    const searchQuery = this.query().trim().toLowerCase();
    if (!searchQuery) return this.actionSets();
    return this.actionSets().filter((item) => `${item.name} ${item.target_app} ${item.status}`.toLowerCase().includes(searchQuery));
  });

  constructor() {
    this.refresh();
  }

  refresh(): void {
    this.loading.set(true);
    this.api.listActionSets()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (response) => {
          this.actionSets.set(response.items ?? []);
          this.loading.set(false);
        },
        error: () => this.loading.set(false)
      });
  }

  stepCount(item: ActionSetItem): number {
    const steps = item.steps?.['steps'];
    return Array.isArray(steps) ? steps.length : 0;
  }

  deleteActionSet(item: ActionSetItem): void {
    if (!confirm(`Are you sure you want to delete "${item.name}"?`)) {
      return;
    }
    this.api.deleteActionSet(item.id)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.actionSets.update((sets) => sets.filter((set) => set.id !== item.id));
        },
        error: (error) => {
          console.error('Failed to delete action set', error);
          alert('Failed to delete action set. See console for details.');
        }
      });
  }
}
