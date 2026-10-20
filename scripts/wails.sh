#!/usr/bin/env sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
# Override GO126_BIN when Go is installed elsewhere.
export PATH="${GO126_BIN:-$HOME/Env/GoSDK/go1.26.0/bin}:$project_root/.tools/bin:$PATH"
export GOTOOLCHAIN=local
export CGO_ENABLED=1
cd "$project_root"

if [ ! -x "$project_root/.tools/bin/wails3" ]; then
  mkdir -p "$project_root/.tools/bin"
  GOBIN="$project_root/.tools/bin" go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.33
fi
if [ "${1:-}" = "dev" ]; then
  shift
  export WAILS_VITE_PORT="${WAILS_VITE_PORT:-9256}"
  exec "$project_root/.tools/bin/wails3" dev -config ./build/config.yml -port "$WAILS_VITE_PORT" "$@"
fi
exec "$project_root/.tools/bin/wails3" "$@"
