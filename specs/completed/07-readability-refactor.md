# 07: Readability refactor of main.go and main_test.go

## Problem

A design and readability review (2026-11-15, architect and Go reviewers)
found the code sound but with a handful of spots a reader has to re-read:
one error message spelled three times, the `git-grab:` prefix spelled four
times, a two-pass `splitArgs` for a one-pass job, a nested `append`, and
`ParseRepo` deriving the domain 35 lines after it split off the host. The
scp branch of `splitHostPath` re-parses what `scpSyntax` already decided.
Several tested behaviors are missing from CLAUDE.md's rules list.

None of this is a bug. The goal is that each rule in CLAUDE.md "Behavior
rules encoded in tests" maps to one obviously named piece of code.

## Decisions

- **No behavior change.** Every row of `TestParseRepo`, `TestSplitArgs`,
  and `TestRun` keeps its expected value. Rows may be regrouped and tests
  may be added; none may be weakened. Error text printed to users is
  unchanged except where a phase below says otherwise.
- **Still stdlib only, still two source files.** `slices` and
  `strings.FieldsFunc` are stdlib and already available on Go 1.26.
- **Three stacked PRs**, smallest and safest first, so each can be
  reviewed on its own and the structural change is isolated:
  - A: local cleanups and documentation.
  - B: structure of `ParseRepo` and `splitHostPath`.
  - C: polish, test layout, and closing this spec.
  Each PR branches from the previous one and targets it on GitHub; when
  the base merges, GitHub retargets the next PR to `master`.
- **CLAUDE.md is part of the deliverable.** Its helper list and rules list
  must describe the code after each phase.

## Phase A: local cleanups (PR `tidy-run-and-args`)

1. `ParseRepo`: the `expected <owner>/<repo> in %q` error is built at three
   sites. Add `noOwnerRepoError(repo string) error` next to `noHostError`
   and use it at all three. Both helpers get a doc comment.
2. `run`: add a local `fail(err error) int` closure that prints
   `git-grab: <err>` to stderr and returns 1, and use it for the four error
   exits. The "already exists" message becomes
   `fail(fmt.Errorf("%s already exists", dest))`, same text as today.
   Extract the exec error mapping into `gitExitCode(err error) (code int,
   ok bool)`: ok is false unless err is an `ExitError`; code >= 0 passes
   through and -1 (signal) becomes 1. `run` prints `git-grab:` only when
   ok is false, as today. (Returning a bare int would make `run` repeat
   the `errors.As` check to decide whether to print; found in review.)
3. `splitArgs`: one reverse scan that returns at the first argument not
   starting with `-`. Drop `passthrough = []string{}`; `slices.Equal`
   already treats nil and empty as equal. Do not mutate the caller's
   slice (`slices.Concat(args[:i], args[i+1:])` allocates a new one).
4. `run`: build the git argument list with
   `slices.Concat([]string{"clone"}, passthrough, []string{repo, dest})`.
5. `TestDestination`: convert to a table with `root` and `want` columns
   and `t.Run` per row, so one failure does not hide the others. The
   relative-root row uses `t.Chdir(t.TempDir())` and asserts the joined
   absolute path, not just `IsAbs`.
6. CLAUDE.md "Behavior rules encoded in tests" gains the behaviors that
   are tested but undocumented:
   - Scheme-less input is accepted only when the host contains a dot, so
     `localhost/owner/repo` is rejected while `ssh://git@localhost/owner/repo`
     works. Without this rule `jmcgarr/git-grab` would parse as host
     `jmcgarr`.
   - The domain is lowercased too, not only owner and groups.
   - A dangling symlink at the destination counts as "already exists".
   - Query string and fragment are dropped from scheme URLs.

## Phase B: structure (PR `parse-repo-pipeline`)

7. `ParseRepo` reads as a pipeline in the order the rules are stated:
   split host and path, derive the domain (fail with `noHostError` if
   empty), validate path segments, rewrite the owner, join. Extract:
   - `pathSegments(path, repo string) ([]string, error)`: trim slashes and
     `.git`, split on `/` dropping empty segments, reject `.` and `..`,
     drop a leading `scm`, require at least two segments.
   - `ownerDir(owner, repo string) (string, error)`: `~user` becomes
     `users/user`; bare `~` is an error.
   `strings.FieldsFunc` replaces the hand-rolled "drop empty strings"
   loops here and in `domainOf`, whose switch collapses to
   `labels[max(len(labels)-2, 0)]` after the empty check.
