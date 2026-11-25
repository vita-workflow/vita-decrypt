#!/usr/bin/env bash
# Regenerates the desktop app's Windows icon from cmd/vita-decrypt-gui/winres/icon.svg:
#   winres/icon.ico                 the icon at the sizes Windows uses, 16 to 256 pixels
#   rsrc_windows_amd64.syso         that icon as resource 1 with version info for "dev" (scripts/winres), which Go links
#                                   into windows/amd64 builds; Explorer shows the icon for the .exe and Gio uses it for
#                                   the window and taskbar
# Needs rsvg-convert and ImageMagick 7 (magick); on Debian or Ubuntu: librsvg2-bin imagemagick. The output is
# committed, so ordinary builds don't need them. Runs with the same tools give the same bytes, but other ImageMagick
# versions can write the .ico differently, so run this only when the SVG changes and commit what it writes.
set -euo pipefail
cd "$(dirname "$0")/.."
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

pngs=()
for size in 16 20 24 32 40 48 64 256; do
  rsvg-convert -w "$size" -h "$size" cmd/vita-decrypt-gui/winres/icon.svg -o "$tmp/$size.png"
  pngs+=("$tmp/$size.png")
done
# -strip and the excluded chunks drop timestamps, so the same SVG always gives the same bytes.
magick "${pngs[@]}" -strip -define png:exclude-chunks=date,time cmd/vita-decrypt-gui/winres/icon.ico

go run ./scripts/winres -version dev -target gui
sha256sum cmd/vita-decrypt-gui/winres/icon.ico cmd/vita-decrypt-gui/rsrc_windows_amd64.syso
