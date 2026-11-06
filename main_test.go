package main

import (
	"testing"
)

func TestParseRepo(t *testing.T) {
	tests := []struct {
		in, want string
		wantErr  bool
	}{
		// Existing behavior. These define the directory scheme and must not change.
		{"git@github.com:sstephenson/bats.git", "github/sstephenson/bats", false},
		{"https://github.com/sstephenson/bats.git", "github/sstephenson/bats", false},
		{"https://github.com/JMCGARR/bats", "github/jmcgarr/bats", false},
		{"git@github.com:nebula-plugins/nebula-plugins.github.io.git", "github/nebula-plugins/nebula-plugins.github.io", false},
		{"ssh://git@stash.funville.com:1234/~jmcgarr/homedir.git", "funville/users/jmcgarr/homedir", false},
		{"http://jmcgarr@stash.funville.com/scm/~jmcgarr/homedir.git", "funville/users/jmcgarr/homedir", false},
		{"ssh://git@stash.funville.com:7999/hackday/template.git", "funville/hackday/template", false},
		{"http://jmcgarr@stash.funville.com/scm/hackday/template.git", "funville/hackday/template", false},

		// Inputs the v0.4 parser got wrong.
		{"https://github.com/jmcgarr/git-grab/", "github/jmcgarr/git-grab", false},
		{"https://github.com/jmcgarr/Mixed.Case.Repo", "github/jmcgarr/Mixed.Case.Repo", false},
		{"git@github.com:jmcgarr/repo-with.dot.git", "github/jmcgarr/repo-with.dot", false},
		{"https://gitlab.com/group/sub/project.git", "gitlab/group/sub/project", false},
		{"https://bitbucket.org/Team/Repo.git", "bitbucket/team/Repo", false},
		{"https://192.168.1.10/owner/repo.git", "192.168.1.10/owner/repo", false},
		{"ssh://git@localhost/owner/repo.git", "localhost/owner/repo", false},
		{"github.com/jmcgarr/git-grab", "github/jmcgarr/git-grab", false},
		{"https://user:pass@github.com/jmcgarr/repo.git", "github/jmcgarr/repo", false},
		{"ssh://git@github.com:22/jmcgarr/git-grab.git", "github/jmcgarr/git-grab", false},

		// Hardening beyond the spec table, found in review.
		{"ssh://git@[::1]/owner/repo.git", "::1/owner/repo", false},
		{"git@[::1]:owner/repo.git", "::1/owner/repo", false},
		{"https://github.com./jmcgarr/repo.git", "github/jmcgarr/repo", false},
		{"user@github.com/jmcgarr/repo", "github/jmcgarr/repo", false},
		{"https://github.com/jmcgarr/repo?ref=main#readme", "github/jmcgarr/repo", false},
		{"https://github.com/../../../../tmp/evil", "", true},
		{"https://github.com/owner/../repo", "", true},
		{"../owner/repo", "", true},
		{"./owner/repo", "", true},
		{"C:/repos/foo/bar", "", true},
		{"C:\\repos\\foo", "", true},
		{"https://stash.funville.com/~/repo.git", "", true},

		// Inputs that must return an error instead of panicking.
		{"git-grab", "", true},
		{"jmcgarr/git-grab", "", true},
		{"--depth", "", true},
		{"/Users/me/repo", "", true},
		{"file:///Users/me/repo.git", "", true},
		{"", "", true},
		{"https://github.com/jmcgarr", "", true},
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
