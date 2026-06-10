import {Injectable, signal} from '@angular/core';
import {ShadowStep, WorkspaceSummary} from './types';

/**
 * Service managing application session state and active workspace context.
 */
@Injectable({providedIn: 'root'})
export class SessionStateService {
  readonly activeWorkspaceId = signal<string | null>(null);
  readonly latestWorkspace = signal<WorkspaceSummary | null>(null);
  readonly latestCaptureSessionId = signal<string | null>(null);
  readonly latestSteps = signal<ShadowStep[]>([]);
  readonly latestFrameSrc = signal<string | null>(null);
  readonly latestCurrentUrl = signal<string | null>(null);

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
