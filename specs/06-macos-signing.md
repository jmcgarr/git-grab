# 06: Sign and notarize the macOS binaries

## Problem

A git-grab binary downloaded from the releases page in a browser carries
the quarantine attribute. Because it is not signed with a Developer ID and
not notarized, Gatekeeper blocks it until the user runs
`xattr -d com.apple.quarantine`. That is issue #2, open since 2022.
Homebrew and `go install` users are unaffected; this is only about direct
downloads, but those are the first thing a new user tries.

`dirty` has the same problem and its spec 007 decided on Developer ID
signing plus notarization. Its `codesign.yml` is half finished: literal
keychain password, hard-coded identity hash, the `codesign` call commented
out, no notarization, the signed file never re-uploaded, and a typo in the
secret name (`MAOS_CERTIFICATE_PWD`). Solving this once, in a form both
repos call, is the point of this spec.

## What exists already

- Two **Developer ID Application** certificates for team `WNP822AXPW` are
  in the maintainer's login keychain under the same name: one valid until
  2027-02-01 (hash `AD33...`) and a renewal valid until 2030-04-07 (hash
  `8801...`). No new Apple enrollment is needed. The `.p12` first exported
  for CI holds the 2027 one; re-exporting the 2030 one and updating
  `MACOS_CERTIFICATE` postpones the renewal chore.
- An App Store Connect API key for `notarytool` does **not** exist yet (or
  is not recorded). Creating one is a maintainer step below.
- dirty holds `MACOS_CERTIFICATE` and `MAOS_CERTIFICATE_PWD` secrets from
  April 2025. Their contents are unknown; re-export rather than guess.

## Decisions

### Sign and notarize bare binaries; no installer package

`codesign` the two darwin binaries with the Developer ID Application
identity, hardened runtime, and a secure timestamp. Submit each to Apple's
notary service with `xcrun notarytool submit --wait`. Upload the signed
binaries exactly as today.

A bare Mach-O cannot have the notarization ticket stapled to it; only
bundles, `.pkg`, and `.dmg` can. Without a staple, Gatekeeper fetches the
ticket from Apple on first run. That is fine for a CLI tool: anyone who
just downloaded it is online. A `.pkg` installer would allow stapling but
puts a file in `/usr/local/bin` through a GUI installer, which is the wrong
shape for a git subcommand. A universal binary via `lipo` is possible later
and is out of scope here.

### A composite action, shared by both repos

`.github/actions/sign-macos/action.yml` in git-grab, wrapping a bash
script `sign-macos.sh` so it can also run locally. Inputs:

| Input | Required | Meaning |
|---|---|---|
| `files` | yes | shell glob of Mach-O files to sign, e.g. `dist/git-grab-darwin-*` |
| `certificate` | no | base64 `.p12` with the Developer ID Application cert and private key |
| `certificate-password` | no | password for the `.p12` |
| `notary-key-id` | no | App Store Connect API key ID |
| `notary-issuer-id` | no | App Store Connect issuer ID |
| `notary-key` | no | base64 `.p8` API key |

When `certificate` is empty the action prints a `::warning::` and exits 0
without touching the files. That keeps tag builds working in forks and
lets the pipeline be dry-run before the secrets exist. When the
certificate is present but the notary inputs are not, it signs and skips
notarization with a warning.

Script steps, all in one bash process so a `trap ... EXIT` can clean up:

1. `set -euo pipefail`; never `set -x`. Secrets arrive as environment
   variables, never as arguments.
2. Keychain password from `openssl rand -hex 24`. `security create-keychain`,
   `unlock-keychain`, `set-keychain-settings -lut 1200` (auto-lock after 20
   minutes), add it to the search list.
3. Decode the `.p12` to a temp file, `security import ... -T /usr/bin/codesign
   -T /usr/bin/security`, then `security set-key-partition-list -S
   apple-tool:,apple:,codesign: -s -k "$password"` so codesign can use the
   key without a UI prompt.
4. Identity: `security find-identity -v -p codesigning "$keychain"` filtered
   to `Developer ID Application`. Fail if zero or more than one match. No
   identity secret; the certificate is the source of truth.
5. For each file: `codesign --force --options runtime --timestamp --sign
   "$identity" "$f"`, then `codesign --verify --strict --verbose=2 "$f"`.
   `--force` replaces the ad-hoc signature the Go linker puts on arm64.
