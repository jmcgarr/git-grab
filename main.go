package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

//var log = logging.MustGetLogger("git-grab")

func main() {
	home := os.Getenv("HOME")
	usage := `Usage: 
  git grab <repository>`

	args := os.Args[1:] // get cmdline args without program
	if len(args) < 1 {
		fmt.Println(usage)
		os.Exit(1)
	}

	repoPath := ParseRepo(args[0])
	var cloneDir string = fmt.Sprintf("%s/Projects/%s", home, repoPath)
	cmd := exec.Command("git", "clone", args[0], cloneDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

// Parses the git repository path and determines the folder location on
// local file system
func ParseRepo(repo string) string {
	var domain string = ""
	var owner string = ""
	var repoName string = ""

	var noProtocol = strings.Split(repo, "://")
	var withoutGit = strings.Split(noProtocol[len(noProtocol)-1], "@")
	var basicRepo string = withoutGit[len(withoutGit)-1]
	basicRepo = strings.TrimSuffix(basicRepo, ".git")
	var ownerRepoSplit = strings.Split(basicRepo, "/")

	var tokens = strings.Split(basicRepo, ".")
	domain = tokens[len(tokens)-2]
	owner = ownerRepoSplit[len(ownerRepoSplit)-2]
	var colonSplit = strings.Split(owner, ":")
	owner = colonSplit[len(colonSplit)-1]
	owner = strings.Replace(owner, "~", "users/", 1) // for Stash, adding a subpath for user owned repos
	owner = strings.ToLower(owner)
	repoName = ownerRepoSplit[len(ownerRepoSplit)-1]

	return fmt.Sprintf("%s/%s/%s", domain, owner, repoName)
}
