# git-grab

Single-package Go CLI. `git grab <repo-url>` clones a repo into
`~/Projects/<domain>/<owner>/<repo>` so repos are organized by origin.
It behaves like `git clone`; only the destination is computed.

## Git workflow (required)

- Never commit directly to `master`. Always create a branch first, commit
  there, and open a pull request. This applies to every change, including
  docs and one-line fixes.
- Branch names: short kebab-case describing the change, e.g. `fix-trailing-slash`.
- Push the branch and open the PR in the same step as committing. A commit
  that is not on an open PR is unfinished work.
- Commit messages are short plain sentences, no type prefix. Reference issue
  numbers where one exists (`Fixes #3`).
- Releases are tags `0.N` on `master` after the PR merges.

## Plan of record

`specs/` holds the current fix and cleanup plan. Read `specs/README.md`
before starting work and follow the specs in order. Update a spec if the
implementation needs to deviate from it.

## Layout

- `main.go` holds everything: `main()` and the pure function `ParseRepo`.
- `main_test.go` is the only test file. Its expectations define the
  directory scheme; treat them as the spec.
- `dist/` and `.idea/` are gitignored build and IDE output. Never commit them.
- Stdlib only. v0.4 deliberately removed the last third-party dependency.
  Do not add new ones.

## Commands

- Build: `go build -o git-grab .` Never run bare `go build ./...` in the
  root; it leaves an untracked `git-grab` binary.
- Test: `go test -cover ./...`
- Lint: `go vet ./...` and `gofmt -l .` (must print nothing)
- Cross-compile all platforms: `make all` (writes to `dist/`).
- The current Makefile uses bash parameter expansion and needs bash, not
  dash. CI runs on macOS for this reason until spec 01 lands.

## Environment

- Go 1.22.0 binaries abort on macOS 26 with `missing LC_UUID load command`.
  Until `go.mod` is bumped, use `GOTOOLCHAIN=go1.26.0 go test ./...`.

## Behavior rules encoded in tests

- Domain is the second-level label of the host: `stash.funville.com` ->
  `funville`, `github.com` -> `github`. Do not regress to the first label
  (that was issue #3).
- Owner is lowercased. Repo name keeps its case.
- Strip a trailing `.git`. Names like `foo.github.io` must survive intact.
- Stash user repos: `~user` becomes `users/user`. A leading `scm/` path
  segment is ignored.
- Accept ssh, scp-style `git@host:owner/repo`, http(s) with `user@`, and ports.
- Never panic on user input. Return an error (spec 02).

## Conventions

- gofmt formatting, tabs. `ParseRepo` stays exported so tests can call it.
- Clone output streams straight to the user's stdout and stderr; git-grab
  adds nothing on success.
- Keep the tool small. No config files, no new flags beyond `--help` and
  `--version` without a spec.
