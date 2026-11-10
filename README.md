# git-grab

`git grab` is a git subcommand that clones a repository into a predictable
place: `~/Projects/<domain>/<owner>/<repo>`. Repos end up organized by where
they came from, so finding them later is easy. It behaves exactly like
`git clone`; only the destination is computed for you.

```
$ git grab https://github.com/jmcgarr/git-grab.git
Cloning into '/Users/you/Projects/github/jmcgarr/git-grab'...
```

```
Projects
    └── github
          └── jmcgarr
                └── git-grab
```

The domain is the second-level label of the host (`github.com` -> `github`,
`stash.example.com` -> `example`). The owner is lowercased; the repository name
keeps its case. A trailing `.git` is dropped. GitLab subgroups are kept
(`gitlab.com/group/sub/project` -> `gitlab/group/sub/project`), and
Bitbucket Server user repos (`~user`) become `users/user`.

## Install

With Homebrew on macOS or Linux:

```
brew install jmcgarr/tap/git-grab
```

Homebrew asks you to trust the tap on first install; `brew trust jmcgarr/tap`
answers once for everything in it. Homebrew downloads carry no quarantine
flag, so there is nothing else to approve.

With Go 1.26 or newer:

```
go install github.com/jmcgarr/git-grab@latest
```

Or download a binary for your platform from the
[releases page](https://github.com/jmcgarr/git-grab/releases), rename it to
`git-grab`, make it executable, and put it on your `PATH`. On macOS a direct
download is unsigned, so clear the quarantine flag once:

```
xattr -d com.apple.quarantine git-grab
```

Git finds any executable named `git-<name>` on `PATH` and runs it as
`git <name>`, so no further setup is needed.

## Usage

```
Usage: git grab [git-clone options] <repository>

Clones <repository> into $GIT_GRAB_ROOT (default ~/Projects) under
<domain>/<owner>/<repo>. All other options are passed to git clone.

  -h, --help     show this help
      --version  print version
```

Every option other than `-h` and `--version` is forwarded to
[`git clone`](https://git-scm.com/docs/git-clone) unchanged, so a shallow
clone is:

```
git grab --depth 1 https://github.com/jmcgarr/git-grab.git
```

Put options before the URL. The last argument that does not start with `-` is
taken as the repository, so `git grab <url> --branch main` would treat `main`
as the URL.

Use `git grab -h` for help. `git grab --help` never reaches git-grab: git
rewrites `git <cmd> --help` to `git help <cmd>` and looks for a man page.
`--help` works when the binary is run directly as `git-grab --help`.

Accepted URL forms include `https://`, `ssh://`, scp-style
`git@host:owner/repo.git`, URLs with user info or ports, and scheme-less
`github.com/owner/repo`. Local paths and `file://` URLs are rejected.

If the destination already exists, git-grab says so and exits 1 without
contacting the remote. When `git clone` fails, git-grab exits with git's exit
code and adds no output of its own.

## Configuration

`GIT_GRAB_ROOT` replaces `~/Projects` as the root directory. It is the only
setting.

```
export GIT_GRAB_ROOT=$HOME/src
```

## Changing into the new directory

A child process cannot change its parent shell's directory, so `git grab`
cannot `cd` for you. The destination appears in git's own
`Cloning into '...'` line. If you want a one-step clone-and-enter, add a
shell function that captures that line:

```sh
grab() {
  local out
  out=$(git grab "$@" 2>&1 | tee /dev/stderr) || return $?
  cd "$(printf '%s\n' "$out" | sed -n "s/^Cloning into '\(.*\)'\.\.\.$/\1/p")"
}
```

## Known issues

- Windows binaries are built but untested.
- macOS binaries downloaded directly from the releases page are unsigned
  and will be quarantined until you run the `xattr` command above
  ([#2](https://github.com/jmcgarr/git-grab/issues/2)). Homebrew installs
  are not affected.

## Development

```
make build   # builds ./git-grab
make test    # go test -cover ./...
make vet     # go vet ./...
make fmt     # fails if gofmt would change anything
make all     # cross-compiles every platform into dist/
```

Releases are tags on `master`. Pushing a tag builds every platform in CI,
attaches the binaries and a `SHA256SUMS` file to a GitHub release, and asks
[jmcgarr/homebrew-tap](https://github.com/jmcgarr/homebrew-tap) to update
its formula.