8. scp handling becomes one function. `scpSplit(repo string) (host, path
   string, ok bool)` both decides that the input is scp syntax (the
   current `scpSyntax` rule, including the Windows drive-letter exception)
   and splits it, including the bracketed IPv6 form. `scpSyntax` and
   `isASCIILetter` fold into it. A shared `stripUserInfo(s string) string`
   (strip through the last `@`) serves both the scp and bare-host
   branches, so user stripping is written once. `splitHostPath` becomes
   three short cases.
9. CLAUDE.md "Layout" lists the new helper names in place of
   `splitHostPath`, `scpSyntax`, `domainOf`.

### Phase B implementation notes (2026-11-20)

- `isASCIILetter` stays as a helper. Folding it into `scpSplit` produced a
  bit trick (`c|0x20`) that reads worse than the two range checks.
- Checking the domain before the path changes which error some inputs
  get, never whether they get one. No test row changed.
- Two inputs no test covered now behave differently, both in the
  direction CLAUDE.md's rules already describe. `scpSplit` returns
  ok=false for an empty host, so the bare-host branch gained a
  `:`-in-host rejection to keep `:x.y/owner/repo` an error; as a side
  effect a drive-letter form with no slash after the colon is now
  rejected: `ParseRepo("C:x.y/owner/repo")` used to return
  `c:x/owner/repo` and is now an error. User info is stripped through
  the last `@` in both branches, so `a@b@host.com:owner/repo` now gives
  `host/owner/repo` rather than `b@host/owner/repo`. Rows pin both, plus
  the `@`-in-path, empty-host, and leading-colon cases. Review ran a
  differential sweep of about 5.4M short inputs against the old code and
  found no other class of change.

## Phase C: polish (PR `readability-polish`)

10. `TestParseRepo` rows are regrouped to mirror the CLAUDE.md rules list
    (domain derivation; case; `.git`, slashes, dots in names; Stash
    `~user` and `scm/`; accepted URL forms; rejected inputs), with a
    comment per group. The "must not change" sentence stays above the
    table. No row's expected value changes.
11. `destination` wraps its two errors (`resolving home directory: %w`,
    `resolving GIT_GRAB_ROOT: %w`) and uses `root, err = ...` instead of
    a redeclaration. This is the one permitted message change; `TestRun`
    uses "contains" checks and is unaffected.
12. Tests: rename the `url` variable and field in `TestSplitArgs` to
    `repo`; extract the duplicated "empty means must be empty, otherwise
    must contain" check in `TestRun` into `checkOutput(t, name, got,
    want)` with `t.Helper()`; comment the dual meaning on the struct
    fields; `defer`-free `ln.Close()` gets its error checked or moves
    into a `freePort(t)` helper.
13. Doc comments on `main`, `noHostError`, `noOwnerRepoError`, and every
    helper added in phase B. `ParseRepo`'s comment states that it never
    touches the filesystem and names the two errors it can return.
14. Move this spec to `specs/completed/` and mark it done in the index.

## Tests

No new behavior, so the existing tables are the oracle. Each PR must leave
`go test -cover ./...` at or above the current 91.3%, with `go vet ./...`
and `gofmt -l .` clean. Phase A adds the table rows in `TestDestination`;
phase C adds no rows and changes no expected values.

## Acceptance

- The three PRs merge in order with no behavior change visible to users
  other than the wrapped `destination` errors.
- `main.go` has no duplicated error string, no nested `append`, and
  `ParseRepo` derives the domain before it inspects the path.
- CLAUDE.md's helper list and rules list match the code.
- `specs/` top level holds only `README.md` after phase C merges.

## Out of scope

- New flags, config, or dependencies.
- A struct type for the parse result; both reviewers judged the string
  returns right for this size.
- Splitting `main.go` into packages or files.
