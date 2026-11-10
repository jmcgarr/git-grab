#!/usr/bin/env bash
# Sign and notarize bare Mach-O binaries with a Developer ID Application
# identity.
#
#   sign-macos.sh FILE...
#
# Environment (all optional):
#   MACOS_CERTIFICATE           base64 .p12 with the certificate and private key
#   MACOS_CERTIFICATE_PASSWORD  its password (required when the above is set)
#   NOTARY_KEY_ID               App Store Connect API key ID
#   NOTARY_ISSUER_ID            App Store Connect issuer ID
#   NOTARY_KEY                  base64 .p8 API key
#
# With MACOS_CERTIFICATE set, the certificate is imported into a throwaway
# keychain that is deleted on exit. Without it, a Developer ID Application
# identity already in the user's keychains is used (local dry runs); if there
# is none, the files are left untouched and the script exits 0 with a warning.
# Without the three NOTARY_* values, files are signed but not notarized.
#
# Secrets are read only from the environment and written only to files under
# a private temp directory that is removed on exit. The one exception is
# `security import -P`, which has no non-interactive alternative; the
# password is briefly visible in the process list on the (ephemeral) runner.
set -euo pipefail

die()  { echo "::error::$*" >&2; exit 1; }
warn() { echo "::warning::$*" >&2; }
note() { echo "==> $*" >&2; }

