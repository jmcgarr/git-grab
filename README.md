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

`git grab [git-clone options] <repo-url>`

Every option other than `-h` and `--version` is passed to `git clone` unchanged, so `git grab --depth 1 <repo-url>` performs a shallow clone. Put options before the URL: the last argument that does not start with `-` is taken as the repository URL, so `git grab <repo-url> --branch main` would treat `main` as the URL.

Use `git grab -h` for help. `git grab --help` never reaches git-grab because git rewrites it to `git help grab` and looks for a man page; `--help` only works when the binary is invoked directly as `git-grab --help`.

The `git grab` command will checkout the git repository to your local machine using the following directory scheme:

`~/Projects/<server>/<owner>/<repo_name>/`

Set `GIT_GRAB_ROOT` to use a different root than `~/Projects`:

```
export GIT_GRAB_ROOT=$HOME/src
```

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

