/** Represents the recording state of a Shadow session. */
export type RecordingState = 'on' | 'off';

/** Represents the lifecycle status of a workspace. */
export type WorkspaceStatus = 'launching' | 'active' | 'recording' | 'killed' | 'completed' | string;

/** Summary metadata describing a single workspace instance. */
export interface WorkspaceSummary {
  workspaceId: string;
  name?: string;
  apiKey?: string;
  status: WorkspaceStatus;
  recordingState: RecordingState;
  currentUrl?: string;
  scriptStatus?: string;
  scriptRunId?: string;
  createdAt: string;
  updatedAt?: string;
}

/** DOM locator candidate for automated interaction. */
export interface LocatorCandidate {
  kind: string;
  value?: string;
  role?: string;
  name?: string;
}

/** Recorded user or browser event step. */
export interface ShadowStep {
  stepIndex: number;
  type: string;
  payload?: Record<string, unknown>;
  meta?: Record<string, unknown>;
}

/** Saved workflow action set entity. */
export interface ActionSetItem {
  id: string;
  name: string;
  target_app: string;
  version: number;
  status: string;
  steps: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  playwrightScript?: string;
}

/** Screencast video frame payload. */
export interface FrameMessage {
  type: 'frame';
  seq: number;
  ts: number;
  format: 'jpeg';
  width: number;
  height: number;
  data: string;
}

/** Status update broadcast from workspace container. */
export interface WorkspaceStatusMessage {
  type: 'workspace.status';
  status: WorkspaceStatus;
  recordingState: RecordingState;
  currentUrl?: string;
  apiKey?: string;
}

/** Live shadow step recorded event. */
export interface ShadowStepMessage {
  type: 'shadow.step';
  step: ShadowStep;
}

/** Raw shadow DOM event telemetry. */
export interface ShadowRawEventMessage {
  type: 'shadow.raw_event';
  event: Record<string, unknown>;
}

/** Script log entry from Playwright execution. */
export interface ScriptLogEntry {
  ts: string;
  level: string;
  message: string;
}

/** Status and metadata of a script execution run. */
export interface ScriptRunStatus {
  runId?: string | null;
  status: 'idle' | 'running' | 'cancelling' | 'succeeded' | 'failed' | 'cancelled' | string;
  filename?: string | null;
  targetUrl?: string | null;
  startedAt?: string | null;
  finishedAt?: string | null;
  cancelRequested?: boolean;
  error?: string | null;
  traceback?: string | null;
  result?: unknown;
  logs?: ScriptLogEntry[];
}

/** Script status update message. */
export interface ScriptStatusMessage {
  type: 'script.status';
  runId?: string | null;
  status: ScriptRunStatus['status'];
  filename?: string | null;
  targetUrl?: string | null;
  startedAt?: string | null;
  finishedAt?: string | null;
  cancelRequested?: boolean;
  error?: string | null;
}

/** Live script log message. */
export interface ScriptLogMessage {
  type: 'script.log';
  runId?: string | null;
  ts: string;
  level: string;
  message: string;
}

/** Script execution completion result message. */
export interface ScriptResultMessage {
  type: 'script.result';
  runId?: string | null;
  status: ScriptRunStatus['status'];
  result?: unknown;
  error?: string | null;
  traceback?: string | null;
  finishedAt?: string | null;
}

/** Metadata for a batch script execution record. */
export interface BatchExecutionItem {
  id: string;
  status: string;
  startedAt: string;
  finishedAt?: string;
  error?: string;
}

/** Metadata for a workspace cron schedule. */
export interface WorkspaceCronSchedule {
  cronExpression: string;
  scriptFilename?: string;
  csvFilename?: string;
  updatedAt?: string;
}

/** General error message payload. */
export interface ErrorMessage {
  type: 'error';
  code: string;
  message: string;
}

/** Union of all incoming WebSocket messages from the workspace stream. */
export type WorkspaceWsMessage =
  | FrameMessage
  | WorkspaceStatusMessage
  | ShadowStepMessage
  | ShadowRawEventMessage
  | ScriptStatusMessage
  | ScriptLogMessage
  | ScriptResultMessage
  | ErrorMessage;
