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

import {HttpClient} from '@angular/common/http';
import {Injectable, inject} from '@angular/core';
import {Observable} from 'rxjs';
import {
  ActionSetItem,
  BatchExecutionItem,
  ScriptRunStatus,
  ShadowStep,
  WorkspaceCronSchedule,
  WorkspaceSummary
} from './types';

/**
 * Service providing HTTP REST client methods for ShadowOps backend APIs.
 */
@Injectable({providedIn: 'root'})
export class ApiService {
  private readonly http = inject(HttpClient);
  readonly baseUrl = '/api/v1';

  health(): Observable<{ok: boolean; shadowopsEnabled?: boolean}> {
    return this.http.get<{ok: boolean; shadowopsEnabled?: boolean}>(`${this.baseUrl}/health`);
  }

  listWorkspaces(): Observable<{items: WorkspaceSummary[]}> {
    return this.http.get<{items: WorkspaceSummary[]}>(`${this.baseUrl}/workspaces`);
  }

  listDefaultScripts(): Observable<{items: Array<{filename: string; source: string}>}> {
    return this.http.get<{items: Array<{filename: string; source: string}>}>(`${this.baseUrl}/default-scripts`);
  }

  createWorkspace(targetApp = 'google_ads'): Observable<{workspaceId: string; status: string}> {
    return this.http.post<{workspaceId: string; status: string}>(`${this.baseUrl}/workspaces`, {targetApp});
  }

  getWorkspace(workspaceId: string): Observable<WorkspaceSummary> {
    return this.http.get<WorkspaceSummary>(`${this.baseUrl}/workspaces/${workspaceId}`);
  }

  renameWorkspace(workspaceId: string, name: string): Observable<{ok: boolean; name: string}> {
    return this.http.patch<{ok: boolean; name: string}>(`${this.baseUrl}/workspaces/${workspaceId}`, {name});
  }

  navigateWorkspace(workspaceId: string, url: string): Observable<{ok: boolean}> {
    return this.http.post<{ok: boolean}>(`${this.baseUrl}/workspaces/${workspaceId}/navigate`, {url});
  }

  killWorkspace(workspaceId: string): Observable<{ok: boolean; status: string}> {
    return this.http.post<{ok: boolean; status: string}>(`${this.baseUrl}/workspaces/${workspaceId}/kill`, {});
  }

  startShadow(workspaceId: string): Observable<{ok: boolean; recordingState: 'on'; startedAt: string; captureSessionId: string}> {
    return this.http.post<{ok: boolean; recordingState: 'on'; startedAt: string; captureSessionId: string}>(`${this.baseUrl}/workspaces/${workspaceId}/shadow/start`, {});
  }

  endShadow(workspaceId: string): Observable<{ok: boolean; recordingState: 'off'; captureSessionId: string; steps: ShadowStep[]}> {
    return this.http.post<{ok: boolean; recordingState: 'off'; captureSessionId: string; steps: ShadowStep[]}>(`${this.baseUrl}/workspaces/${workspaceId}/shadow/end`, {});
  }

  getShadowActions(workspaceId: string): Observable<{captureSessionId?: string; steps: ShadowStep[]}> {
    return this.http.get<{captureSessionId?: string; steps: ShadowStep[]}>(`${this.baseUrl}/workspaces/${workspaceId}/shadow/actions`);
  }

  runWorkspaceScript(workspaceId: string, payload: {filename: string; script: string; targetUrl?: string; inputs?: Record<string, unknown>}): Observable<{ok: boolean; runId: string; status: string}> {
    return this.http.post<{ok: boolean; runId: string; status: string}>(`${this.baseUrl}/workspaces/${workspaceId}/script/run`, payload);
  }

  getWorkspaceScriptStatus(workspaceId: string): Observable<ScriptRunStatus> {
    return this.http.get<ScriptRunStatus>(`${this.baseUrl}/workspaces/${workspaceId}/script/status`);
  }

  cancelWorkspaceScript(workspaceId: string): Observable<{ok: boolean; runId?: string; status?: string}> {
    return this.http.post<{ok: boolean; runId?: string; status?: string}>(`${this.baseUrl}/workspaces/${workspaceId}/script/cancel`, {});
  }

  listActionSets(): Observable<{items: ActionSetItem[]}> {
    return this.http.get<{items: ActionSetItem[]}>(`${this.baseUrl}/action-sets`);
  }

  getActionSet(actionSetId: string): Observable<ActionSetItem> {
    return this.http.get<ActionSetItem>(`${this.baseUrl}/action-sets/${actionSetId}`);
  }

  createActionSet(payload: {name: string; targetApp: string; steps: Record<string, unknown>}): Observable<{actionSetId: string; version: number}> {
    return this.http.post<{actionSetId: string; version: number}>(`${this.baseUrl}/action-sets`, payload);
  }

  updateActionSet(actionSetId: string, payload: Partial<{name: string; targetApp: string; status: string; steps: Record<string, unknown>; playwrightScript: string}>): Observable<{actionSetId: string; version: number}> {
    return this.http.put<{actionSetId: string; version: number}>(`${this.baseUrl}/action-sets/${actionSetId}`, payload);
  }

  deleteActionSet(actionSetId: string): Observable<{ok: boolean; actionSetId: string}> {
    return this.http.delete<{ok: boolean; actionSetId: string}>(`${this.baseUrl}/action-sets/${actionSetId}`);
  }

  generatePlaywrightScript(actionSetId: string, feedback?: {error?: string; traceback?: string; logs?: string}): Observable<{script: string}> {
    return this.http.post<{script: string}>(`${this.baseUrl}/action-sets/${actionSetId}/generate-script`, {feedback});
  }

  getWorkspaceCron(workspaceId: string): Observable<WorkspaceCronSchedule> {
    return this.http.get<WorkspaceCronSchedule>(`${this.baseUrl}/workspaces/${workspaceId}/cron`);
  }

  setWorkspaceCron(workspaceId: string, payload: {cronExpression: string; scriptFilename: string; scriptSource: string; csvFilename: string; csvData: string}): Observable<{ok: boolean}> {
    return this.http.post<{ok: boolean}>(`${this.baseUrl}/workspaces/${workspaceId}/cron`, payload);
  }

  deleteWorkspaceCron(workspaceId: string): Observable<{ok: boolean}> {
    return this.http.delete<{ok: boolean}>(`${this.baseUrl}/workspaces/${workspaceId}/cron`);
  }

  listBatchExecutions(workspaceId: string): Observable<BatchExecutionItem[]> {
    return this.http.get<BatchExecutionItem[]>(`${this.baseUrl}/workspaces/${workspaceId}/executions`);
  }

  cancelBatchExecution(workspaceId: string, executionId: string): Observable<{ok: boolean}> {
    return this.http.post<{ok: boolean}>(`${this.baseUrl}/workspaces/${workspaceId}/executions/${executionId}/cancel`, {});
  }
}
