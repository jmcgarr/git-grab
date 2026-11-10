# git-grab: fix and cleanup plan

This directory holds the plan of record. The fix and cleanup plan (specs 01
to 04) shipped as 0.5 and lives in [completed/](completed/). Open specs sit
at this level and follow the same shape: problem, changes, tests, acceptance.

The goal was a small, dependency-free CLI that works reliably, not a rewrite.
Each spec is a self-contained unit of work that can be done in one PR.

## Principles

- Stdlib only. No new dependencies.
- Keep `main.go` and `main_test.go` as the only source files unless a spec
  says otherwise.
- Preserve the existing directory scheme `~/Projects/<domain>/<owner>/<repo>`
  for every URL the current tests cover. Existing users' layouts must not move.
- Return errors; never panic on user input.
- Every behavior change ships with a table-driven test case.

## Order of work

| # | Spec | Why this order |
|---|------|----------------|
| 1 | [01-toolchain-and-build.md](completed/01-toolchain-and-build.md) (done) | Nothing built locally runs on macOS 26 until Go is upgraded. CI must be green before code changes land. |
| 2 | [02-parse-repo.md](completed/02-parse-repo.md) (done) | The core bug cluster. Panics and wrong paths on common URLs. |
| 3 | [03-cli-behavior.md](completed/03-cli-behavior.md) (done) | Option passthrough, home directory, exit codes. Depends on 02 returning errors. |
| 4 | [04-repo-hygiene.md](completed/04-repo-hygiene.md) (done) | README, module path, dead code, stale branches. Low risk, do last. |
| 5 | [05-homebrew-tap.md](05-homebrew-tap.md) | `brew install jmcgarr/tap/git-grab`. Designs a tap shared with `dirty` and future tools; most work is in the new tap repo. |
| 6 | [06-macos-signing.md](completed/06-macos-signing.md) (done) | Developer ID signing and notarization of the darwin binaries (issue #2), as a composite action `dirty` can call too. |

## Out of scope

- GoReleaser. Signing and notarization (issue #2) shipped in 0.6 via spec 06.
- Changing the parent shell's directory after clone (issue #1). Impossible from
  a child process. Spec 04 closes it with a documented shell wrapper.
- A config file. One environment variable is enough.

## Completed specs

When a spec's work is done, move its file into [completed/](completed/) with
`git mv` in the PR that finishes the work. The top level of this directory
lists only the specs with open work, so it always shows what is left.

## Definition of done

- `go vet ./...`, `gofmt -l .` (empty), and `go test ./...` pass on Go 1.26.
- CI runs on every push and PR and is green.
- `make all` produces working binaries for every listed platform.
- Every input in the table in spec 02 produces the listed result.