6. Notarize: `ditto -c -k --keepParent "$f" "$f.zip"`; `xcrun notarytool
   submit "$f.zip" --key "$p8" --key-id "$id" --issuer "$issuer" --wait
   --timeout 30m`. On any status other than `Accepted`, run `notarytool log`
   for the submission ID and fail. Delete the zip; the binary itself is what
   ships.
7. Confirm the ticket is visible (see implementation notes: `codesign
   --check-notarization`, not `spctl`). This is the real acceptance check
   and runs on the runner before upload.
8. Trap: `security delete-keychain`, remove the `.p12`, `.p8`, and zips.
   Runs on success and failure.

Composite action steps cannot use `if: always()`, which is why cleanup
lives in the script's trap rather than a separate step.

### Where it runs in git-grab

`ci.yml` gains a job between `check` and `release`:

```yaml
  sign-macos:
    if: startsWith(github.ref, 'refs/tags/')
    needs: check
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4          # for the local action
      - uses: actions/download-artifact@v4
        with: { name: dist, path: dist }
      - uses: ./.github/actions/sign-macos
        with:
          files: dist/git-grab-darwin-*
          certificate: ${{ secrets.MACOS_CERTIFICATE }}
          certificate-password: ${{ secrets.MACOS_CERTIFICATE_PASSWORD }}
          notary-key-id: ${{ secrets.NOTARY_KEY_ID }}
          notary-issuer-id: ${{ secrets.NOTARY_ISSUER_ID }}
          notary-key: ${{ secrets.NOTARY_KEY }}
      - run: cd dist && rm -f SHA256SUMS && shasum -a 256 * > SHA256SUMS
      - uses: actions/upload-artifact@v4
        with: { name: dist-release, path: dist/ }
```

`release` then `needs: sign-macos` and downloads `dist-release`. The
`check` job is unchanged and still runs on PRs with no secrets.

Signing changes the bytes, so `SHA256SUMS` is regenerated after signing.
The Homebrew formula hashes come from that file after the release exists,
so the tap flow from spec 05 needs no change; brew users get the signed
binaries too.

### Secret names, identical in both repos

| Secret | Contents |
|---|---|
| `MACOS_CERTIFICATE` | base64 of the `.p12` export |
| `MACOS_CERTIFICATE_PASSWORD` | the `.p12` password |
| `NOTARY_KEY_ID` | App Store Connect API key ID (10 chars) |
| `NOTARY_ISSUER_ID` | App Store Connect issuer ID (UUID) |
| `NOTARY_KEY` | base64 of the `.p8` file |

dirty's `MAOS_CERTIFICATE_PWD` is replaced, not renamed, when dirty adopts
the action.

### Security rules for the implementation PR

- No secret value is ever a command-line argument, echoed, or written to a
  file that outlives the job. Temp files live under `$RUNNER_TEMP` and are
  removed by the trap. One documented exception: `security import -P` has
  no non-interactive alternative, so the `.p12` password is briefly an
  argument on the single-tenant, ephemeral runner.
- Workflow inputs and `github.*` values reach shell steps only through
  environment variables, never as `${{ }}` templates inside `run:` text.
- The signing job runs only on tag pushes. GitHub does not expose secrets to
  `pull_request` runs from forks anyway; the `if` makes it explicit.
- The maintainer exports the certificate and creates the API key
  personally and sets the secrets with `gh secret set`. The assistant never
  sees the `.p12`, its password, or the `.p8`.
- Run the `security-review` skill on the branch before opening the PR.

## Implementation notes (2026-11-10)

Deviations from the algorithm above, found while implementing and in review:

- **Identity selection.** Identities are selected by SHA-1 hash, not name,
  and when several `Developer ID Application` identities match, the one
  expiring last is used with a warning rather than failing. The maintainer's
  login keychain holds a renewed certificate next to the old one under the
  same name; signing by name is ambiguous and "fail on more than one" would
  make the local dry run impossible. In CI the `.p12` holds one identity, so
  the rule is moot there.
- **Keychain handling.** `security find-identity` and `find-certificate`
  take the keychain positionally; only `codesign` takes `--keychain`. The
  temp keychain is added to the front of the user search list so the
  Developer ID intermediate resolves through the normal chain, and the
  original list is restored in the trap.
- **Verification uses `codesign`, not `spctl`.** `spctl --assess --type
  execute` rejects every bare Mach-O with "does not seem to be an app", even
  notarized ones, so it cannot be the acceptance check. The script runs
  `codesign --verify --verbose -R="notarized" --check-notarization`, which
  exits 0 only when Apple's ticket is present, with retries for propagation.
  The acceptance criteria below are updated to match.
