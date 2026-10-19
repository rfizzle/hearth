#!/bin/sh
# Submits the signed app to Apple's notary service, waits for the verdict,
# staples the ticket and validates it. The credentials are an App Store
# Connect API key: MACOS_NOTARY_ISSUER_ID, MACOS_NOTARY_KEY_ID and
# MACOS_NOTARY_KEY (the .p8 contents, as text or base64). They exist only in
# the Release workflow's secrets (docs/decisions.md, D18).
#
# Usage: notarize.sh <path to Hearth.app>
set -eu

if [ "$#" -ne 1 ]; then
	echo "usage: $0 <path to Hearth.app>" >&2
	exit 2
fi
app=$1

missing=
for v in MACOS_NOTARY_ISSUER_ID MACOS_NOTARY_KEY_ID MACOS_NOTARY_KEY; do
	eval "val=\${$v:-}"
	[ -n "$val" ] || missing="$missing $v"
done
if [ -n "$missing" ]; then
	echo "notarize: needs MACOS_NOTARY_ISSUER_ID, MACOS_NOTARY_KEY_ID and MACOS_NOTARY_KEY; missing:$missing" >&2
	exit 1
fi

cache_dir=${CACHE_DIR:-.cache}
sign_dir="$cache_dir/sign"
key="$sign_dir/AuthKey_$MACOS_NOTARY_KEY_ID.p8"
zip="$(dirname "$app")/Hearth.zip"
trap 'rm -f "$key" "$zip"' EXIT INT TERM

mkdir -p "$sign_dir"
(
	umask 077
	case $MACOS_NOTARY_KEY in
	-----BEGIN*) printf '%s\n' "$MACOS_NOTARY_KEY" >"$key" ;;
	*) printf '%s' "$MACOS_NOTARY_KEY" | base64 --decode >"$key" ;;
	esac
)

ditto -c -k --keepParent "$app" "$zip"
xcrun notarytool submit "$zip" --key "$key" --key-id "$MACOS_NOTARY_KEY_ID" --issuer "$MACOS_NOTARY_ISSUER_ID" --wait --timeout 20m
xcrun stapler staple "$app"
xcrun stapler validate "$app"
