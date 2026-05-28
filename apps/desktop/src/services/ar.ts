import { invoke } from "@tauri-apps/api/core";
import type {
  CopyResult,
  HistoryResult,
  InfoResult,
  InitResult,
  MaskAddResult,
  MaskListResult,
  MaskRmResult,
  ResetResult,
  StatusResult,
  SyncResult,
  UndoResult,
} from "../types/ar";

interface RawArResult {
  stdout: string;
  stderr: string;
  code: number;
}

async function runJSON<T>(folder: string, args: string[]): Promise<T> {
  const res = await invoke<RawArResult>("run_ar", {
    folder,
    args: [...args, "--json"],
  });
  if (res.code !== 0) {
    throw new Error(res.stderr.trim() || `ar exited ${res.code}`);
  }
  try {
    return JSON.parse(res.stdout) as T;
  } catch (e) {
    throw new Error(`ar returned non-JSON output: ${res.stdout.slice(0, 200)}`);
  }
}

async function runRaw(folder: string, args: string[]): Promise<RawArResult> {
  return invoke<RawArResult>("run_ar", { folder, args });
}

export const ar = {
  init: (folder: string, force = false) =>
    runJSON<InitResult>(folder, ["init", ...(force ? ["--force"] : [])]),

  copy: (folder: string, opts: { dest?: string; force?: boolean } = {}) =>
    runJSON<CopyResult>(folder, [
      "copy",
      ...(opts.dest ? ["--dest", opts.dest] : []),
      ...(opts.force ? ["--force"] : []),
    ]),

  status: (folder: string) => runJSON<StatusResult>(folder, ["status"]),

  sync: (folder: string, opts: { force?: boolean; withDelete?: boolean } = {}) =>
    runJSON<SyncResult>(folder, [
      "sync",
      "--yes",
      ...(opts.force ? ["--force"] : []),
      ...(opts.withDelete === false ? ["--with-delete=false"] : []),
    ]),

  undo: (folder: string, steps = 1) =>
    runJSON<UndoResult>(folder, ["undo", "-n", String(steps)]),

  history: (folder: string) => runJSON<HistoryResult>(folder, ["history"]),

  info: (folder: string) => runJSON<InfoResult>(folder, ["info"]),

  reset: (folder: string, opts: { withHistory?: boolean } = {}) =>
    runJSON<ResetResult>(folder, [
      "reset",
      "--yes",
      ...(opts.withHistory ? ["--with-history"] : []),
    ]),

  maskList: (folder: string) => runJSON<MaskListResult>(folder, ["mask", "list"]),

  maskAdd: (folder: string, file: string, paths: string[]) =>
    runJSON<MaskAddResult>(folder, ["mask", "add", file, ...paths]),

  maskRm: (folder: string, file: string, path?: string) =>
    runJSON<MaskRmResult>(folder, ["mask", "rm", file, ...(path ? [path] : [])]),

  raw: runRaw,
};
