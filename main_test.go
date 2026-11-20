package main

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseRepo(t *testing.T) {
	tests := []struct {
		in, want string
		wantErr  bool
	}{
		// These rows define the directory scheme and must not change.
		// Domain: second-level label, whole IP, single-label host,
		// trailing-dot host.
		{"https://github.com/sstephenson/bats.git", "github/sstephenson/bats", false},
		{"ssh://git@stash.funville.com:7999/hackday/template.git", "funville/hackday/template", false},
		{"https://192.168.1.10/owner/repo.git", "192.168.1.10/owner/repo", false},
		{"ssh://git@[::1]/owner/repo.git", "::1/owner/repo", false},
		{"ssh://git@localhost/owner/repo.git", "localhost/owner/repo", false},
		{"https://github.com./jmcgarr/repo.git", "github/jmcgarr/repo", false},

		// Case: owner and groups are lowercased, the repo name keeps its case.
		{"https://github.com/JMCGARR/bats", "github/jmcgarr/bats", false},
		{"https://bitbucket.org/Team/Repo.git", "bitbucket/team/Repo", false},
		{"https://gitlab.com/group/sub/project.git", "gitlab/group/sub/project", false},

		// Trailing .git and slashes are stripped; dots elsewhere in the
		// name survive.
		{"git@github.com:sstephenson/bats.git", "github/sstephenson/bats", false},
		{"https://github.com/jmcgarr/git-grab/", "github/jmcgarr/git-grab", false},
		{"git@github.com:nebula-plugins/nebula-plugins.github.io.git", "github/nebula-plugins/nebula-plugins.github.io", false},
		{"https://github.com/jmcgarr/Mixed.Case.Repo", "github/jmcgarr/Mixed.Case.Repo", false},
		{"git@github.com:jmcgarr/repo-with.dot.git", "github/jmcgarr/repo-with.dot", false},

		// Stash: ~user becomes users/user, and a leading scm/ is ignored.
		{"ssh://git@stash.funville.com:1234/~jmcgarr/homedir.git", "funville/users/jmcgarr/homedir", false},
		{"http://jmcgarr@stash.funville.com/scm/~jmcgarr/homedir.git", "funville/users/jmcgarr/homedir", false},
		{"http://jmcgarr@stash.funville.com/scm/hackday/template.git", "funville/hackday/template", false},

		// Accepted URL forms: scp (IPv6, @ in path), user and user:pass,
		// ports, scheme-less, query and fragment.
		{"git@[::1]:owner/repo.git", "::1/owner/repo", false},
		{"git@host.com:owner/re@po", "host/owner/re@po", false},
		{"a@b@host.com:owner/repo", "host/owner/repo", false},
		{"https://user:pass@github.com/jmcgarr/repo.git", "github/jmcgarr/repo", false},
		{"ssh://git@github.com:22/jmcgarr/git-grab.git", "github/jmcgarr/git-grab", false},
		{"github.com/jmcgarr/git-grab", "github/jmcgarr/git-grab", false},
		{"user@github.com/jmcgarr/repo", "github/jmcgarr/repo", false},
		{"https://github.com/jmcgarr/repo?ref=main#readme", "github/jmcgarr/repo", false},

		// Rejected inputs: local paths, drive letters, traversal, bare
		// names, flags, file://, missing owner/repo, empty input, empty scp
		// host, leading colon, bare ~.
		{"../owner/repo", "", true},
		{"./owner/repo", "", true},
		{"/Users/me/repo", "", true},
		{"C:/repos/foo/bar", "", true},
		{"C:\\repos\\foo", "", true},
		{"https://github.com/../../../../tmp/evil", "", true},
		{"https://github.com/owner/../repo", "", true},
		{"git-grab", "", true},
		{"jmcgarr/git-grab", "", true},
		{"--depth", "", true},
		{"file:///Users/me/repo.git", "", true},
		{"https://github.com/jmcgarr", "", true},
		{"", "", true},
		{"git@:owner/repo", "", true},
		{":x.y/owner/repo", "", true},
		{"C:x.y/owner/repo", "", true},
		{"https://stash.funville.com/~/repo.git", "", true},
	}
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

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		args, passthrough []string
		repo              string
	}{
		{[]string{"url"}, []string{}, "url"},
		{[]string{"--depth", "1", "url"}, []string{"--depth", "1"}, "url"},
		{[]string{"--branch", "main", "--depth", "1", "url"}, []string{"--branch", "main", "--depth", "1"}, "url"},
		{[]string{}, []string{}, ""},
		// Deliberately odd: "1" becomes the URL and ParseRepo rejects it.
		{[]string{"--depth", "1"}, []string{"--depth"}, "1"},
	}
	for _, tc := range tests {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			passthrough, repo := splitArgs(tc.args)
			if !slices.Equal(passthrough, tc.passthrough) {
				t.Errorf("splitArgs(%q) passthrough = %q, want %q", tc.args, passthrough, tc.passthrough)
			}
			if repo != tc.repo {
				t.Errorf("splitArgs(%q) repo = %q, want %q", tc.args, repo, tc.repo)
			}
		})
	}
}

