#!/usr/bin/env bash
# Reproducible builds into dist/, plus SHA-256 checksums.
#
# The command-line tool is static (no cgo) for Linux, Windows, and macOS. The desktop app cross-compiles for Windows
# without cgo, but on Linux and macOS it needs cgo and the system's graphics headers, so only the build for the
# machine running this script is made; run it on each platform to make the others.
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${1:?usage: scripts/build.sh VERSION}"
rm -rf dist && mkdir dist

# Windows resources (icon and version info) for this version, from scripts/winres. Afterwards the desktop app's
# committed file (version "dev") is put back and the command's is removed.
gui_res=cmd/vita-decrypt-gui/rsrc_windows_amd64.syso cli_res=rsrc_windows_amd64.syso
saved_res="$(mktemp)"
cp "$gui_res" "$saved_res"
trap 'cp "$saved_res" "$gui_res"; rm -f "$saved_res" "$cli_res"' EXIT
go run ./scripts/winres -version "$VERSION" -target gui
go run ./scripts/winres -version "$VERSION" -target cli

for target in linux/amd64 linux/arm64 windows/amd64 darwin/amd64 darwin/arm64; do
  os="${target%/*}" arch="${target#*/}"
  ext=""; [ "$os" = windows ] && ext=".exe"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -buildvcs=false \
    -ldflags="-s -w -buildid=" -o "dist/vita-decrypt-${VERSION}-${os}-${arch}${ext}" .
done

gui() { # os arch cgo [extra ldflags]
  local ext=""; [ "$1" = windows ] && ext=".exe"
  CGO_ENABLED="$3" GOOS="$1" GOARCH="$2" go build -trimpath -buildvcs=false \
    -ldflags="-s -w -buildid= -X main.version=${VERSION} ${4:-}" \
    -o "dist/vita-decrypt-gui-${VERSION}-$1-$2${ext}" ./cmd/vita-decrypt-gui
}
gui windows amd64 0 "-H windowsgui"
host_os="$(go env GOOS)" host_arch="$(go env GOARCH)"
case "$host_os" in
  linux | darwin) gui "$host_os" "$host_arch" 1 ;;
esac

(cd dist && sha256sum vita-decrypt-* > SHA256SUMS)
cat dist/SHA256SUMS
