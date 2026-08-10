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

import {Injectable, signal} from '@angular/core';
import {ShadowStep, WorkspaceSummary} from './types';

/**
 * Service managing application session state and active workspace context.
 */
@Injectable({providedIn: 'root'})
export class SessionStateService {
  readonly activeWorkspaceId = signal<string | null>(null);
  readonly shadowopsEnabled = signal(false);
  readonly latestWorkspace = signal<WorkspaceSummary | null>(null);
  readonly latestCaptureSessionId = signal<string | null>(null);
  readonly latestSteps = signal<ShadowStep[]>([]);
  readonly latestFrameSrc = signal<string | null>(null);
  readonly latestCurrentUrl = signal<string | null>(null);

  setShadowopsEnabled(enabled: boolean): void {
    this.shadowopsEnabled.set(enabled);
  }

  setWorkspace(workspace: WorkspaceSummary): void {
    this.activeWorkspaceId.set(workspace.workspaceId);
    this.latestWorkspace.set(workspace);
    this.latestCurrentUrl.set(workspace.currentUrl ?? this.latestCurrentUrl());
  }

  setCapture(captureSessionId: string | null, steps: ShadowStep[]): void {
    this.latestCaptureSessionId.set(captureSessionId);
    this.latestSteps.set(steps);
  }

  updateSteps(steps: ShadowStep[]): void {
    this.latestSteps.set(steps);
  }

  setLiveFrame(frameSrc: string | null, currentUrl?: string | null): void {
    this.latestFrameSrc.set(frameSrc);
    if (currentUrl !== undefined) {
      this.latestCurrentUrl.set(currentUrl);
    }
  }

  setCurrentUrl(currentUrl: string): void {
    this.latestCurrentUrl.set(currentUrl);
  }
}
