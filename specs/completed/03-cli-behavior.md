# 03: CLI behavior, portability, exit codes

## Problem

- README says `git grab <options> <repo-url>`, but `main` uses `os.Args[1]` as
  the URL and forwards nothing else. `git grab --depth 1 <url>` panics.
- Destination is built from `$HOME`, which is unset on Windows, with `/`
  separators. Windows binaries are shipped but cannot work.
- On clone failure the program prints a timestamped `exit status 128` on top
  of git's own message and exits 1 regardless of git's code.
- No `-h`, `--help`, or `--version`.
- A commented-out logger declaration remains in `main.go`.

## Changes

### Argument handling

Do not use the `flag` package; it would reject git's own options. Walk the
arguments by hand:

- `-h`, `--help`: print usage to stdout, exit 0.
- `--version`: print `git-grab <version>` and exit 0. `version` is a package
  variable defaulting to `"dev"`, set at build time via `-X main.version`.
- The **last argument that does not start with `-`** is the repository URL.
  Every other argument is forwarded to `git clone` unchanged, in order, before
  the URL.
- No URL found: print usage to stderr, exit 2.

This accepts `git grab --depth 1 --branch main <url>` and `git grab <url>`.
It does not accept `git clone`'s optional `<directory>` argument; that is the
one thing this tool exists to decide.

### Destination

```go
root := os.Getenv("GIT_GRAB_ROOT")
if root == "" {
	home, err := os.UserHomeDir()   // works on Windows too
	if err != nil { fail }
	root = filepath.Join(home, "Projects")
}
dest := filepath.Join(root, filepath.FromSlash(rel))
```

`GIT_GRAB_ROOT` is the single configuration knob. Document it in the README.

Before cloning, if `dest` already exists, print
`git-grab: %s already exists` to stderr and exit 1. git would fail anyway, but
the message is clearer and avoids a pointless network round trip.

### Running git

```go
cmd := exec.Command("git", append(append([]string{"clone"}, passthrough...), url, dest)...)
cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
err := cmd.Run()
```

- `Stdin` is attached so credential and SSH host-key prompts work.
- On `*exec.ExitError`, exit with the same code and print nothing. git already
  explained the failure.
- On any other error (git not found), print `git-grab: <err>` to stderr, exit 1.

### Usage text

```
Usage: git grab [git-clone options] <repository>

Clones <repository> into $GIT_GRAB_ROOT (default ~/Projects) under
<domain>/<owner>/<repo>. All other options are passed to git clone.

  -h, --help     show this help
      --version  print version
```

### Cleanup

- Remove the `//var log = ...` line and the `log` import.
- Replace `var x string = ""` declarations with `:=` or zero values.
- Keep everything in `main.go`. Pull the argument walk into a small function
  `splitArgs(args []string) (passthrough []string, url string)` so it can be
  tested without running git.

## Implementation notes

- `main()` is reduced to `os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))`.
  `run` holds the logic above and returns the exit code, so help, version,
  usage, parse errors, the already-exists check, and git's exit-code
  passthrough are all covered by `TestRun` without touching the network. The
  passthrough case points git at `https://127.0.0.1:9/...`, which is refused
  immediately and makes git exit 128.
- The destination logic lives in `destination(rel string) (string, error)`
  and is covered by `TestDestination` using `t.Setenv`.
- `-h`, `--help`, and `--version` are recognized anywhere in the argument
  list and take precedence over cloning.
- `git grab --help` never reaches the binary: git rewrites `git <cmd> --help`
  to `git help <cmd>` before dispatching external commands, and prints
  `No manual entry for git-grab`. `git grab -h` and `git-grab --help` work.
  The README tells users to use `-h`.
- The "last non-dash argument" rule means options placed after the URL
  mis-select the URL (`git grab <url> --branch main` treats `main` as the
  URL). `git clone` accepts that order, this tool does not. The README says
  to put options first. Changing the rule would need a new spec.
- Hardening from review: the already-exists check uses `os.Lstat` so a
  dangling symlink counts as occupied; `GIT_GRAB_ROOT` is made absolute with
  `filepath.Abs` so a relative value gives a clear message; a git process
  killed by a signal (`ExitCode() == -1`) exits 1 rather than 255.
- The exit-code passthrough test isolates git from the developer's proxy
  settings and global config (`no_proxy=*`, `GIT_CONFIG_GLOBAL` pointing at
  an empty temp file, `GIT_CONFIG_NOSYSTEM=1`, `GIT_TERMINAL_PROMPT=0`), and
  uses a freshly released loopback port rather than a fixed one. Without
  this, a configured proxy made the test hang instead of failing fast.

## Tests

Add `TestSplitArgs` with cases:

| Args | passthrough | url |
|------|-------------|-----|
| `[url]` | `[]` | `url` |
| `[--depth 1 url]` | `[--depth 1]` | `url` |
| `[--branch main --depth 1 url]` | `[--branch main --depth 1]` | `url` |
| `[]` | `[]` | `` |
| `[--depth 1]` | `[--depth]` | `1` |

The last row is deliberately odd: `1` is treated as the URL and `ParseRepo`
then rejects it with a clear error. That is acceptable and documents the rule.

## Acceptance

- `git grab --depth 1 https://github.com/jmcgarr/git-grab.git` performs a
  shallow clone into `~/Projects/github/jmcgarr/git-grab`.
- `GIT_GRAB_ROOT=/tmp/x git grab <url>` clones under `/tmp/x`.
- Running against a bad URL exits with git's exit code and no extra output.
- `git grab` with no arguments prints usage and exits 2.
- `go test -cover ./...` reports coverage above 80 percent.
