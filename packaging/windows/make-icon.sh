#!/bin/bash
# Render the app's artwork into icon.ico, the icon of the Windows executable and
# of its installer.
#
# Run by hand when the artwork changes, never by the build: the result is
# committed. rsvg-convert comes from `brew install librsvg`; the .ico is made by
# the Wails CLI of the version the app is built with.
set -euo pipefail
cd "$(dirname "$0")"

command -v rsvg-convert >/dev/null || {
	echo "needs rsvg-convert: brew install librsvg" >&2
	exit 1
}

rsvg-convert -w 1024 -h 1024 ../darwin/icon.svg -o icon.png
go run "github.com/wailsapp/wails/v3/cmd/wails3@$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)" \
	generate icons -input icon.png -windowsfilename icon.ico -macfilename ""
rm icon.png
echo "icon.ico  $(du -h icon.ico | cut -f1)"
