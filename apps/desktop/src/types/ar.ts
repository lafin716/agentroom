export type ChangeOp = "added" | "modified" | "deleted";

export interface ArChange {
  op: ChangeOp;
  path: string;
}

export interface InitResult {
  initialized: boolean;
  agent_dir: string;
  ignore_path: string;
  config_path: string;
  ignore_wrote: boolean;
  config_wrote: boolean;
}

export interface CopyResult {
  workspace_path: string;
  main_path: string;
  files_copied: number;
  bytes_copied: number;
  baseline_path: string;
  files_indexed: number;
}

export interface StatusResult {
  main: string;
  workspace: string;
  last_sync?: string;
  workspace_changes: ArChange[];
  main_changes: ArChange[];
  conflicts: string[];
}

export interface SyncResult {
  applied: boolean;
  history_id?: string;
  workspace_changes: ArChange[];
  applied_changes: ArChange[];
  conflicts: string[];
  reason?: string;
}

export interface UndoEntry {
  id: string;
  changes_count: number;
}

export interface UndoResult {
  undone: UndoEntry[];
  remaining: number;
}

export interface HistoryEntry {
  seq: number;
  id: string;
  summary: string;
  applied_at: string;
}

export interface HistoryResult {
  entries: HistoryEntry[];
}

export interface InfoResult {
  version: number;
  cli_version: string;
  project_name: string;
  main_path: string;
  workspace_path?: string;
  created_at: string;
  last_sync_at?: string;
  initialized: boolean;
}

export interface ResetResult {
  workspace_deleted: boolean;
  workspace_path: string;
  baseline_deleted: boolean;
  history_deleted: boolean;
}

export interface Mask {
  file: string;
  paths: string[];
}

export interface MaskListResult {
  masks: Mask[];
}

export interface MaskAddResult {
  added: boolean;
  file: string;
  paths: string[];
  total_masks: number;
}

export interface MaskRmResult {
  removed: string[];
  file: string;
  remaining: number;
}

export interface RegisteredFolder {
  id: string;
  path: string;
  name: string;
  added_at: string;
}
