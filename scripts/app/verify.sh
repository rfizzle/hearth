#!/bin/sh
# Checks what notarisation will check, before submitting: the reading
# service, the helper and the app carry the hardened runtime, the service is
# sandboxed, the bundle's signatures verify, and the helper has an
# LC_BUILD_VERSION load command. Reports whether a notarisation
# ticket is stapled without failing on its absence, since only the Release
# workflow can notarise.
#
# Usage: verify.sh <path to Hearth.app>
set -eu

if [ "$#" -ne 1 ]; then
	echo "usage: $0 <path to Hearth.app>" >&2
	exit 2
fi
app=$1
helper="$app/Contents/Helpers/hearthd"
reader="$app/Contents/XPCServices/com.rfizzle.hearth.reader.xpc"

for target in "$reader" "$helper" "$app"; do
	flags=$(codesign -dv --verbose=2 "$target" 2>&1 | grep 'flags=' || true)
	echo "$target: $flags"
	case $flags in
	*runtime*) ;;
	*)
		echo "$target is not signed with the hardened runtime" >&2
		exit 1
		;;
	esac
done
# The reader's entitlements must be exactly {app-sandbox: true}: a false value
# or any added entitlement (network, file access, a temporary exception) fails.
reader_ents=$(codesign -d --entitlements :- "$reader" 2>/dev/null | plutil -convert json -o - - 2>/dev/null || true)
if [ "$reader_ents" != '{"com.apple.security.app-sandbox":true}' ]; then
	echo "$reader entitlements are not exactly the sandbox: $reader_ents" >&2
	exit 1
fi
echo "$reader: sandboxed"
codesign --verify --deep --strict --verbose=2 "$app"
if ! xcrun vtool -show "$helper" | grep -q LC_BUILD_VERSION; then
	echo "$helper has no LC_BUILD_VERSION" >&2
	exit 1
fi
echo "$helper: LC_BUILD_VERSION present"
if xcrun stapler validate "$app" >/dev/null 2>&1; then
	echo "stapled"
else
	echo "not stapled (run make notarize)"
fi
