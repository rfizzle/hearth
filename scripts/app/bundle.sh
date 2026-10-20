#!/bin/sh
# Assembles Hearth.app from a SwiftPM build: the executable, the Go helper
# under Contents/Helpers, the reading service under Contents/XPCServices,
# every SwiftPM resource bundle under Contents/Resources (Bundle.module looks
# there; without them the app aborts at launch), and Info.plist rendered from
# App/Hearth/Info.plist. The reading service is signed here, ad-hoc, with its
# sandbox entitlement: an XPC service that is not sandboxed would read with
# the app's own rights. make app-sign signs it again with the identity.
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
reader_dir="$here/../../App/HearthReader"
app="$out_dir/Hearth.app"
reader="$app/Contents/XPCServices/com.rfizzle.hearth.reader.xpc"

rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Helpers" "$app/Contents/Resources"
cp "$build_dir/Hearth" "$app/Contents/MacOS/Hearth"
for b in "$build_dir"/*.bundle; do
	[ -e "$b" ] || continue
	cp -R "$b" "$app/Contents/Resources/"
done
cp "$helper" "$app/Contents/Helpers/hearthd"
chmod 755 "$app/Contents/Helpers/hearthd"
mkdir -p "$reader/Contents/MacOS"
cp "$build_dir/HearthReader" "$reader/Contents/MacOS/HearthReader"
sed "s/__VERSION__/$version/g" "$reader_dir/Info.plist" >"$reader/Contents/Info.plist"
codesign --force --options runtime --sign - \
	--entitlements "$reader_dir/HearthReader.entitlements" "$reader"
sed "s/__VERSION__/$version/g" "$plist" >"$app/Contents/Info.plist"
printf 'APPL????' >"$app/Contents/PkgInfo"
echo "assembled $app"
