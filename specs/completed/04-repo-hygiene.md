# 04: README, releases, and repository hygiene

## README

Rewrite `README.md` to cover, in this order:

1. One-paragraph description and the directory scheme with the existing tree
   example.
2. **Install.**
   - `go install github.com/jmcgarr/git-grab@latest` (works after spec 01
     renames the module).
   - Or download a binary from the releases page, rename it to `git-grab`, make
     it executable, and put it on `PATH`. On macOS add:
     `xattr -d com.apple.quarantine git-grab` for unsigned downloads.
3. **Usage.** The usage text from spec 03, plus an example with `--depth 1`.
4. **Configuration.** `GIT_GRAB_ROOT`.
5. **Changing into the new directory.** A shell function for issue #1:
   ```sh
   grab() { git grab "$@" && cd "$(git grab --print-dest "${@: -1}")"; }
   ```
   Only include this if `--print-dest` is added. If not, document that the
   destination is printed by git's own `Cloning into '...'` line and leave
   the wrapper out. Recommendation: skip `--print-dest`, keep the tool small,
   and close issue #1 as "won't fix" with the explanation that a child process
   cannot change its parent's directory.
6. **Known issues.** Remove the empty bullet. List: Windows is built but
   untested; macOS binaries are unsigned (issue #2).
7. Fix the typo "You can down the latest binaries".

## Code

- Remove the commented-out logger line (also in spec 03).
- Add a `doc.go`-style package comment at the top of `main.go` describing the
  tool in two lines. Not required, but `go vet` and `golint`-style tools expect
  one.

## Repository

- Delete remote branches `fix-subdomain-issue` (merged) and `gradle-build`
  (2015, superseded by the Makefile).
- Close issue #1 as described above. Leave issue #2 open; it is out of scope.
- Add a `.gitignore` entry for `git-grab` and `*.exe` (spec 01).
- Add `specs/` to the repo. It is the plan of record until the work is done,
  then it can be deleted or kept as history.
- Tag the result as `0.5`. The release job from spec 01 attaches binaries.
  Release notes should mention: Go 1.26, option passthrough, `GIT_GRAB_ROOT`,
  Windows support, and the `ParseRepo` fixes with issue references.

## CLAUDE.md

Add the `CLAUDE.md` drafted during the review, updated for the new behavior:
`ParseRepo` returns an error, `make build` is the build command, Go 1.26 is
required, and the Makefile no longer needs bash-on-macOS caveats once
`SHELL := /bin/bash` is set.

## Acceptance

- README install instructions work on a clean machine via `go install`.
- `gh issue list` shows only issue #2 open.
- `git branch -r` shows only `origin/master`.
