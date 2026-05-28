#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="$REPO_ROOT/apps/desktop/src-tauri/binaries"
DESKTOP_DIR="$REPO_ROOT/apps/desktop"

mkdir -p "$BIN_DIR"

TRIPLE="$(rustc -vV | sed -n 's/^host: //p')"
if [ -z "$TRIPLE" ]; then
  echo "could not determine rustc host triple" >&2
  exit 1
fi

EXT=""
case "$TRIPLE" in
  *windows*) EXT=".exe" ;;
esac

OUT="$BIN_DIR/ar-$TRIPLE$EXT"
echo ">> Building ar CLI for triple $TRIPLE"
(cd "$REPO_ROOT/apps/cli" && go build -o "$OUT" .)
echo ">> Sidecar at $OUT"

case "${1:-}" in
  --dev)   (cd "$DESKTOP_DIR" && pnpm tauri dev) ;;
  --build) (cd "$DESKTOP_DIR" && pnpm tauri build) ;;
  *)
    cat <<EOF

Next steps:
  cd apps/desktop
  pnpm install     # first time only
  pnpm tauri dev   # run app
  pnpm tauri build # produce installer
EOF
    ;;
esac