- **`notarytool` exit status.** Its output is captured even on non-zero exit
  so the `Invalid` log can be fetched.
- **`security import -P`** is the one secret on a command line; see the
  security rules.
- **README** keeps a Known Issues line for pre-0.6 downloads rather than
  deleting it, since 0.5 binaries stay on the releases page.

## Maintainer steps (cannot be automated)

1. **Export the certificate.** Keychain Access, login keychain, My
   Certificates, `Developer ID Application: Joseph Mcgarr (WNP822AXPW)`.
   Expand it, select both the certificate and its private key, File >
   Export Items, format `.p12`, choose a strong password. Then:
   ```sh
   base64 -i DeveloperID.p12 | gh secret set MACOS_CERTIFICATE -R jmcgarr/git-grab
   gh secret set MACOS_CERTIFICATE_PASSWORD -R jmcgarr/git-grab   # prompts
   rm DeveloperID.p12
   ```
2. **Create the notary API key.** appstoreconnect.apple.com > Users and
   Access > Integrations > App Store Connect API > Team Keys > Generate.
   Name it `notarytool`, role **Developer**. Record the Key ID and the
   Issuer ID shown at the top of the page. Download the `.p8` (allowed once).
   ```sh
   gh secret set NOTARY_KEY_ID -R jmcgarr/git-grab
   gh secret set NOTARY_ISSUER_ID -R jmcgarr/git-grab
   base64 -i AuthKey_XXXXXXXXXX.p8 | gh secret set NOTARY_KEY -R jmcgarr/git-grab
   ```
   Keep the `.p8` somewhere safe (a password manager); it is needed again
   for dirty and cannot be downloaded a second time.
3. Repeat the five `gh secret set` commands with `-R jmcgarr/dirty` when
   dirty adopts the action.

## Tests

- **Local dry run before any secrets exist.** `sign-macos.sh` detects a
  `Developer ID Application` identity already in the login keychain when
  `MACOS_CERTIFICATE` is unset, so on the maintainer's Mac it can sign a
  locally built binary and pass `codesign --verify`. Notarization is
  skipped with a warning. This shakes out everything except the notary
  round trip.
- **Pipeline dry run without secrets.** Push tag `0.6-rc1`. The sign job
  warns and passes the binaries through unsigned; the release is created
  as before. Delete the tag and release.
- **Full run.** After secrets are set, push `0.6-rc2`. Check the job log for
  `verified notarized:` on both files. Download `git-grab-darwin-arm64` from
  the release page in Safari, `chmod +x`, run `./git-grab-darwin-arm64
  --version`. No Gatekeeper dialog and no `xattr` needed. Delete tag and
  release.
- Unit-level: the script's identity selection fails clearly with zero or
  two Developer ID identities. Covered by running it against the login
  keychain with a bogus filter in a shell test, if cheap; otherwise by
  reading.

## Acceptance

- Tag builds produce darwin binaries where `codesign -dvv` shows
  `Authority=Developer ID Application: Joseph Mcgarr (WNP822AXPW)`, a
  `Timestamp=`, and `flags=0x10000(runtime)`.
- `codesign --verify -R="notarized" --check-notarization` on a freshly
  downloaded release binary exits 0.
- A tag build with no signing secrets still produces a release, with a
  visible warning in the job log.
- README's install section drops the `xattr` instruction for current
  releases and the Known Issues entry for #2. Issue #2 is closed.
- dirty's release workflow calls
  `jmcgarr/git-grab/.github/actions/sign-macos@master` (or a pinned tag),
  its `codesign.yml` is deleted, and its spec 007 codesign section points
  here. That part ships in dirty, not in this repo.

## Rollout

1. Merge this spec.
2. PR in git-grab: the action and script, `ci.yml` changes, README. Local
   dry run and the no-secrets `0.6-rc1` dry run happen on that branch.
3. Maintainer sets the five secrets. `0.6-rc2` full run, verified from a
   browser download. Delete rc tags and releases.
4. Tag `0.6`. Release notes: "macOS binaries are now signed and notarized;
   no quarantine step". Close #2.
5. dirty adopts the action per its spec 007.

## Out of scope

- Windows Authenticode signing.
- `.pkg` or `.dmg` distribution; universal binary. Either could be a later
  spec if stapling turns out to matter.
- Moving the action to a dedicated `jmcgarr/actions` repository. Callers
  reference it by path, so moving later is a one-line change in each.
