# 02: Rewrite `ParseRepo`

## Problem

`ParseRepo` splits the whole "host plus path" string on `.` and `/` and indexes
from the end. That works for the eight URLs in the test file and fails on
anything else. Observed with the v0.4 code:

| Input | Today | Problem |
|-------|-------|---------|
| `git-grab` | panic | no `/` |
| `--depth` | panic | any flag before the URL |
| `/Users/me/repo`, `file:///...` | panic | local paths |
| `https://github.com/jmcgarr/Mixed.Case.Repo` | `Case/jmcgarr/Mixed.Case.Repo` | dot in repo name picked as domain |
| `git@github.com:jmcgarr/repo-with.dot.git` | `com:jmcgarr/repo-with/...` | same |
| `https://github.com/jmcgarr/git-grab/` | `github/git-grab/` | trailing slash |
| `https://gitlab.com/group/sub/project.git` | `gitlab/sub/project` | subgroup dropped |
| `https://192.168.1.10/owner/repo.git` | `1/owner/repo` | IP host |

The `.github.io` test passes by coincidence: the second-to-last dot token of
`github.com/nebula-plugins/nebula-plugins.github.io` happens to be `github`.

## New signature

```go
// ParseRepo returns the path, relative to the projects root, where repo
// should be cloned: "<domain>/<owner>[/<group>...]/<name>".
func ParseRepo(repo string) (string, error)
```

## Algorithm

Parse into `host` and `path`, then build the result from those two values only.

1. **Split host from path.**
   - If the string contains `://`, parse with `net/url`. Use `u.Hostname()`
     (strips port and userinfo) and `u.Path`.
   - Else if it matches scp syntax `[user@]host:path` (a `:` before the first
     `/`), split on the first `:`. Strip anything up to and including `@` from
     the host part.
   - Else if it looks like `host/owner/name` (first segment contains a `.`),
     treat the first segment as host and the rest as path.
   - Otherwise return an error: `"cannot determine host in %q; git-grab needs a remote URL"`.
     This covers bare names, flags, local paths, and `file://` URLs.
2. **Domain.** Split `host` on `.`.
   - If `net.ParseIP(host) != nil`, domain is the full host.
   - Else if there are two or more labels, domain is the second-to-last label
     (`stash.funville.com` -> `funville`, `github.com` -> `github`).
   - Else domain is the only label (`localhost`).
   - Lowercase it.
3. **Path segments.** Trim leading and trailing `/`, trim a trailing `.git`,
   then split on `/`. Drop empty segments. Drop a leading `scm` segment
   (Bitbucket Server / Stash URL prefix, already in the tests). Require at
   least two segments, else error `"expected <owner>/<repo> in %q"`.
4. **Owner.** First segment. If it starts with `~`, replace with `users/`
   (Stash convention, kept from today). Lowercase.
5. **Groups.** Any segments between owner and the last one are kept verbatim
   and lowercased. This handles GitLab subgroups and Azure DevOps `_git`.
6. **Name.** Last segment, case preserved. This matches current behavior and
   avoids moving anyone's existing checkouts.
7. Join with `/` (not `filepath.Join`; the caller does OS-specific joining).

## Expected results

Every row becomes a case in a table-driven test. Rows marked `keep` are the
existing tests and must not change.

| Input | Want | Note |
|-------|------|------|
| `git@github.com:sstephenson/bats.git` | `github/sstephenson/bats` | keep |
| `https://github.com/sstephenson/bats.git` | `github/sstephenson/bats` | keep |
| `https://github.com/JMCGARR/bats` | `github/jmcgarr/bats` | keep |
| `git@github.com:nebula-plugins/nebula-plugins.github.io.git` | `github/nebula-plugins/nebula-plugins.github.io` | keep |
| `ssh://git@stash.funville.com:1234/~jmcgarr/homedir.git` | `funville/users/jmcgarr/homedir` | keep |
| `http://jmcgarr@stash.funville.com/scm/~jmcgarr/homedir.git` | `funville/users/jmcgarr/homedir` | keep |
| `ssh://git@stash.funville.com:7999/hackday/template.git` | `funville/hackday/template` | keep |
| `http://jmcgarr@stash.funville.com/scm/hackday/template.git` | `funville/hackday/template` | keep |
| `https://github.com/jmcgarr/git-grab/` | `github/jmcgarr/git-grab` | trailing slash |
| `https://github.com/jmcgarr/Mixed.Case.Repo` | `github/jmcgarr/Mixed.Case.Repo` | dots in name |
| `git@github.com:jmcgarr/repo-with.dot.git` | `github/jmcgarr/repo-with.dot` | dots in name |
| `https://gitlab.com/group/sub/project.git` | `gitlab/group/sub/project` | subgroups |
| `https://bitbucket.org/Team/Repo.git` | `bitbucket/team/Repo` | owner lowercased, name kept |
| `https://192.168.1.10/owner/repo.git` | `192.168.1.10/owner/repo` | IP host |
| `ssh://git@localhost/owner/repo.git` | `localhost/owner/repo` | single label |
| `github.com/jmcgarr/git-grab` | `github/jmcgarr/git-grab` | no scheme |
| `https://user:pass@github.com/jmcgarr/repo.git` | `github/jmcgarr/repo` | userinfo |
| `ssh://git@github.com:22/jmcgarr/git-grab.git` | `github/jmcgarr/git-grab` | port |
| `git-grab` | error | bare name |
| `jmcgarr/git-grab` | error | no host |
| `--depth` | error | flag |
| `/Users/me/repo` | error | local path |
| `file:///Users/me/repo.git` | error | local path |
| `` | error | empty |
| `https://github.com/jmcgarr` | error | no repo segment |

## Hardening added during implementation

Review of the first implementation found inputs outside the table that
produced wrong directories instead of errors. These rules were added, each
with a test row:

- `.` and `..` path segments are an error. Without this,
  `https://github.com/../../../tmp/evil` resolved to a path outside
  `~/Projects`.
- A host with no non-empty labels (`.`, `..`) is an error, so `../owner/repo`
  and `./owner/repo` are rejected as the local paths they are. Empty labels
  are otherwise dropped, so `github.com.` (trailing-dot FQDN) gives `github`.
- A single ASCII letter before the colon is a Windows drive, not an scp host.
  `C:/repos/foo/bar` is an error instead of cloning into `~/Projects/c/...`.
- Bracketed IPv6 in scp syntax (`git@[::1]:owner/repo`) is split on `]:`.
- A bare `~` owner is an error rather than producing `users//repo`.
- Scheme-less input strips `user@` from the host, matching the scp branch.

## Tests

Replace `TestScenarios` and `AssertEquals` with one table-driven test:

```go
func TestParseRepo(t *testing.T) {
	tests := []struct {
		in, want string
		wantErr  bool
	}{ /* rows above */ }
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseRepo(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseRepo(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("ParseRepo(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
```

## Acceptance

- All rows above pass.
- `go vet` clean. No `panic`, no index arithmetic on split results without a
  length check.
- `ParseRepo` has no side effects and does not read the environment.
