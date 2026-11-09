# 05: Homebrew tap for git-grab and friends

## Problem

Installing git-grab today means downloading a binary, renaming it, clearing
the macOS quarantine flag, and putting it on `PATH`. The same is true for
[`dirty`](https://github.com/jmcgarr/dirty), whose README has said
"Installation: coming soon" since 2021 and which builds on git-grab's
directory layout. Both tools should install with one command:

```
brew install jmcgarr/tap/git-grab
brew install jmcgarr/tap/dirty
```

The tap is a separate repository that will outlive any one tool, so this
spec designs it for N formulas, not one. git-grab and dirty are the first
two. `bard` (macOS-only Swift CLI) is a plausible third and is used below to
check that the design does not assume Go or cross-platform.

This spec lives in git-grab because git-grab is first in, but most of the
work happens in the new tap repo. `dirty`'s spec 007 already decided on a
tap at `jmcgarr/homebrew-tap` fed by per-platform release binaries plus a
`SHA256SUMS` file. This spec adopts those decisions so both tools fit the
same mold.

## Decisions

### Repository: `jmcgarr/homebrew-tap`

Homebrew requires the `homebrew-` prefix; users type the short form:
`brew tap jmcgarr/tap` then `brew install git-grab`, or the one-shot
`brew install jmcgarr/tap/git-grab`. Layout:

```
homebrew-tap/
  README.md                 what is here, how to install, how to add a formula
  Formula/
    git-grab.rb
    dirty.rb
  bin/
    update-formula          bump one formula to a release tag (see Automation)
  .github/workflows/
    tests.yml               audit + install + test on macOS and Linux
    update-formula.yml      bump a formula; triggered by the tool repos
```

Create it with `brew tap-new --no-git jmcgarr/tap`, copy the scaffold into
a fresh GitHub repo, and delete the generated `publish.yml` (bottle
publishing; not needed when formulas download prebuilt binaries).

### Formula shape: prebuilt binaries, not source builds

Two options were considered.

| | Source build | Prebuilt binaries |
|---|---|---|
| git-grab formula | `depends_on "go" => :build`, one tarball, one sha256 | four url/sha256 pairs (macOS and Linux, arm and intel) |
| dirty formula | needs GraalVM at install time; impractical | same four pairs |
| Install time | downloads Go toolchain (~70 MB) on first install | seconds |
| Per-release maintenance | 1 hash | 4 hashes per tool, so must be scripted |
| Gatekeeper | n/a (built locally) | brew downloads with curl; no quarantine attribute, so issue #2 disappears for brew users |

Prebuilt binaries win because dirty cannot reasonably be built from source
inside Homebrew, and one formula shape means one update script. The cost,
four hashes per release, is paid by automation below.

Homebrew core forbids prebuilt binaries; third-party taps may ship them.
`brew audit --strict` passes for this shape.

### `Formula/git-grab.rb`

```ruby
class GitGrab < Formula
  desc "Clone git repositories into ~/Projects/<domain>/<owner>/<repo>"
  homepage "https://github.com/jmcgarr/git-grab"
  version "0.5"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/jmcgarr/git-grab/releases/download/#{version}/git-grab-darwin-arm64"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
    on_intel do
      url "https://github.com/jmcgarr/git-grab/releases/download/#{version}/git-grab-darwin-amd64"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/jmcgarr/git-grab/releases/download/#{version}/git-grab-linux-arm64"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
    on_intel do
      url "https://github.com/jmcgarr/git-grab/releases/download/#{version}/git-grab-linux-amd64"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  livecheck do
    url :stable
    strategy :github_latest
  end

  def install
    binary = Dir["git-grab-*"].first
    chmod 0755, binary
    bin.install binary => "git-grab"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/git-grab --version")
    assert_match "cannot determine host", shell_output("#{bin}/git-grab --depth 1 2>&1", 1)
  end
end
```

Notes:

- `version` must be explicit because Homebrew cannot infer it from a bare
  binary URL.
- The downloaded file is not an archive, so Homebrew drops it into the build
  directory under its asset name. `chmod` because GitHub release assets do
  not preserve the executable bit.
- `test do` runs offline. `--version` must print the tag; the release
  binaries are built with `-X main.version=$(git describe --tags)`, which is
  exactly the tag on a tag build.
- `livecheck` lets `brew livecheck --tap jmcgarr/tap` report formulas that
  are behind their latest GitHub release. The scheduled fallback below uses
  it.

### `Formula/dirty.rb`

Same skeleton with these differences, all of which the update script
tolerates because it reads URLs from the formula rather than assuming a
naming convention:

- Tags are `v0.0.5`; the formula has `version "0.0.5"` and URLs use
  `v#{version}`.
- Asset names follow dirty's spec 007: `dirty-macos-aarch64`,
  `dirty-linux-x86_64`, and so on. Intel macOS and Linux arm64 are optional
  there; the formula includes only the blocks for assets that exist, and
  `brew install` on an unsupported platform fails with Homebrew's standard
  message.
- `test do` checks `--version` and runs `dirty` against an empty
  `GIT_GRAB_ROOT=#{testpath}` expecting no dirty repos.
- No license is declared in the dirty repo today. Add one there (Apache-2.0
  to match git-grab, or whatever the maintainer prefers) before the formula
  lands, or `brew audit --strict` fails.

A future macOS-only tool such as `bard` adds `depends_on :macos` and
`depends_on arch: :arm64`, drops the `on_linux` block, and otherwise fits.

### Automation: how a release reaches the tap

Each tool keeps its own release pipeline. The tap owns the formula update.

1. **The tool's release job publishes `SHA256SUMS`** next to its binaries.
   git-grab: add `(cd dist && shasum -a 256 * > SHA256SUMS)` to the `make all`
   step's successor in `ci.yml` before upload. dirty: already in spec 007.
2. **`bin/update-formula <formula> <tag>`** in the tap repo (bash):
   - `version` is the tag with a leading `v` removed.
   - Fetch `SHA256SUMS` from the release. If absent (git-grab 0.5 has none),
     download each referenced asset and hash it locally.
   - Rewrite the `version "..."` line. For every `url` line, substitute the
     new version into the URL template, take the basename, look it up in the
     sums, and rewrite the `sha256` line that follows. Fail if any asset is
     missing from the release.
   - Run `brew style`, `brew audit --strict`, `brew install --verbose`, and
     `brew test` on the result.
   - Commit as `git-grab 0.6` on `main`.
3. **`update-formula.yml`** runs the script on `macos-latest`. Triggers:
   - `workflow_dispatch` with inputs `formula` and `tag`, so a human can bump
     from the Actions tab.
   - `schedule` daily: `brew livecheck --tap jmcgarr/tap --json` lists
     outdated formulas; bump each. Zero secrets, 24-hour worst case.
4. **Each tool's release job triggers the bump** right after assets upload:
   ```yaml
   - run: gh workflow run update-formula.yml -R jmcgarr/homebrew-tap
            -f formula=git-grab -f tag=${{ github.ref_name }}
     env:
       GH_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
   ```
   `HOMEBREW_TAP_TOKEN` is one fine-grained PAT scoped to the single repo
   `jmcgarr/homebrew-tap` with Actions: read and write. It is stored as a
   secret in each tool repo. If it is ever missing or expired the step fails
   visibly and the daily schedule catches up.

The script commits straight to `main` after the in-job checks pass, rather
than opening a PR. A PR opened with `GITHUB_TOKEN` does not trigger
`pull_request` workflows, so auto-merge would need a second PAT. Direct
commit keeps the tap at one secret, held by the tool repos. `tests.yml` runs
on every push to `main` too, so a bad bump is still red on the badge.

### `tests.yml`

On `push` to `main` and `pull_request`: matrix of `macos-latest` and
`ubuntu-latest` (Linuxbrew via `Homebrew/actions/setup-homebrew`). Steps:
`brew tap jmcgarr/tap "$GITHUB_WORKSPACE"`, then for each changed or all
formulas: `brew style`, `brew audit --strict --online`, `brew install`,
`brew test`. Intel macOS and Linux arm64 are not covered by runners here;
the formula blocks for them are validated only by `brew audit`'s URL and
checksum checks, which do download the assets.

### Changes in git-grab

- `ci.yml`: generate `SHA256SUMS` in `dist/` after `make all`; add the
  `gh workflow run` step to the release job, guarded so a missing secret
  fails that step only.
- README: lead the Install section with `brew install jmcgarr/tap/git-grab`;
  move binary download to second place and note brew avoids the quarantine
  step.
- Known issues: issue #2 becomes "only affects direct downloads".

### Changes in dirty

Covered by its spec 007 (multi-platform assets, `SHA256SUMS`,
version-from-tag). Add the `gh workflow run` step and `HOMEBREW_TAP_TOKEN`
secret there once a release with those assets exists, and a LICENSE file.

## Rollout

1. git-grab PR: `SHA256SUMS` in release, README install section. Small, no
   dependency on the tap.
2. Create `jmcgarr/homebrew-tap` from `brew tap-new`. Add `git-grab.rb` with
   hashes computed from the existing 0.5 assets. Add `tests.yml`. Verify on
   this Mac: `brew install jmcgarr/tap/git-grab && git grab --version`.
3. Add `bin/update-formula` and `update-formula.yml`. Create the PAT, add it
   to git-grab, add the dispatch step. Tag git-grab `0.6` and watch the
   formula update itself.
4. When dirty's spec 007 ships a release: add `dirty.rb`, its PAT secret, and
   its dispatch step.
5. Both READMEs point at the tap.

## Acceptance

- `brew install jmcgarr/tap/git-grab` on a clean macOS arm64 machine installs
  and `git grab --version` prints the tag. Same on Linux via Linuxbrew.
- Tagging a git-grab release updates `Formula/git-grab.rb` on the tap's
  `main` within a few minutes with no manual step, and the tap's tests stay
  green.
- `brew audit --strict --online` passes for every formula in the tap.
- Adding a third formula requires only a new `Formula/*.rb` and a dispatch
  step in that tool's release job. No change to the script or workflows.

## Open questions

- Tap name: `homebrew-tap` (`brew install jmcgarr/tap/x`) is the common
  convention. `homebrew-tools` is the alternative if "tap" reads oddly.
  Recommendation: `tap`.
- Is a 24-hour scheduled fallback with no secrets enough, skipping the PAT
  and dispatch step entirely? Recommendation: do the dispatch; the PAT is
  one secret and releases become installable immediately.
- Should the git-grab release pipeline stop building FreeBSD and OpenBSD
  binaries now that the install story is brew plus `go install`?
  Recommendation: keep them; they cost nothing and the formula ignores them.
