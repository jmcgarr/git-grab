package main

import (
	"testing"
)

func AssertEquals(t *testing.T, expected string, actual string) {
	if expected != actual {
		t.Errorf("Expected [%s], but got: [%s]", expected, actual)
	}
}

func TestScenarios(t *testing.T) {
	AssertEquals(t, "github/sstephenson/bats", ParseRepo("git@github.com:sstephenson/bats.git"))
	AssertEquals(t, "github/sstephenson/bats", ParseRepo("https://github.com/sstephenson/bats.git"))
	AssertEquals(t, "github/jmcgarr/bats", ParseRepo("https://github.com/JMCGARR/bats"))
	AssertEquals(t, "github/nebula-plugins/nebula-plugins.github.io", ParseRepo("git@github.com:nebula-plugins/nebula-plugins.github.io.git"))
	AssertEquals(t, "funville/users/jmcgarr/homedir", ParseRepo("ssh://git@stash.funville.com:1234/~jmcgarr/homedir.git"))
	AssertEquals(t, "funville/users/jmcgarr/homedir", ParseRepo("http://jmcgarr@stash.funville.com/scm/~jmcgarr/homedir.git"))
	AssertEquals(t, "funville/hackday/template", ParseRepo("ssh://git@stash.funville.com:7999/hackday/template.git"))
	AssertEquals(t, "funville/hackday/template", ParseRepo("http://jmcgarr@stash.funville.com/scm/hackday/template.git"))
}