func TestDestination(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory:", err)
	}
	absRoot := t.TempDir()
	// A relative GIT_GRAB_ROOT resolves against the working directory.
	cwd := t.TempDir()
	t.Chdir(cwd)
	tail := filepath.Join("github", "jmcgarr", "git-grab")

	tests := []struct {
		name string
		root string // GIT_GRAB_ROOT; "" means unset
		want string
	}{
		{"default", "", filepath.Join(home, "Projects", tail)},
		{"absolute root", absRoot, filepath.Join(absRoot, tail)},
		{"relative root", "relroot", filepath.Join(cwd, "relroot", tail)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GIT_GRAB_ROOT", tc.root)
			got, err := destination("github/jmcgarr/git-grab")
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("destination() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRun covers every exit path that does not need a network. The clone
// case points git at a closed local port so it fails fast with git's own
// exit code, which run must pass through unchanged.
func TestRun(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_GRAB_ROOT", root)

	existing := filepath.Join(root, "github", "jmcgarr", "exists")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	// A dangling symlink is occupied too: git would fail to create the dir.
	if err := os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(root, "github", "jmcgarr", "dangling")); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
		code int
		// An empty want means the stream must be empty; otherwise it
		// must contain want.
		wantStdout string
		wantStderr string
	}{
		{"help", []string{"--help"}, 0, "Usage: git grab", ""},
		{"short help", []string{"-h"}, 0, "Usage: git grab", ""},
		{"help wins over url", []string{"https://github.com/o/r", "-h"}, 0, "Usage: git grab", ""},
		{"version", []string{"--version"}, 0, "git-grab dev", ""},
		{"no args", []string{}, 2, "", "Usage: git grab"},
		{"only options", []string{"--depth"}, 2, "", "Usage: git grab"},
		{"bad url", []string{"--depth", "1"}, 1, "", "cannot determine host"},
		{"no repo segment", []string{"https://github.com/jmcgarr"}, 1, "", "expected <owner>/<repo>"},
		{"already exists", []string{"https://github.com/jmcgarr/exists.git"}, 1, "", "already exists"},
		{"dangling symlink exists", []string{"https://github.com/jmcgarr/dangling.git"}, 1, "", "already exists"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tc.args, &stdout, &stderr)
			if code != tc.code {
				t.Errorf("run(%q) = %d, want %d", tc.args, code, tc.code)
			}
			checkOutput(t, "stdout", stdout.String(), tc.wantStdout)
			checkOutput(t, "stderr", stderr.String(), tc.wantStderr)
		})
	}

	t.Run("git exit code passes through", func(t *testing.T) {
		if _, err := exec.LookPath("git"); err != nil {
			t.Skip("git not installed")
		}
		// Isolate git from the developer's environment: a proxy from the
		// shell or global git config would make the connection hang instead
		// of being refused, and a prompt would block the test.
		t.Setenv("no_proxy", "*")
		t.Setenv("NO_PROXY", "*")
		t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
		t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
		t.Setenv("GIT_TERMINAL_PROMPT", "0")

		addr := freePort(t)

		var stdout, stderr bytes.Buffer
		code := run([]string{"--depth", "1", "https://" + addr + "/owner/repo.git"}, &stdout, &stderr)
		if code != 128 {
			t.Errorf("run() = %d, want git's exit code 128; stderr: %s", code, stderr.String())
		}
		if strings.Contains(stderr.String(), "git-grab:") {
			t.Errorf("run() added its own message on top of git's: %s", stderr.String())
		}
		if _, err := os.Stat(filepath.Join(root, "127.0.0.1", "owner", "repo")); err == nil {
			t.Error("failed clone left a directory behind")
		}
	})
}

// checkOutput fails t unless got is empty when want is empty, or contains
// want otherwise. stream names the stream in the failure message.
func checkOutput(t *testing.T, stream string, got, want string) {
	t.Helper()
	if want == "" {
		if got != "" {
			t.Errorf("%s = %q, want empty", stream, got)
		}
		return
	}
	if !strings.Contains(got, want) {
		t.Errorf("%s = %q, want it to contain %q", stream, got, want)
	}
}

// freePort returns a loopback address whose port was free a moment ago:
// it is grabbed and released so nothing listens there.
func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}
