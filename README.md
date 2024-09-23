git-grab
========
git-grab is a git subcommand that allows you to clone a git repository to a specific directory structure on your local machine.  This makes finding and organizing git repositories a snap.

Installation:
-------------
You can down the latest binaries from the [releases page](https://github.com/jmcgarr/git-grab/releases).  

Once you download the binaries, rename it to `git-grab` and add it to your path. 


Usage:
------
You can run the `git grab` command exactly like you would run a [`git clone`](https://git-scm.com/docs/git-clone) command. 

`git grab <options> <repo-url>`

The `git grab` command will checkout the git repository to your local machine using the following directory scheme:

`~/Projects/<server>/<owner>/<repo_name>/`

For example, running `git grab https://github.com/jmcgarr/git-grab.git` will checkout the repository to:

```
Projects
    └── github
          └── jmcgarr
                └── git-grab
```

Known Issues:
-------------
The following are known issues with the `git grab` command:

* 

