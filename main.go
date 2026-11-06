package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

func main() {
	home := os.Getenv("HOME")
	usage := `Usage: 
  git grab <repository>`

	args := os.Args[1:] // get cmdline args without program
	if len(args) < 1 {
		fmt.Println(usage)
		os.Exit(1)
	}

	repoPath, err := ParseRepo(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "git-grab:", err)
		os.Exit(1)
	}
	cloneDir := fmt.Sprintf("%s/Projects/%s", home, repoPath)
	cmd := exec.Command("git", "clone", args[0], cloneDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "git-grab:", err)
		os.Exit(1)
	}
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
			return "", fmt.Errorf("expected <owner>/<repo> in %q", repo)
		}
		if s != "" {
			segments = append(segments, s)
		}
	}
	if len(segments) > 0 && segments[0] == "scm" {
		segments = segments[1:]
	}
	if len(segments) < 2 {
		return "", fmt.Errorf("expected <owner>/<repo> in %q", repo)
	}

	// Owner: Stash user repos "~user" become "users/user".
	owner := segments[0]
	if strings.HasPrefix(owner, "~") {
		if len(owner) == 1 {
			return "", fmt.Errorf("expected <owner>/<repo> in %q", repo)
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

func noHostError(repo string) error {
	return fmt.Errorf("cannot determine host in %q; git-grab needs a remote URL", repo)
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
