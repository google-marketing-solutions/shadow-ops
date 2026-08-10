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
import {
  AfterViewInit,
  ChangeDetectionStrategy,
  Component,
  computed,
  DestroyRef,
  ElementRef,
  HostListener,
  inject,
  OnDestroy,
  signal,
  ViewChild
} from '@angular/core';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';
import {FormsModule} from '@angular/forms';
import {ActivatedRoute, Router, RouterLink} from '@angular/router';
import * as Papa from 'papaparse';
import {interval, Subscription} from 'rxjs';
import {ApiService} from '../api.service';
import {SessionStateService} from '../session-state.service';
import {
  ActionSetItem,
  BatchExecutionItem,
  ScriptRunStatus,
  ShadowStep,
  WorkspaceCronSchedule,
  WorkspaceSummary,
  WorkspaceWsMessage
} from '../types';
import {WorkspaceSocketService} from '../workspace-socket.service';

const STATUS_SUCCEEDED = 'succeeded';
const STATUS_FAILED = 'failed';

/**
 * WorkspacePageComponent renders the real-time interactive browser viewport,
 * input forwarder, batch CSV runner, cron scheduler, and live session stream.
 */
@Component({
  standalone: true,
  selector: 'app-workspace-page',
  imports: [CommonModule, FormsModule, RouterLink],
  templateUrl: './workspace-page.component.ng.html',
  styleUrl: './workspace-page.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class WorkspacePageComponent implements AfterViewInit, OnDestroy {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly api = inject(ApiService);
  private readonly wsService = inject(WorkspaceSocketService);
  private readonly sessionState = inject(SessionStateService);
  private readonly destroyRef = inject(DestroyRef);

  @ViewChild('frameImg') private frameImg?: ElementRef<HTMLImageElement>;
  @ViewChild('viewportBox') private viewportBox?: ElementRef<HTMLElement>;

  readonly workspaceId = signal('');
  readonly workspaceStatus = signal<WorkspaceSummary | null>(null);
  readonly recordingState = signal<'on' | 'off'>('off');
  readonly currentUrl = signal('https://accounts.google.com/ServiceLogin/signinchooser');
  readonly frameSrc = signal<string | null>(null);
  readonly frameSize = signal({ width: 1280, height: 720 });
  readonly steps = signal<ShadowStep[]>([]);
  readonly rawEvents = signal<string[]>([]);
  readonly socketConnected = signal(false);
  readonly error = signal<string | null>(null);
  readonly busy = signal(false);
  readonly viewportFocused = signal(false);
  readonly navigateUrl = signal('https://accounts.google.com/ServiceLogin/signinchooser');
  readonly navigateUrlFocused = signal(false);
  readonly navigateUrlDirty = signal(false);
  readonly navigateUrlPending = signal<string | null>(null);
  readonly scriptTargetUrl = signal('https://accounts.google.com/ServiceLogin/signinchooser');
  readonly scriptTargetUrlFocused = signal(false);
  readonly scriptTargetUrlDirty = signal(false);
  readonly scriptFilename = signal('automation.py');
  readonly scriptSource = signal<string>('');
  readonly scriptStatusState = signal<ScriptRunStatus>({ status: 'idle', logs: [] });
  readonly scriptPanelError = signal<string | null>(null);
  readonly scriptSubmitting = signal(false);
  readonly captureKeyboardInput = signal(true);
  readonly highlightClicks = signal(true);
  readonly captureAudio = signal(false);
  readonly sessionStartedAt = signal(Date.now());
  readonly nowMs = signal(Date.now());
  readonly scriptableActionSets = signal<ActionSetItem[]>([]);
  readonly defaultScripts = signal<Array<{filename: string; source: string}>>([]);

  // Batch Execution State
  readonly csvFile = signal<File | null>(null);
  readonly csvData = signal<Array<Record<string, unknown>>>([]);
  readonly csvColumns = signal<string[]>([]);
  readonly batchRunning = signal(false);
  readonly batchProgress = signal({ current: 0, total: 0 });

  // Cron Schedule State
  readonly cronExpression = signal<string>('');
  readonly cronSchedule = signal<WorkspaceCronSchedule | null>(null);
  readonly batchExecutions = signal<BatchExecutionItem[]>([]);

  // API Upload State
  readonly apiModalOpen = signal(false);
  readonly apiCurlCopied = signal(false);

  openApiModal(event?: Event): void {
    if (event) {
      event.preventDefault();
      event.stopPropagation();
    }
    this.apiModalOpen.set(true);
    this.apiCurlCopied.set(false);
  }

  closeApiModal(event?: Event): void {
    if (event) {
      event.preventDefault();
      event.stopPropagation();
    }
    this.apiModalOpen.set(false);
    this.apiCurlCopied.set(false);
  }

  copyApiCurl(): void {
    const workspace = this.workspaceStatus();
    if (!workspace) return;
    const apiKey = workspace.apiKey || 'UNKNOWN';
    const host = window.location.origin;
    const curlCommand = `curl -X POST -H "Authorization: Bearer ${apiKey}" -F "file=@data.csv" ${host}/api/v1/workspaces/${workspace.workspaceId}/upload-csv`;

    if (!navigator.clipboard) {
      console.error('Clipboard API not available');
      return;
    }

    navigator.clipboard.writeText(curlCommand)
      .then(() => {
        this.apiCurlCopied.set(true);
        setTimeout(() => this.apiCurlCopied.set(false), 3000);
      })
      .catch((err) => {
        console.error('Failed to copy to clipboard', err);
      });
  }

  get windowLocationOrigin(): string {
    return window.location.origin;
  }

  readonly scriptRunning = computed(() => ['running', 'cancelling'].includes(String(this.scriptStatusState().status ?? 'idle')));
  readonly canReview = computed(() => !this.scriptRunning() && this.recordingState() === 'off' && this.steps().length > 0);
  readonly sessionSeconds = computed(() => Math.floor((this.nowMs() - this.sessionStartedAt()) / 1000));
  readonly shadowopsEnabled = computed(() => this.sessionState.shadowopsEnabled());

  private wsHandle?: ReturnType<WorkspaceSocketService['connect']>;
  private wsSub?: Subscription;
  private pollSub?: Subscription;
  private clockSub?: Subscription;
  private batchWaitInterval?: number;

  constructor() {
    this.route.paramMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params) => {
      const id = params.get('workspaceId') ?? '';
      if (!id || id === this.workspaceId()) {
        return;
      }
      this.workspaceId.set(id);
      this.connectWorkspace();
    });
    this.loadScriptableActionSets();
    this.loadDefaultScripts();
  }

  ngAfterViewInit(): void {
    queueMicrotask(() => this.sendViewportResize());
  }

  ngOnDestroy(): void {
    this.wsSub?.unsubscribe();
    this.wsHandle?.close();
    this.pollSub?.unsubscribe();
    this.clockSub?.unsubscribe();
    if (this.batchWaitInterval) {
      window.clearInterval(this.batchWaitInterval);
      this.batchWaitInterval = undefined;
    }
  }

  private connectWorkspace(): void {
    this.error.set(null);
    this.scriptPanelError.set(null);
    this.steps.set([]);
    this.rawEvents.set([]);
    this.frameSrc.set(null);
    this.viewportFocused.set(false);
    this.navigateUrlFocused.set(false);
    this.navigateUrlDirty.set(false);
    this.navigateUrlPending.set(null);
    this.scriptTargetUrlFocused.set(false);
    this.scriptTargetUrlDirty.set(false);
    this.scriptStatusState.set({ status: 'idle', logs: [] });
    this.wsSub?.unsubscribe();
    this.wsHandle?.close();

    const workspaceId = this.workspaceId();
    if (!workspaceId) return;

    this.api.getWorkspace(workspaceId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (workspace) => this.applyWorkspaceSnapshot(workspace),
        error: (err) => this.error.set(err?.error?.message ?? 'Failed to load workspace')
      });
    this.refreshScriptStatus();
    this.loadCronSchedule();
    this.loadBatchExecutions();

    this.wsHandle = this.wsService.connect(workspaceId);
    this.wsSub = this.wsHandle.messages$.pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: (message) => this.handleWsMessage(message),
      error: (err) => {
        this.socketConnected.set(false);
        this.error.set(err?.message ?? 'Workspace stream disconnected');
      },
      complete: () => this.socketConnected.set(false)
    });

    this.pollSub?.unsubscribe();
    this.pollSub = interval(1500).pipe(takeUntilDestroyed(this.destroyRef)).subscribe(() => {
      this.refreshScriptStatus();
    });

    this.clockSub?.unsubscribe();
    this.clockSub = interval(1000).pipe(takeUntilDestroyed(this.destroyRef)).subscribe(() => {
      this.nowMs.set(Date.now());
    });
  }

  private applyWorkspaceSnapshot(workspace: WorkspaceSummary): void {
    this.workspaceStatus.set(workspace);
    this.sessionState.setWorkspace(workspace);
    this.recordingState.set(workspace.recordingState ?? 'off');

    const incomingUrl = workspace.currentUrl || '';
    if (incomingUrl) {
      this.currentUrl.set(incomingUrl);
      if (!this.navigateUrlFocused() && !this.navigateUrlDirty()) {
        this.navigateUrl.set(incomingUrl);
      }
      if (!this.scriptTargetUrlFocused() && !this.scriptTargetUrlDirty() && !this.scriptRunning()) {
        this.scriptTargetUrl.set(incomingUrl);
      }
    }
  }

  private handleWsMessage(message: WorkspaceWsMessage): void {
    this.socketConnected.set(true);

    if (message.type === 'frame') {
      this.frameSrc.set(`data:image/jpeg;base64,${message.data}`);
      this.frameSize.set({ width: message.width, height: message.height });
      this.sessionState.setLiveFrame(this.frameSrc(), this.currentUrl());
      return;
    }

    if (message.type === 'workspace.status') {
      this.recordingState.set(message.recordingState ?? 'off');
      if (message.currentUrl) {
        this.currentUrl.set(message.currentUrl);
        if (!this.navigateUrlFocused() && !this.navigateUrlDirty()) {
          this.navigateUrl.set(message.currentUrl);
        }
        if (!this.scriptTargetUrlFocused() && !this.scriptTargetUrlDirty() && !this.scriptRunning()) {
          this.scriptTargetUrl.set(message.currentUrl);
        }
        this.sessionState.setCurrentUrl(message.currentUrl);
      }
      return;
    }

    if (message.type === 'shadow.step') {
      this.steps.update((items) => [...items, message.step]);
      this.sessionState.updateSteps(this.steps());
      return;
    }

    if (message.type === 'shadow.raw_event') {
      this.rawEvents.update((events) => [JSON.stringify(message.event), ...events.slice(0, 19)]);
      return;
    }

    if (message.type === 'script.status') {
      this.scriptStatusState.update((current) => ({
        ...current,
        runId: message.runId,
        status: message.status,
        filename: message.filename,
        targetUrl: message.targetUrl,
        startedAt: message.startedAt,
        finishedAt: message.finishedAt,
        cancelRequested: message.cancelRequested,
        error: message.error
      }));
      return;
    }

    if (message.type === 'script.log') {
      const entry = { ts: message.ts, level: message.level, message: message.message };
      this.scriptStatusState.update((current) => ({
        ...current,
        logs: [...(current.logs ?? []), entry]
      }));
      return;
    }

    if (message.type === 'script.result') {
      this.scriptStatusState.update((current) => ({
        ...current,
        status: message.status,
        result: message.result,
        error: message.error,
        traceback: message.traceback,
        finishedAt: message.finishedAt
      }));
      return;
    }

    if (message.type === 'error') {
      this.error.set(message.message);
    }
  }

  focusViewport(): void {
    this.viewportFocused.set(true);
  }

  blurViewport(): void {
    this.viewportFocused.set(false);
  }

  onNavigateUrlChange(url: string): void {
    this.navigateUrlDirty.set(true);
    this.navigateUrl.set(url);
  }

  onNavigateUrlFocus(): void {
    this.navigateUrlFocused.set(true);
  }

  onNavigateUrlBlur(): void {
    this.navigateUrlFocused.set(false);
  }

  navigate(): void {
    const raw = this.navigateUrl().trim();
    if (!raw || this.scriptRunning()) return;
    const target = /^https?:\/\//i.test(raw) ? raw : `https://${raw}`;
    this.wsHandle?.send({ type: 'navigate', url: target });
    this.currentUrl.set(target);
    this.navigateUrl.set(target);
    this.navigateUrlDirty.set(false);
    if (!this.scriptTargetUrlFocused() && !this.scriptTargetUrlDirty()) {
      this.scriptTargetUrl.set(target);
    }
  }

  onScriptTargetUrlChange(url: string): void {
    this.scriptTargetUrlDirty.set(true);
    this.scriptTargetUrl.set(url);
  }

  onScriptTargetUrlFocus(): void {
    this.scriptTargetUrlFocused.set(true);
  }

  onScriptTargetUrlBlur(): void {
    this.scriptTargetUrlFocused.set(false);
  }

  startShadow(): void {
    const workspaceId = this.workspaceId();
    if (!workspaceId || this.scriptRunning()) return;

    this.busy.set(true);
    this.error.set(null);
    this.api.startShadow(workspaceId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (response) => {
          this.busy.set(false);
          this.recordingState.set('on');
          this.steps.set([]);
          this.sessionState.setCapture(response.captureSessionId, []);
        },
        error: (err) => {
          this.busy.set(false);
          this.error.set(err?.error?.message ?? 'Failed to start recording');
        }
      });
  }

  endShadow(): void {
    const workspaceId = this.workspaceId();
    if (!workspaceId || this.scriptRunning()) return;

    this.busy.set(true);
    this.error.set(null);
    this.api.endShadow(workspaceId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (response) => {
          this.busy.set(false);
          this.recordingState.set('off');
          this.sessionState.setCapture(response.captureSessionId, response.steps ?? []);
          this.steps.set(response.steps ?? []);
        },
        error: (err) => {
          this.busy.set(false);
          this.error.set(err?.error?.message ?? 'Failed to stop recording');
        }
      });
  }

  killWorkspace(): void {
    const workspaceId = this.workspaceId();
    if (!workspaceId) return;

    this.api.killWorkspace(workspaceId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => this.router.navigate(['/dashboard']),
        error: (err) => this.error.set(err?.error?.message ?? 'Failed to kill workspace')
      });
  }

  openReview(): void {
    if (!this.canReview()) return;
    this.router.navigate(['/review', this.workspaceId()]);
  }

  refreshScriptStatus(): void {
    const workspaceId = this.workspaceId();
    if (!workspaceId) return;

    this.api.getWorkspaceScriptStatus(workspaceId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (status) => {
          if (status) {
            this.scriptStatusState.set(status);
            if (status.targetUrl && !this.scriptTargetUrlFocused() && !this.scriptTargetUrlDirty()) {
              this.scriptTargetUrl.set(status.targetUrl);
            }
            if (status.filename && this.scriptFilename() === 'automation.py') {
              this.scriptFilename.set(status.filename);
            }
          }
        },
        error: () => {}
      });
  }

  loadScriptableActionSets(): void {
    this.api.listActionSets()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (response) => {
          const items = (response.items ?? []).filter((item) => !!item.playwrightScript);
          this.scriptableActionSets.set(items);
        },
        error: () => {}
      });
  }

  private loadDefaultScripts(): void {
    this.api.listDefaultScripts()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (res) => this.defaultScripts.set(res.items || []),
        error: () => {}
      });
  }

  onDefaultScriptSelected(event: Event): void {
    const select = event.target as HTMLSelectElement;
    const found = this.defaultScripts().find((s) => s.filename === select.value);
    if (found) {
      this.scriptFilename.set(found.filename);
      this.scriptSource.set(found.source);
    }
  }

  loadScriptFromActionSet(event: Event): void {
    const select = event.target as HTMLSelectElement;
    const actionSetId = select.value;
    if (!actionSetId) return;

    const item = this.scriptableActionSets().find((it) => it.id === actionSetId);
    if (!item || !item.playwrightScript) return;

    this.scriptSource.set(item.playwrightScript);
    this.scriptFilename.set(`${item.name.toLowerCase().replace(/[^a-z0-9_]+/g, '_')}_v${item.version}.py`);
  }

  onScriptFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    this.scriptFilename.set(file.name);
    const reader = new FileReader();
    reader.onload = () => {
      this.scriptSource.set(String(reader.result ?? ''));
    };
    reader.readAsText(file);
  }

  runScript(): void {
    const workspaceId = this.workspaceId();
    const scriptSource = this.scriptSource().trim();
    if (!workspaceId || !scriptSource || this.scriptRunning()) return;

    this.scriptSubmitting.set(true);
    this.scriptPanelError.set(null);

    const filename = this.scriptFilename().trim() || 'script.py';
    const targetUrl = this.scriptTargetUrl().trim() || this.currentUrl();

    this.api.runWorkspaceScript(workspaceId, {
      filename,
      script: scriptSource,
      targetUrl,
      inputs: targetUrl ? { URL: targetUrl, url: targetUrl } : undefined
    })
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (response) => {
          this.scriptSubmitting.set(false);
          this.scriptStatusState.set({
            runId: response.runId,
            status: (response.status || 'running') as ScriptRunStatus['status'],
            filename: this.scriptFilename().trim() || 'script.py',
            targetUrl: this.scriptTargetUrl().trim() || this.currentUrl(),
            startedAt: new Date().toISOString(),
            logs: []
          });
        },
        error: (err) => {
          this.scriptSubmitting.set(false);
          this.scriptPanelError.set(err?.error?.message ?? 'Failed to submit script run');
        }
      });
  }

  cancelScript(): void {
    const workspaceId = this.workspaceId();
    if (!workspaceId || !this.scriptRunning()) return;

    this.scriptSubmitting.set(true);
    this.api.cancelWorkspaceScript(workspaceId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => this.scriptSubmitting.set(false),
        error: (err) => {
          this.scriptSubmitting.set(false);
          this.scriptPanelError.set(err?.error?.message ?? 'Failed to cancel script run');
        }
      });
  }

  loadCronSchedule(): void {
    const workspaceId = this.workspaceId();
    if (!workspaceId) return;

    this.api.getWorkspaceCron(workspaceId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (schedule) => {
          this.cronSchedule.set(schedule);
          if (schedule?.cronExpression) {
            this.cronExpression.set(schedule.cronExpression);
          }
        },
        error: () => this.cronSchedule.set(null)
      });
  }

  saveCronSchedule(): void {
    const workspaceId = this.workspaceId();
    const expr = this.cronExpression().trim();
    if (!workspaceId || !expr) return;

    let csvContent = '';
    if (this.csvData().length) {
      csvContent = Papa.unparse(this.csvData());
    }

    const payload = {
      cronExpression: expr,
      scriptFilename: this.scriptFilename() || this.cronSchedule()?.scriptFilename || 'cron_automation.py',
      scriptSource: this.scriptSource() || '',
      csvFilename: this.csvFile()?.name || this.cronSchedule()?.csvFilename || 'data.csv',
      csvData: csvContent
    };

    this.api.setWorkspaceCron(workspaceId, payload)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.cronSchedule.set({
            cronExpression: expr,
            scriptFilename: payload.scriptFilename,
            csvFilename: payload.csvFilename,
            updatedAt: new Date().toISOString()
          });
          this.scriptPanelError.set(null);
          this.loadBatchExecutions();
        },
        error: (err) => this.scriptPanelError.set(err?.error?.message ?? 'Failed to save cron schedule')
      });
  }

  deleteCronSchedule(): void {
    const workspaceId = this.workspaceId();
    if (!workspaceId) return;

    this.api.deleteWorkspaceCron(workspaceId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => {
          this.cronSchedule.set(null);
          this.cronExpression.set('');
        },
        error: (err) => this.scriptPanelError.set(err?.error?.message ?? 'Failed to delete cron schedule')
      });
  }

  loadBatchExecutions(): void {
    const workspaceId = this.workspaceId();
    if (!workspaceId) return;

    this.api.listBatchExecutions(workspaceId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (items) => this.batchExecutions.set(items || []),
        error: () => {}
      });
  }

  downloadExecutionCsv(executionId: string): void {
    const workspaceId = this.workspaceId();
    if (!workspaceId || !executionId) return;

    const host = window.location.port === '4200' ? `${window.location.protocol}//${window.location.hostname}:8080` : '';
    const url = `${host}/api/v1/workspaces/${workspaceId}/executions/${executionId}/csv`;
    const link = document.createElement('a');
    link.setAttribute('href', url);
    link.setAttribute('download', `execution_${executionId}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  }

  cancelExecution(executionId: string): void {
    const workspaceId = this.workspaceId();
    if (!workspaceId || !executionId) return;

    this.api.cancelBatchExecution(workspaceId, executionId)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: () => this.loadBatchExecutions(),
        error: (err) => this.scriptPanelError.set(err?.error?.message ?? 'Failed to cancel execution')
      });
  }

  onViewportMouse(event: MouseEvent, action: 'down' | 'up' | 'move' | 'click'): void {
    if (this.scriptRunning()) return;
    const coords = this.normalizeCoordinates(event);
    if (!coords) return;

    this.wsHandle?.send({
      type: 'mouse',
      action,
      x: coords.x,
      y: coords.y,
      button: this.mouseButtonString(event.button),
      buttons: event.buttons,
      modifiers: this.modifierState(event)
    });
  }

  onViewportWheel(event: WheelEvent): void {
    if (this.scriptRunning()) return;
    event.preventDefault();
    const coords = this.normalizeCoordinates(event);
    if (!coords) return;

    this.wsHandle?.send({
      type: 'wheel',
      x: coords.x,
      y: coords.y,
      deltaX: event.deltaX,
      deltaY: event.deltaY,
      modifiers: this.modifierState(event)
    });
  }

  @HostListener('window:keydown', ['$event'])
  onGlobalKeyDown(event: KeyboardEvent): void {
    if (!this.viewportFocused() || this.scriptRunning() || this.isUrlBarEvent(event)) return;

    if (['Tab', 'Space', 'ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(event.code)) {
      event.preventDefault();
    }

    this.wsHandle?.send({
      type: 'keyboard',
      action: 'keydown',
      key: event.key,
      code: event.code,
      modifiers: this.modifierState(event)
    });
  }

  @HostListener('window:keyup', ['$event'])
  onGlobalKeyUp(event: KeyboardEvent): void {
    if (!this.viewportFocused() || this.scriptRunning() || this.isUrlBarEvent(event)) return;

    this.wsHandle?.send({
      type: 'keyboard',
      action: 'keyup',
      key: event.key,
      code: event.code,
      modifiers: this.modifierState(event)
    });
  }

  private isUrlBarEvent(event: Event): boolean {
    const target = event.target as HTMLElement | null;
    if (!target) return false;
    const tag = target.tagName.toLowerCase();
    return tag === 'input' || tag === 'textarea' || target.isContentEditable;
  }

  private modifierState(event: MouseEvent | KeyboardEvent): { alt: boolean; ctrl: boolean; meta: boolean; shift: boolean } {
    return {
      alt: event.altKey,
      ctrl: event.ctrlKey,
      meta: event.metaKey,
      shift: event.shiftKey
    };
  }

  private normalizeCoordinates(event: MouseEvent): { x: number; y: number } | null {
    const img = this.frameImg?.nativeElement;
    if (!img) return null;

    const rect = img.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0) return null;

    const clientX = event.clientX - rect.left;
    const clientY = event.clientY - rect.top;

    const naturalWidth = img.naturalWidth || 1280;
    const naturalHeight = img.naturalHeight || 800;

    const normX = Math.max(0, Math.min(1, clientX / rect.width));
    const normY = Math.max(0, Math.min(1, clientY / rect.height));

    return {
      x: Math.round(normX * naturalWidth),
      y: Math.round(normY * naturalHeight)
    };
  }

  private mouseButtonString(btn: number): 'left' | 'middle' | 'right' {
    if (btn === 1) return 'middle';
    if (btn === 2) return 'right';
    return 'left';
  }

  private sendViewportResize(): void {
    const box = this.viewportBox?.nativeElement;
    if (!box) return;
    const width = Math.max(320, Math.round(box.clientWidth));
    const height = Math.max(240, Math.round(box.clientHeight));
    this.wsHandle?.send({ type: 'resize', width, height });
  }

  formattedStep(step: ShadowStep): string {
    if (!step) return '';
    const payload = step.payload ?? {};
    if (step.type === 'navigate') return String(payload['url'] ?? '');
    if (step.type === 'click') {
      const loc = payload['locator'] as { role?: string; name?: string } | undefined;
      return loc?.role ? `${loc.role} "${loc.name || ''}"` : `(${payload['x']}, ${payload['y']})`;
    }
    if (step.type === 'input') return `Type: "${payload['text'] ?? ''}"`;
    return JSON.stringify(payload);
  }

  timerText(): string {
    const total = this.sessionSeconds();
    const mins = Math.floor(total / 60);
    const secs = total % 60;
    return `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
  }

  cancelBatch(): void {
    this.batchRunning.set(false);
    if (this.scriptRunning()) {
      this.cancelScript();
    }
  }

  getCsvKeys(): string[] {
    const data = this.csvData();
    if (!data || data.length === 0) return [];
    return Object.keys(data[0]);
  }

  onCsvSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input?.files?.[0];
    if (!file) {
      this.csvFile.set(null);
      this.csvData.set([]);
      this.csvColumns.set([]);
      return;
    }
    this.csvFile.set(file);
    Papa.parse(file, {
      header: true,
      skipEmptyLines: true,
      complete: (results: Papa.ParseResult<Record<string, unknown>>) => {
        this.csvData.set(results.data || []);
        this.csvColumns.set(results.meta.fields || []);
      },
      error: (error: Error) => {
        this.scriptPanelError.set(`Failed to parse CSV: ${error.message}`);
      }
    });
  }

  async runBatch(): Promise<void> {
    if (!this.workspaceId() || this.batchRunning() || this.scriptRunning()) return;
    if (!this.csvData().length) {
      this.scriptPanelError.set('Please load a CSV file first');
      return;
    }
    if (!this.scriptSource().trim()) {
      this.scriptPanelError.set('Upload or paste a Python script first');
      return;
    }

    this.batchRunning.set(true);
    this.scriptPanelError.set(null);
    const data = [...this.csvData()];

    for (let i = 0; i < data.length; i++) {
      if (!this.batchRunning()) break;
      this.batchProgress.set({ current: i + 1, total: data.length });

      const row = { ...data[i] };

      try {
        await this.runScriptAndWait(row);
        const status = this.scriptStatusState();

        if (status.status === STATUS_SUCCEEDED && status.result) {
          const resultObj = typeof status.result === 'string' ? JSON.parse(status.result) : status.result;
          if (typeof resultObj === 'object' && resultObj !== null) {
            const flattenObject = (obj: Record<string, unknown>, prefix = ''): Record<string, unknown> => {
              return Object.keys(obj).reduce((acc: Record<string, unknown>, key: string) => {
                const pre = prefix.length ? prefix + '_' : '';
                const val = obj[key];
                if (typeof val === 'object' && val !== null && !Array.isArray(val)) {
                  Object.assign(acc, flattenObject(val as Record<string, unknown>, pre + key));
                } else {
                  if (typeof val === 'object' && val !== null) {
                    acc[pre + key] = JSON.stringify(val);
                  } else {
                    acc[pre + key] = val;
                  }
                }
                return acc;
              }, {});
            };
            const flatResult = flattenObject(resultObj as Record<string, unknown>);
            for (const [key, value] of Object.entries(flatResult)) {
              row[key] = value;
            }
          } else {
            row['Result'] = status.result;
          }
        } else if (status.status === STATUS_FAILED) {
          row['Error'] = status.error || 'Script failed without error message';
        } else {
          row['Error'] = `Script ended with status: ${status.status}`;
        }
      } catch (err: unknown) {
        const errorMsg = err instanceof Error ? err.message : String(err);
        row['Error'] = errorMsg || 'Failed to run script';
      }
      data[i] = row;
    }

    this.csvData.set(data);
    this.batchRunning.set(false);
    this.downloadBatchResults();
  }

  private async runScriptAndWait(inputs?: Record<string, unknown>): Promise<void> {
    return new Promise((resolve) => {
      const workspaceId = this.workspaceId();
      const scriptSource = this.scriptSource().trim();
      const filename = this.scriptFilename().trim() || 'script.py';
      const targetUrl = this.scriptTargetUrl().trim() || this.currentUrl();

      this.api.runWorkspaceScript(workspaceId, {
        filename,
        script: scriptSource,
        targetUrl,
        inputs: inputs ?? (targetUrl ? { URL: targetUrl, url: targetUrl } : undefined)
      })
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe({
          next: (response) => {
            this.scriptStatusState.set({
              runId: response.runId,
              status: (response.status || 'running') as ScriptRunStatus['status'],
              filename,
              targetUrl,
              startedAt: new Date().toISOString(),
              logs: []
            });
            if (this.batchWaitInterval) {
              window.clearInterval(this.batchWaitInterval);
            }
            this.batchWaitInterval = window.setInterval(() => {
              const status = this.scriptStatusState();
              if (['succeeded', 'failed', 'cancelled'].includes(String(status.status))) {
                if (this.batchWaitInterval) {
                  window.clearInterval(this.batchWaitInterval);
                  this.batchWaitInterval = undefined;
                }
                resolve();
              }
            }, 500);
          },
          error: (err) => {
            this.scriptStatusState.set({ status: 'failed', error: err?.error?.message ?? 'Execution error' });
            resolve();
          }
        });
    });
  }

  downloadBatchResults(): void {
    if (!this.csvData().length) return;
    const csv = Papa.unparse(this.csvData());
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.setAttribute('href', url);
    link.setAttribute('download', `results_${this.csvFile()?.name || 'batch.csv'}`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  }
}
