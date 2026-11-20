// Command git-grab clones a repository into $GIT_GRAB_ROOT (default
// ~/Projects) under <domain>/<owner>/<repo>, so checkouts are organized by
// origin. It is a thin wrapper around git clone that only computes the
// destination.
package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

const usage = `Usage: git grab [git-clone options] <repository>

Clones <repository> into $GIT_GRAB_ROOT (default ~/Projects) under
<domain>/<owner>/<repo>. All other options are passed to git clone.

  -h, --help     show this help
      --version  print version
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without the process exit, so every path short of actually
// invoking git can be tested. It returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	for _, a := range args {
		switch a {
		case "-h", "--help":
			fmt.Fprint(stdout, usage)
			return 0
		case "--version":
			fmt.Fprintln(stdout, "git-grab", version)
			return 0
		}
	}

	passthrough, repo := splitArgs(args)
	if repo == "" {
		fmt.Fprint(stderr, usage)
		return 2
	}

	// fail reports one of git-grab's own errors and returns its exit code.
	fail := func(err error) int {
		fmt.Fprintln(stderr, "git-grab:", err)
		return 1
	}

	rel, err := ParseRepo(repo)
	if err != nil {
		return fail(err)
	}
	dest, err := destination(rel)
	if err != nil {
		return fail(err)
	}
	// Lstat so a dangling symlink counts as occupied too.
	if _, err := os.Lstat(dest); err == nil {
		return fail(fmt.Errorf("%s already exists", dest))
	}

	cmd := exec.Command("git", slices.Concat([]string{"clone"}, passthrough, []string{repo, dest})...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
	err = cmd.Run()
	if err == nil {
		return 0
	}
	if code, ok := gitExitCode(err); ok {
		// git already explained the failure; print nothing extra.
		return code
	}
	return fail(err)
}

// gitExitCode reports whether err means git ran and failed, and if so the
// exit code to pass through. ExitCode is -1 when git was killed by a
// signal; that becomes 1. Any other error (git not found, for instance)
// is not git's and returns ok=false.
func gitExitCode(err error) (code int, ok bool) {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return 0, false
	}
	if code := exitErr.ExitCode(); code >= 0 {
		return code, true
	}
	return 1, true
}

// splitArgs separates git-clone options from the repository URL. The last
// argument that does not start with "-" is the URL; everything else is
// forwarded to git clone unchanged and in order. repo is "" when no
// candidate exists.
func splitArgs(args []string) (passthrough []string, repo string) {
	for i := len(args) - 1; i >= 0; i-- {
		if !strings.HasPrefix(args[i], "-") {
			return slices.Concat(args[:i], args[i+1:]), args[i]
		}
	}
	return slices.Clone(args), ""
}

// destination returns the absolute clone directory for a ParseRepo result:
// $GIT_GRAB_ROOT, or ~/Projects when unset, joined with rel using the OS
// path separator. A relative GIT_GRAB_ROOT is resolved against the current
// directory so messages show where the clone actually went.
func destination(rel string) (string, error) {
	root := os.Getenv("GIT_GRAB_ROOT")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, "Projects")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}

// ParseRepo returns the path, relative to the projects root, where repo
// should be cloned: "<domain>/<owner>[/<group>...]/<name>".
func ParseRepo(repo string) (string, error) {
	host, path, err := splitHostPath(repo)
	if err != nil {
		return "", err
	}

	// Path segments: strip surrounding slashes and a trailing .git, then
	// split. Drop empty segments and Bitbucket Server's leading "scm".
	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	var segments []string
	for _, s := range strings.Split(path, "/") {
		if s == "." || s == ".." {
			// Would escape the projects root once joined to a directory.
			return "", noOwnerRepoError(repo)
		}
		if s != "" {
			segments = append(segments, s)
		}
	}
	if len(segments) > 0 && segments[0] == "scm" {
		segments = segments[1:]
	}
	if len(segments) < 2 {
		return "", noOwnerRepoError(repo)
	}

	// Owner: Stash user repos "~user" become "users/user".
	owner := segments[0]
	if strings.HasPrefix(owner, "~") {
		if len(owner) == 1 {
			return "", noOwnerRepoError(repo)
		}
		owner = "users/" + owner[1:]
	}

	// A host made only of dots ("." or "..") has no usable domain.
	domain := domainOf(host)
	if domain == "" {
		return "", noHostError(repo)
	}

	parts := []string{domain, strings.ToLower(owner)}
	for _, group := range segments[1 : len(segments)-1] {
		parts = append(parts, strings.ToLower(group))
	}
	// Name keeps its case so existing checkouts stay where they are.
	parts = append(parts, segments[len(segments)-1])
	return strings.Join(parts, "/"), nil
}

// splitHostPath separates a remote URL into its host and path. It accepts
// scheme URLs (ssh://, https://), scp syntax (git@host:path), and bare
// host/owner/name. Anything else, including local paths, is an error.
func splitHostPath(repo string) (host, path string, err error) {
	switch {
	case strings.Contains(repo, "://"):
		u, perr := url.Parse(repo)
		if perr != nil || u.Hostname() == "" {
			return "", "", noHostError(repo)
		}
		return u.Hostname(), u.Path, nil

	case scpSyntax(repo):
		hostPart := repo
		if at := strings.Index(hostPart, "@"); at >= 0 && at < strings.Index(hostPart, ":") {
			hostPart = hostPart[at+1:]
		}
		var pathPart string
		if strings.HasPrefix(hostPart, "[") {
			// Bracketed IPv6: git@[::1]:owner/repo. Cut after the bracket.
			var found bool
			hostPart, pathPart, found = strings.Cut(hostPart[1:], "]:")
			if !found {
				return "", "", noHostError(repo)
			}
		} else {
			hostPart, pathPart, _ = strings.Cut(hostPart, ":")
		}
		if hostPart == "" {
			return "", "", noHostError(repo)
		}
		return hostPart, pathPart, nil

	default:
		first, rest, found := strings.Cut(repo, "/")
		if at := strings.LastIndex(first, "@"); at >= 0 {
			first = first[at+1:]
		}
		if !found || !strings.Contains(first, ".") {
			return "", "", noHostError(repo)
		}
		return first, rest, nil
	}
}

// noHostError is returned when repo has no usable host, so it cannot be a
// remote URL.
func noHostError(repo string) error {
	return fmt.Errorf("cannot determine host in %q; git-grab needs a remote URL", repo)
}

// noOwnerRepoError is returned when repo has a host but its path lacks an
// <owner>/<repo> pair, or contains segments that could escape the root.
func noOwnerRepoError(repo string) error {
	return fmt.Errorf("expected <owner>/<repo> in %q", repo)
}

// scpSyntax reports whether repo looks like [user@]host:path, i.e. it has a
// ":" before the first "/". A single letter before the colon is a Windows
// drive (C:/repos/foo), not a host, which is the rule git itself uses.
func scpSyntax(repo string) bool {
	colon := strings.Index(repo, ":")
	if colon < 0 {
		return false
	}
	if colon == 1 && isASCIILetter(repo[0]) {
		return false
	}
	slash := strings.Index(repo, "/")
	return slash < 0 || colon < slash
}

func isASCIILetter(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

// domainOf reduces a host to the directory name used under the projects
// root: the second-level label ("stash.funville.com" -> "funville"), the
// whole address for IPs, or the only label for single-label hosts.
func domainOf(host string) string {
	host = strings.ToLower(host)
	if net.ParseIP(host) != nil {
		return host
	}
	var labels []string
	for _, l := range strings.Split(host, ".") {
		if l != "" {
			labels = append(labels, l)
		}
	}
	switch len(labels) {
	case 0:
		return ""
	case 1:
		return labels[0]
	default:
		return labels[len(labels)-2]
	}
}
