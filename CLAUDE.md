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

`specs/` holds the plan of record. Read `specs/README.md` before starting
work. Any open spec at the top level of `specs/` is the next thing to do,
in numeric order. New features start with a new spec (numbered from 05)
before any code. Update a spec if the implementation needs to deviate
from it, and record what the review found.

When a spec's work is complete, move its file from `specs/` to
`specs/completed/` in the same PR that finishes the work (`git mv`). Only
specs that still have open work live in the top level of `specs/`.

## Layout

- `main.go` holds everything. `main()` only calls `run(args, stdout, stderr)`
  and exits with its return value, so every path short of invoking git is
  testable. `splitArgs` separates git-clone options from the URL,
  `destination` resolves the clone directory, and the pure function
  `ParseRepo` (with helpers `splitHostPath`, `scpSyntax`, `domainOf`)
  computes the relative path.
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
- Cross-compile all platforms: `make all` (writes to `dist/`). It runs
  `clean`, `test`, and `vet` first.
- The Makefile sets `SHELL := /bin/bash`, so it works on Ubuntu (dash) too.
- CI (`.github/workflows/ci.yml`) runs fmt, vet, test, and `make all` on
  Ubuntu for every push and PR to `master`. Pushing a tag also creates a
  GitHub release with the `dist/` binaries attached.

## Environment

- `go.mod` declares Go 1.26. Any newer local Go works; `go` selects the
  toolchain automatically. Go 1.22 binaries abort on macOS 26, so do not
  downgrade `go.mod`.

## Behavior rules encoded in tests

- `ParseRepo` first splits the input into host and path, then derives
  everything from those two values. Never go back to splitting the whole
  string on `.`; that is what broke dotted repo names.
- Domain is the second-level label of the host: `stash.funville.com` ->
  `funville`, `github.com` -> `github`. Do not regress to the first label
  (that was issue #3). IP hosts keep the full address; single-label hosts
  such as `localhost` keep the label.
- Owner and any intermediate group segments (GitLab subgroups) are
  lowercased. Repo name keeps its case.
- Strip a trailing `.git` and trailing slashes. Dots elsewhere in the name
  (`foo.github.io`, `Mixed.Case.Repo`) must survive intact.
- Stash user repos: `~user` becomes `users/user`. A leading `scm/` path
  segment is ignored.
- Accept ssh, scp-style `git@host:owner/repo`, http(s) with `user@` or
  `user:pass@`, ports, and scheme-less `host/owner/repo`.
- Never panic on user input. Bare names, flags, local paths (absolute,
  `./`, `../`, Windows drive letters), `file:///` URLs, `.`/`..` path
  segments, and inputs without an `<owner>/<repo>` path return an error.

## Conventions

- gofmt formatting, tabs. `ParseRepo` stays exported so tests can call it.
- Clone output streams straight to the user's stdout and stderr; git-grab
  adds nothing on success. Stdin is attached so credential prompts work.
- Exit codes: 0 success, 1 git-grab's own errors (bad URL, destination
  exists, git not found), 2 usage, and git's own code when the clone fails.
  When git fails, print nothing extra; git already explained it.
- Options are walked by hand, not with the `flag` package, because every
  unknown option must be forwarded to `git clone`. The last argument not
  starting with `-` is the URL.
- `GIT_GRAB_ROOT` is the only configuration knob. Default is `~/Projects`
  via `os.UserHomeDir`, which works on Windows too.
- `git grab --help` is swallowed by git itself (rewritten to `git help
  grab`). Only `-h` works as a subcommand; `--help` works as `git-grab`.
- Keep the tool small. No config files, no new flags beyond `--help` and
  `--version` without a spec.
