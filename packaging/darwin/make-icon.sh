#!/bin/bash
# Render icon.svg into AppIcon.icns and the disk image background.
#
# Run by hand when the artwork changes, never by the build: the results are
# committed, so `make dmg` needs nothing installed beyond what macOS ships.
# rsvg-convert comes from `brew install librsvg`.
set -euo pipefail
cd "$(dirname "$0")"

command -v rsvg-convert >/dev/null || {
	echo "needs rsvg-convert: brew install librsvg" >&2
	exit 1
}

rm -rf AppIcon.iconset && mkdir AppIcon.iconset
# The sizes macOS actually asks for. Each @2x is the same artwork at twice the
# pixels, not an upscale, which is the whole reason to keep the source as SVG.
for size in 16 32 128 256 512; do
	rsvg-convert -w $size -h $size icon.svg -o "AppIcon.iconset/icon_${size}x${size}.png"
	rsvg-convert -w $((size * 2)) -h $((size * 2)) icon.svg -o "AppIcon.iconset/icon_${size}x${size}@2x.png"
done
iconutil -c icns AppIcon.iconset -o AppIcon.icns
rm -rf AppIcon.iconset
echo "AppIcon.icns  $(du -h AppIcon.icns | cut -f1)"

# The disk image window. 660x400 at 1x, and a @2x copy so it is not soft on a
# Retina display — Finder picks the right one from the .tiff.
rsvg-convert -w 660 -h 400 dmg-background.svg -o background.png
rsvg-convert -w 1320 -h 800 dmg-background.svg -o background@2x.png
tiffutil -cathidpicheck background.png background@2x.png -out background.tiff
rm -f background.png background@2x.png
echo "background.tiff  $(du -h background.tiff | cut -f1)"
