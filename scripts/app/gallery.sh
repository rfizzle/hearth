#!/bin/sh
# Assembles HearthGallery.app from a debug SwiftPM build. A bare executable
# started by `open` shows no window; it needs a bundle with an Info.plist, and
# the SwiftPM resource bundles under Contents/Resources (Bundle.module looks
# there; without them the gallery aborts at launch). Ad-hoc signed, so it runs.
#
# Usage: gallery.sh <swift build dir> <out dir>
set -eu

if [ "$#" -ne 2 ]; then
	echo "usage: $0 <swift build dir> <out dir>" >&2
	exit 2
fi
build_dir=$1
out_dir=$2
here=$(cd "$(dirname "$0")" && pwd)
app="$out_dir/HearthGallery.app"

rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
cp "$build_dir/HearthGallery" "$app/Contents/MacOS/HearthGallery"
for b in "$build_dir"/*.bundle; do
	[ -e "$b" ] || continue
	cp -R "$b" "$app/Contents/Resources/"
done
cp "$here/../../App/HearthGallery/Info.plist" "$app/Contents/Info.plist"
printf 'APPL????' >"$app/Contents/PkgInfo"
codesign --force --sign - "$app" >/dev/null 2>&1
echo "assembled $app"
