# agentroom desktop

Tauri 2 + Vue 3 GUI that wraps the `ar` CLI.

## First-time setup

```bash
# 1. Bundle the ar CLI as a sidecar (creates src-tauri/binaries/ar-<triple>.exe)
#    Windows
powershell -ExecutionPolicy Bypass -File ../../scripts/build-desktop.ps1
#    macOS / Linux
../../scripts/build-desktop.sh

# 2. Install JS deps
pnpm install

# 3. (Optional) Replace placeholder icons.
#    apps/desktop/src-tauri/icons/{icon.png,icon.ico} ships with a generated
#    "ar" placeholder. To use a proper logo, drop a 1024x1024 PNG and run:
pnpm tauri icon path/to/source.png
```

## Run

```bash
pnpm tauri dev
```

## Build installer

```bash
pnpm tauri build
```

## Architecture

- Frontend (`src/`): Vue 3 + Pinia + Reka UI + Tailwind, talks to Rust via `@tauri-apps/api` `invoke()`.
- Backend (`src-tauri/`): Rust commands run the `ar` sidecar with `--json` and parse the result.
- The `ar` binary is bundled via `tauri.conf.json` → `bundle.externalBin`. Tauri picks the
  correct `ar-<triple>.exe` automatically per platform.
- Registered folders persist to `<app data>/folders.json`.

## Security model

`.agentignore`-matched files are **never** copied to the agent workspace and **never**
touched by `ar sync`, regardless of UI actions. The GUI is a thin wrapper around the CLI
and inherits the CLI's safety guarantees.
