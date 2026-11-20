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
	domain := domainOf(host)
	if domain == "" {
		// A host made only of dots ("." or "..") has no usable domain.
		return "", noHostError(repo)
	}
	segments, err := pathSegments(path, repo)
	if err != nil {
		return "", err
	}
	owner, err := ownerDir(segments[0], repo)
	if err != nil {
		return "", err
	}

	parts := []string{domain, strings.ToLower(owner)}
	for _, group := range segments[1 : len(segments)-1] {
		parts = append(parts, strings.ToLower(group))
	}
	// Name keeps its case so existing checkouts stay where they are.
	parts = append(parts, segments[len(segments)-1])
	return strings.Join(parts, "/"), nil
}

// pathSegments splits a URL path into its segments: surrounding slashes
// and a trailing .git are trimmed, empty segments dropped, and Bitbucket
// Server's leading "scm" removed. At least owner and name must remain, and
// "." or ".." (which would escape the projects root) is an error.
func pathSegments(path, repo string) ([]string, error) {
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	segments := strings.FieldsFunc(path, func(r rune) bool { return r == '/' })
	if slices.Contains(segments, ".") || slices.Contains(segments, "..") {
		return nil, noOwnerRepoError(repo)
	}
	if len(segments) > 0 && segments[0] == "scm" {
		segments = segments[1:]
	}
	if len(segments) < 2 {
		return nil, noOwnerRepoError(repo)
	}
	return segments, nil
}

// ownerDir rewrites a Stash user owner "~user" to "users/user". A bare "~"
// has no user name and is an error.
func ownerDir(owner, repo string) (string, error) {
	if !strings.HasPrefix(owner, "~") {
		return owner, nil
	}
	if len(owner) == 1 {
		return "", noOwnerRepoError(repo)
	}
	return "users/" + owner[1:], nil
}

// splitHostPath separates a remote URL into its host and path. It accepts
// scheme URLs (ssh://, https://), scp syntax (git@host:path), and bare
// host/owner/name. Anything else, including local paths, is an error.
func splitHostPath(repo string) (host, path string, err error) {
	if strings.Contains(repo, "://") {
		u, perr := url.Parse(repo)
		if perr != nil || u.Hostname() == "" {
			return "", "", noHostError(repo)
		}
		return u.Hostname(), u.Path, nil
	}
	if host, path, ok := scpSplit(repo); ok {
		return host, path, nil
	}
	first, rest, found := strings.Cut(repo, "/")
	first = stripUserInfo(first)
	// A ":" here means scpSplit rejected it (empty host, drive letter).
	if !found || !strings.Contains(first, ".") || strings.Contains(first, ":") {
		return "", "", noHostError(repo)
	}
	return first, rest, nil
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

// scpSplit splits scp syntax [user@]host:path. It applies only when a ":"
// comes before the first "/", and a single letter before it is a Windows
// drive (C:/repos/foo), not a host, which is the rule git itself uses.
// ok is false for anything else, or when the host is empty.
func scpSplit(repo string) (host, path string, ok bool) {
	colon := strings.Index(repo, ":")
	if colon < 0 {
		return "", "", false
	}
	if colon == 1 && (repo[0]|0x20 >= 'a' && repo[0]|0x20 <= 'z') {
		return "", "", false
	}
	if slash := strings.Index(repo, "/"); slash >= 0 && slash < colon {
		return "", "", false
	}
	// User info is only stripped from the part before the colon, so an
	// "@" in the path is left alone.
	rest := stripUserInfo(repo[:colon]) + repo[colon:]
	if strings.HasPrefix(rest, "[") {
		// Bracketed IPv6: git@[::1]:owner/repo. Cut after the bracket.
		host, path, ok = strings.Cut(rest[1:], "]:")
	} else {
		host, path, ok = strings.Cut(rest, ":")
	}
	if !ok || host == "" {
		return "", "", false
	}
	return host, path, true
}

// stripUserInfo drops "user@" or "user:pass@" from the front of s, that
// is everything through the last "@".
func stripUserInfo(s string) string {
	if at := strings.LastIndex(s, "@"); at >= 0 {
		return s[at+1:]
	}
	return s
}

// domainOf reduces a host to the directory name used under the projects
// root: the second-level label ("stash.funville.com" -> "funville"), the
// whole address for IPs, or the only label for single-label hosts.
func domainOf(host string) string {
	host = strings.ToLower(host)
	if net.ParseIP(host) != nil {
		return host
	}
	labels := strings.FieldsFunc(host, func(r rune) bool { return r == '.' })
	if len(labels) == 0 {
		return ""
	}
	return labels[max(len(labels)-2, 0)]
}
