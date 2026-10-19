#!/bin/sh
# Assembles Hearth.app from a SwiftPM build: the executable, the Go helper
# under Contents/Helpers, every SwiftPM resource bundle under
# Contents/Resources (Bundle.module looks there; without them the app aborts
# at launch), and Info.plist rendered from App/Hearth/Info.plist.
#
# Usage: bundle.sh <swift build dir> <helper binary> <version> <out dir>
set -eu

if [ "$#" -ne 4 ]; then
	echo "usage: $0 <swift build dir> <helper binary> <version> <out dir>" >&2
	exit 2
fi
build_dir=$1
helper=$2
version=$3
out_dir=$4
here=$(cd "$(dirname "$0")" && pwd)
plist="$here/../../App/Hearth/Info.plist"
app="$out_dir/Hearth.app"

rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Helpers" "$app/Contents/Resources"
cp "$build_dir/Hearth" "$app/Contents/MacOS/Hearth"
for b in "$build_dir"/*.bundle; do
	[ -e "$b" ] || continue
	cp -R "$b" "$app/Contents/Resources/"
done
cp "$helper" "$app/Contents/Helpers/hearthd"
chmod 755 "$app/Contents/Helpers/hearthd"
sed "s/__VERSION__/$version/g" "$plist" >"$app/Contents/Info.plist"
printf 'APPL????' >"$app/Contents/PkgInfo"
echo "assembled $app"
