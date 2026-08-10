/**
 * Copyright 2026 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

  readonly editingWorkspaceId = signal<string | null>(null);
  readonly editingName = signal<string>('');
  readonly savingRename = signal(false);

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

  startRename(workspace: WorkspaceSummary, event?: Event): void {
    if (this.savingRename()) return;
    if (event) {
      event.stopPropagation();
    }
    this.editingWorkspaceId.set(workspace.workspaceId);
    this.editingName.set(workspace.name || workspace.workspaceId);
  }

  cancelRename(): void {
    this.editingWorkspaceId.set(null);
    this.editingName.set('');
  }

  onRenameInput(event: Event): void {
    const input = event.target as HTMLInputElement;
    this.editingName.set(input.value);
  }

  saveRename(workspace: WorkspaceSummary): void {
    const newName = this.editingName().trim();
    if (!newName || newName === workspace.name) {
      this.cancelRename();
      return;
    }
    this.savingRename.set(true);
    this.api.renameWorkspace(workspace.workspaceId, newName)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (response) => {
          this.savingRename.set(false);
          this.editingWorkspaceId.set(null);
          this.workspaces.update((items) =>
            items.map((item) => (item.workspaceId === workspace.workspaceId ? {...item, name: response.name} : item))
          );
        },
        error: (error: {error?: {message?: string}}) => {
          this.savingRename.set(false);
          this.error.set(error?.error?.message ?? 'Failed to rename workspace');
        }
      });
  }

  statusClass(status: string): string {
    const normalized = (status || '').toLowerCase();
    if (normalized.includes('active') || normalized.includes('record')) return 'is-active';
    if (normalized.includes('kill') || normalized.includes('error')) return 'is-error';
    return 'is-muted';
  }
}