[ $# -gt 0 ] || die "usage: sign-macos.sh FILE..."
for f in "$@"; do [ -f "$f" ] || die "not a file: $f"; done

tmp=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/sign-macos.XXXXXX")
chmod 700 "$tmp"
keychain=""
orig_search_list=""
cleanup() {
  if [ -n "$keychain" ]; then
    if [ -n "$orig_search_list" ]; then
      # shellcheck disable=SC2086
      security list-keychains -d user -s $orig_search_list 2>/dev/null || true
    fi
    security delete-keychain "$keychain" 2>/dev/null || true
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

# --- identity -----------------------------------------------------------

# codesign takes --keychain; the security subcommands take the keychain as a
# positional argument. Both arrays are empty when no temp keychain exists.
codesign_kc=()
security_kc=()
if [ -n "${MACOS_CERTIFICATE:-}" ]; then
  [ -n "${MACOS_CERTIFICATE_PASSWORD:-}" ] || die "MACOS_CERTIFICATE is set but MACOS_CERTIFICATE_PASSWORD is not"
  keychain="$tmp/sign.keychain-db"
  kcpass=$(openssl rand -hex 24)
  note "creating temporary keychain"
  security create-keychain -p "$kcpass" "$keychain"
  security set-keychain-settings -lut 1200 "$keychain"
  security unlock-keychain -p "$kcpass" "$keychain"
  # Put it on the user search list so codesign can resolve the Developer ID
  # intermediate CA through the normal chain; the trap restores the list.
  orig_search_list=$(security list-keychains -d user | tr -d '" ')
  # shellcheck disable=SC2086  # orig_search_list is a whitespace-separated list of paths
  security list-keychains -d user -s "$keychain" $orig_search_list
  printf '%s' "$MACOS_CERTIFICATE" | base64 -d > "$tmp/cert.p12"
  security import "$tmp/cert.p12" -k "$keychain" -P "$MACOS_CERTIFICATE_PASSWORD" \
    -T /usr/bin/codesign -T /usr/bin/security >/dev/null
  rm -f "$tmp/cert.p12"
  # Let codesign use the key without a UI prompt.
  security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$kcpass" "$keychain" >/dev/null
  codesign_kc=(--keychain "$keychain")
  security_kc=("$keychain")
else
  warn "MACOS_CERTIFICATE not set; using any Developer ID Application identity in the current keychains"
fi

# Identities are selected by SHA-1 hash, not name: a developer keychain often
# holds a renewed certificate next to the old one under the same name, and
# codesign refuses an ambiguous name. With several matches, use the one that
# expires last.
found=$(security find-identity -v -p codesigning "${security_kc[@]+"${security_kc[@]}"}")
hashes=()
while read -r h; do [ -n "$h" ] && hashes+=("$h"); done <<<"$(
  awk '/"Developer ID Application: /{print $2}' <<<"$found" | sort -u
)"
if [ ${#hashes[@]} -eq 0 ]; then
  if [ -n "$keychain" ]; then
    die "certificate imported but no Developer ID Application identity found in it"
  fi
  warn "no Developer ID Application identity available; leaving $# file(s) unsigned"
  exit 0
fi

cert_expiry() { # cert_expiry SHA1 -> epoch seconds of notAfter
  local end
  end=$(security find-certificate -a -Z -c "Developer ID Application" -p "${security_kc[@]+"${security_kc[@]}"}" \
    | awk -v h="$1" '/^SHA-1 hash:/{cur=$3} /^-----BEGIN/{p=(cur==h)} p{print} /^-----END/{p=0}' \
    | openssl x509 -noout -enddate 2>/dev/null | cut -d= -f2)
  [ -n "$end" ] || die "cannot read the expiry of certificate $1"
  date -j -f "%b %e %T %Y %Z" "$end" +%s
}

identity=${hashes[0]}
if [ ${#hashes[@]} -gt 1 ]; then
  best=0
  for h in "${hashes[@]}"; do
    e=$(cert_expiry "$h")
    if [ "$e" -gt "$best" ]; then best=$e; identity=$h; fi
  done
  warn "${#hashes[@]} Developer ID Application identities found; using $identity (latest expiry)"
fi
identity_name=$(awk -v h="$identity" '$2 == h {sub(/^[^"]*"/, ""); sub(/"[^"]*$/, ""); print; exit}' <<<"$found")
note "signing as $identity_name ($identity)"

# --- sign ---------------------------------------------------------------

for f in "$@"; do
  # --force replaces the ad-hoc signature the Go linker adds on arm64.
  codesign --force --options runtime --timestamp --sign "$identity" \
    "${codesign_kc[@]+"${codesign_kc[@]}"}" "$f"
  codesign --verify --strict --verbose=2 "$f"
  note "signed $f"
done

# --- notarize -----------------------------------------------------------

if [ -z "${NOTARY_KEY:-}" ] || [ -z "${NOTARY_KEY_ID:-}" ] || [ -z "${NOTARY_ISSUER_ID:-}" ]; then
  warn "NOTARY_KEY, NOTARY_KEY_ID, or NOTARY_ISSUER_ID not set; files are signed but not notarized"
  exit 0
fi

p8="$tmp/AuthKey.p8"
printf '%s' "$NOTARY_KEY" | base64 -d > "$p8"
notary=(xcrun notarytool --key "$p8" --key-id "$NOTARY_KEY_ID" --issuer "$NOTARY_ISSUER_ID")

json_field() { # json_field NAME <<< JSON  (flat string fields only)
  sed -n "s/.*\"$1\" *: *\"\([^\"]*\)\".*/\1/p" | head -1
}

for f in "$@"; do
  zip="$tmp/$(basename "$f").zip"
  ditto -c -k --keepParent "$f" "$zip"
  note "submitting $(basename "$f") for notarization"
  # notarytool exits non-zero on timeouts, auth errors, and (in recent
  # versions) an Invalid verdict; keep its output either way so the log can
  # be fetched.
  result=$("${notary[@]}" submit "$zip" --wait --timeout 30m --output-format json 2>&1) || true
  rm -f "$zip"
  status=$(json_field status <<<"$result")
  id=$(json_field id <<<"$result")
  if [ "$status" != "Accepted" ]; then
    echo "$result" >&2
    if [ -n "$id" ]; then "${notary[@]}" log "$id" >&2 || true; fi
    die "notarization of $f ended with status '${status:-unknown}'"
  fi
  note "notarized $f (submission $id)"

  # Confirm the ticket is visible. spctl --assess rejects every bare
  # executable ("does not seem to be an app"), so ask codesign instead; the
  # ticket is fetched from Apple and can take a moment to propagate.
  for attempt in 1 2 3 4 5 6; do
    if check=$(codesign --verify --verbose -R="notarized" --check-notarization "$f" 2>&1); then
      break
    fi
    [ "$attempt" -lt 6 ] || die "$f does not verify as notarized:"$'\n'"$check"
    sleep 10
  done
  note "verified notarized: $f"
done
