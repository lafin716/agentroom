# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

`agentroom` (CLI binary: `ar`) lets a developer use LLM coding agents on a real project without ever exposing `.env` / API keys / other sensitive files. It creates a sanitized *workspace* copy of the project; the agent edits only that copy; then `ar sync` propagates the agent's changes back to the main project with conflict detection and LIFO undo.

Two consumers ship: a Go CLI (`apps/cli`) and a Tauri 2 + Vue 3 desktop GUI (`apps/desktop`) that shells out to the CLI sidecar with `--json` and renders the result.

## Non-negotiable security invariant

`.agentignore`-matched paths must be **invisible to the agent and untouched by sync**, in every code path:

- Never copied into the workspace (so the agent literally cannot read them).
- Never overwritten, deleted, or backed up during sync, even with `--force`.
- `.agent/` and `.git/` are *always* ignored (hardcoded, not user-configurable).

When changing copier / syncer / ignore code, treat this as the load-bearing property. Tests in `pkg/ignore` and `pkg/syncer` exist to defend it.

## Common commands

```bash
# Go workspace (root) — go.work links apps/cli + pkg
go test ./pkg/...                        # primary test target; CLI has only json_test
go test ./apps/cli/...
go build -o ar.exe ./apps/cli            # ar.exe on Windows; ar on Unix
go test ./pkg/syncer/ -run TestSync_...  # single test
```

```bash
# Desktop app (apps/desktop) — pnpm + Tauri 2
pnpm install                             # first time
pnpm build                               # vue-tsc + vite build → dist/  (required before any cargo step)
pnpm tauri dev                           # full dev: vite + cargo + window
cd src-tauri && cargo check              # Rust-only fast feedback
```

```bash
# Sidecar build (creates apps/desktop/src-tauri/binaries/ar-<rustc-triple>.exe)
powershell -ExecutionPolicy Bypass -File scripts/build-desktop.ps1          # Windows
scripts/build-desktop.sh                                                    # Unix
# Both accept --dev / --build to chain into pnpm tauri.
```

```bash
# Placeholder app icons (regenerates icons/icon.png + icon.ico from a generated "ar" mark)
powershell -ExecutionPolicy Bypass -File scripts/gen-placeholder-icons.ps1
```

`pnpm tauri dev` will fail if any of these are missing: `apps/desktop/dist/` (run `pnpm build`), `apps/desktop/src-tauri/binaries/ar-<triple>.exe` (run the build-desktop script), `apps/desktop/src-tauri/icons/icon.{png,ico}` (run the icon script or `pnpm tauri icon`).

## Architecture

### Go workspace layout

```
go.work → apps/cli  +  pkg
apps/cli/cmd/         cobra subcommands (init/copy/status/sync/undo/history/info) — thin
                      shells over pkg.  Each command honors --json (see cmd/root.go).
pkg/workspace/        .agent/ layout, config, atomic file write
pkg/ignore/           .agentignore matcher (gitignore syntax + hardcoded .agent/ .git/)
pkg/index/            SHA-256 content index used for baseline + diffs
pkg/copier/           main → workspace materialization (skips ignored)
pkg/differ/           three-way diff: workspace vs baseline, main vs baseline
pkg/snapshot/         pre-sync backup of about-to-change main files
pkg/syncer/           the dangerous part: applies workspace diff to main + writes history
pkg/logx/             logging helpers
```

### Sync data flow (the part that's easy to get wrong)

1. `ar copy` writes `.agent/baseline.json` — a SHA-256 index of every non-ignored main file *at copy time*.
2. `ar sync` computes two diffs:
   - **workspace vs baseline** → changes the agent made (what we want to apply)
   - **main vs baseline** → changes the user made in main outside the workspace
3. The intersection of those two sets is the **conflict set**. Non-empty conflict set ⇒ sync aborts unless `--force`.
4. Before mutating main, `syncer` writes affected files to `.agent/history/<id>/files/` and appends a row to `.agent/history/index.json`. Each history entry is one sync.
5. `ar undo` walks history LIFO: applied additions → delete, applied modifications/deletions → restore from backup, then rewinds `baseline.json` to the previous snapshot.

`--with-delete=false` keeps adds/modifies but drops deletions from the apply set. `.agentignore` filtering happens *before* diffing — ignored paths are simply not in either diff.

### Desktop app (Tauri 2 + Vue 3)

The frontend never reimplements `ar` logic. It runs the bundled `ar` sidecar with `--json` and parses the result.

```
apps/desktop/
  src/services/ar.ts       runJSON<T>(folder, args) → invoke('run_ar') → JSON.parse stdout
  src/services/fs.ts       read/write .agentignore via Rust commands (atomic tmp+rename)
  src/services/storage.ts  load/save registered folders via Rust commands
  src/stores/folders.ts    Pinia: registered project list (persisted to <app data>/folders.json)
  src/stores/output.ts     Pinia: command log lines shown in OutputPanel
  src/views/               FolderListView, ProjectDashboard, IgnoreEditor (CodeMirror 6)
  src/components/          Sidebar, OutputPanel, SyncDialog, HistoryDialog (Reka UI)
  src-tauri/src/lib.rs     Tauri builder; plugins: shell, dialog, opener
  src-tauri/src/commands.rs  run_ar (spawns sidecar, collects stdout/stderr/code),
                             read/write_agentignore, pick_folder, open_in_explorer,
                             open_in_vscode, load/save_folders
  src-tauri/binaries/      ar-<rustc-triple>.exe — bundled as Tauri externalBin sidecar
  src-tauri/capabilities/default.json  shell:allow-spawn restricted to binaries/ar;
                                       shell:allow-execute restricted to code / code.cmd
```

When adding a new CLI command, the path is: add it to `apps/cli/cmd/`, ensure it honors `--json`, add a typed wrapper in `apps/desktop/src/types/ar.ts` + a method in `src/services/ar.ts`. The Rust layer is `ar`-agnostic — it just shuttles bytes.

### `--json` output contract

Every command's `--json` payload is a stable shape consumed by the desktop app. When changing a command's JSON shape, also update `apps/desktop/src/types/ar.ts`. `cmd/json_test.go` covers the schemas.

## Things to know when editing

- The repo uses Go workspaces (`go.work`). Run `go` commands from the repo root or inside the module dir; don't `cd pkg && go test` if you've modified `apps/cli` (the replace directive depends on the workspace).
- Tauri's `generate_context!()` macro reads `tauri.conf.json` at compile time and *requires* every file in `bundle.icon` plus `frontendDist` to exist on disk. Empty `dist/` or missing icons ⇒ proc-macro panic, not a friendly error.
- `CommandError` in `src-tauri/src/commands.rs` deliberately doesn't `#[from]` plugin error types (their internals churn across Tauri 2 minor versions). Use `.map_err(|e| CommandError::Msg(...))` at call sites instead.
- Frontend strings in views/components are Korean by design — match the existing tone when adding UI text.
