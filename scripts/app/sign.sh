#!/bin/sh
# Signs the helper, then the app, with the hardened runtime and a timestamp
# (inside-out, as notarisation requires). With MACOS_SIGN_P12 set (a base64
# Developer ID .p12, password in MACOS_SIGN_PASSWORD) the identity is imported
# into a temporary keychain for this run only; without it the bundle is
# ad-hoc signed, which runs here and nowhere else. No Mac holds the identity:
# only the Release workflow has it (docs/decisions.md, D18).
#
# Usage: sign.sh <path to Hearth.app>
set -eu

if [ "$#" -ne 1 ]; then
	echo "usage: $0 <path to Hearth.app>" >&2
	exit 2
fi
app=$1
cache_dir=${CACHE_DIR:-.cache}
sign_dir="$cache_dir/sign"
keychain=${HEARTH_KEYCHAIN:-$sign_dir/hearth.keychain-db}
p12="$sign_dir/identity.p12"

cleanup() {
	if [ -n "${MACOS_SIGN_P12:-}" ]; then
		security delete-keychain "$keychain" >/dev/null 2>&1 || true
	fi
	rm -f "$p12"
}
trap cleanup EXIT INT TERM

if [ -n "${MACOS_SIGN_P12:-}" ]; then
	mkdir -p "$sign_dir"
	(umask 077 && printf '%s' "$MACOS_SIGN_P12" | base64 --decode >"$p12")
	kc_password=$(openssl rand -hex 16)
	security create-keychain -p "$kc_password" "$keychain"
	security set-keychain-settings -lut 21600 "$keychain"
	security unlock-keychain -p "$kc_password" "$keychain"
	security import "$p12" -k "$keychain" -P "${MACOS_SIGN_PASSWORD:-}" -T /usr/bin/codesign
	security set-key-partition-list -S apple-tool:,apple: -s -k "$kc_password" "$keychain" >/dev/null
	# shellcheck disable=SC2046 # the existing list is one path per word
	security list-keychains -d user -s "$keychain" $(security list-keychains -d user | tr -d '"')
	identity=$(security find-identity -v -p codesigning "$keychain" | awk '/Developer ID Application/ {print $2; exit}')
	if [ -z "$identity" ]; then
		echo "MACOS_SIGN_P12 holds no Developer ID Application identity" >&2
		exit 1
	fi
	set -- --keychain "$keychain"
else
	identity=-
	echo "no MACOS_SIGN_P12: ad-hoc signing (not distributable)"
	set --
fi

codesign --force --options runtime --timestamp --sign "$identity" "$@" "$app/Contents/Helpers/hearthd"
codesign --force --options runtime --timestamp --sign "$identity" "$@" "$app"
echo "signed $app"
