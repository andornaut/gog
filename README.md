# gog - Go Overlay Git

[![Release](https://github.com/andornaut/gog/actions/workflows/release.yml/badge.svg)](https://github.com/andornaut/gog/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/license/MIT)

Link files to Git repositories.

`gog` copies a file into a Git repository and replaces the original with a
symbolic link to it, so that dotfiles in `${HOME}` or elsewhere can be
committed, pushed, and applied on another machine. It supports multiple
repositories, which can separate personal and work files.

## Installation

### Pre-compiled binary

Archives are published on the
[releases page](https://github.com/andornaut/gog/releases): one per tagged
version, plus a `dev` release rebuilt on every push to `main`.

| Platform | Asset |
| --- | --- |
| Linux x86_64 | `gog_linux_x86_64.tar.gz` |
| Linux arm64 | `gog_linux_arm64.tar.gz` |
| macOS Apple Silicon | `gog_darwin_arm64.tar.gz` |

The archive also carries `LICENSE` and `README.md` at the top level, so name
the binary rather than extracting everything into the current directory:

```bash
tar -xzf gog_linux_x86_64.tar.gz gog
sudo install -m 755 gog /usr/local/bin/gog
```

### Compile from source

Requires [Go](https://go.dev/doc/install) and
[Make](https://www.gnu.org/software/make/). `make install` copies the binary to
`/usr/local/bin` with `sudo`; `PREFIX` chooses another destination.

```bash
git clone https://github.com/andornaut/gog.git
cd gog
make install
```

## Getting started

```bash
# Clone a repository and add a file to it
gog repository add dotfiles https://example.com/user/dotfiles.git
gog add ~/.config/foorc
# Linked: /home/example/.config/foorc -> /home/example/.local/share/gog/dotfiles/root/\$HOME/.config/foorc

# Publish it
gog git commit -am 'Add foo config'
gog git push

# On another machine
gog repository add dotfiles https://example.com/user/dotfiles.git
gog apply
```

## Commands

| Command | Description |
| --- | --- |
| `gog add <paths>...` | Copy paths into the repository, link them back, and stage them |
| `gog apply` | Link the repository's contents to the filesystem |
| `gog ls` | Print the paths the repository holds |
| `gog rm <paths>...` | Restore paths as ordinary files and untrack them |
| `gog git ...` | Run git in the repository's directory |
| `gog repository add <name> [url]` | Create a repository, by clone if a URL is given |
| `gog repository default` | Print the default repository |
| `gog repository ls` | Print every repository |
| `gog repository rm <name>` | Restore what the repository holds, then delete it |

| Flag | Commands | Description |
| --- | --- | --- |
| `-r, --repository NAME` | `add`, `apply`, `ls`, `rm`, `git` | Repository to use. A unique prefix of the name is accepted |
| `-s, --status` | `ls` | Print what applying would do to each path |
| `--path` | `repository default`, `repository ls` | Print paths instead of names |
| `--force` | `add` | Take a path over from the repository that manages it |
| `--force` | `apply` | Replace links that another repository made |
| `--force` | `repository rm` | Delete even if the repository holds work that no remote has |
| `--version` | `gog` | Print the version |

A short flag means the same thing in every tool that has it, so `--path`,
`--force` and `--version` are spelled out: `-p`, `-f` and `-v` each mean
something else in `mrs`.

Run `gog help <command>` for full usage.

## Behaviour

### Repository layout

A repository keeps what it links under `root/`, whose tree mirrors the
filesystem, and everything beside it belongs to the repository itself:

```text
dotfiles/
  root/
    $HOME/.bashrc     -> ~/.bashrc
    etc/hosts         -> /etc/hosts
  .github/            never linked, as is anything else at this level
  .gitignore
  LICENSE
  README.md
```

A repository with no `root/` has nothing to link, and `gog apply` and `gog ls`
say so rather than exiting silently. A `root/` that is a file or a symbolic link
counts as none, and `gog add` refuses to write through it.

### `$HOME` substitution

- A path under the home directory is stored with a literal `$HOME` component:
  `~/.bashrc` is stored as `root/$HOME/.bashrc`.
- `gog apply` expands it to the home directory of whoever runs it, so one
  repository serves several users and machines.
- A path outside the home directory is stored by its absolute name.
- The repository directory is named `$HOME` literally. gog prints it escaped as
  `\$HOME`, so that the path can be pasted into a shell.

### What gog manages

| Kind | Given to `gog add` | Found inside an added directory |
| --- | --- | --- |
| File, directory | Added | Added |
| Symbolic link | Refused, names its target instead | Skipped, with a warning |
| Path another repository manages | Refused unless `--force` | Skipped, with a warning |
| Named pipe, socket, device node | Refused | Skipped, with a warning |
| A nested repository's `.git` | Refused | Skipped, with a warning |

- A symbolic link that gog created, one whose own target is in gog's data
  directory, is followed, so a path the repository already holds can be added
  again. A link of yours to a path that gog manages is refused like any other.
- A path that another repository manages is refused, because taking it over
  leaves that repository holding a copy nothing points at. Inside an added
  directory it is skipped instead, so that one managed file does not fail the
  whole directory. `--force` acts on the path you name, not on what a directory
  holds.

  ```text
  Error: "/home/example/.bashrc" is managed by repository work (remove it from there first, or pass --force to take it over)
  Warning: skipping /home/example/.config/foorc (repository work already manages it; remove it from there first)
  ```

- A path inside a repository is refused, naming the path it is linked from,
  which is the one `gog add` and `gog rm` mean. So is a path that reaches a
  repository through a symbolic link, and one the repository already holds by
  another name, reached through a symbolic link to a directory.
- A nested repository's `.git`, such as a plugin that its own git manages, is
  skipped: linking its files breaks that repository.
- Skipping the irregular entries lets a directory such as `~/.gnupg` be added
  while the agent sockets in it are left alone.
- A directory with nothing in it that can be added is skipped
  (`Skipped: ...`): git does not track directories.
- A file with more than one name is copied once per name. Git records contents
  per path, so a hard link is not preserved.
- Every path is checked before any is copied, so one unusable argument fails
  the command rather than leaving the repository half modified.

### File permissions

Git records only the executable bit. A file added as `0600` is recreated as
`0644` by the clone, and a directory added as `0700` as `0755`. `gog add`
warns:

```text
Warning: /home/example/.netrc has mode 0600, which git does not record; it will be applied as 0644 on another machine
```

A directory above an added path is created as `0755` where it is missing, so
one that withholds access is reported too, outermost first. The home directory
and everything above it are left out.

Track `~/.ssh` or `~/.netrc` only if you accept that they will be
world-readable wherever the repository is applied.

### Paths that are already there

`gog apply` never deletes a file it did not put there. It reports the path,
leaves it alone, carries on with the rest of the repository, and fails:

```text
Error: "/home/example/.bashrc" already exists (move or remove it, then run the command again)
Error: some paths could not be linked
```

A path is replaced without asking only when nothing of yours is lost:

- a broken symbolic link
- a file whose contents the repository already holds, which is what `gog add`
  leaves behind after copying it in
- with `--force`, another repository's link to the same path

Another repository's link is otherwise a conflict, naming that repository.

Where the repository holds a file and the path is a directory, the directory is
replaced only when it holds nothing but broken links into the repository: what
an earlier run linked from a directory the repository has since turned into a
file. Anything else in it makes the path a conflict.

A path in a directory gog cannot write to is a conflict too, and `gog add`
refuses it before copying anything. Running gog under `sudo -E` to manage a path
such as `/etc/hosts` works, but leaves files that root owns in the repository.

A symbolic link to a directory, such as a `~/.config` that points elsewhere or
a home directory reached through a link, is kept, and the repository's files are
linked inside the directory it points at. A broken one of yours, such as a link
to a drive that is not mounted, is kept too and reported as a conflict, naming
its missing target.

A link to a file the repository no longer holds, such as one a pull deleted or
renamed, points at nothing. `gog apply` reports it and leaves it alone:

```text
Warning: /home/example/.old links to a file dotfiles no longer holds (remove the link; the file's last contents are in the repository's history)
```

### Repositories you did not write

`gog apply` links whatever a repository holds, including shell startup files,
`~/.ssh/authorized_keys`, and git's own configuration. Run
`gog ls --status -r NAME` and read what a cloned repository holds before its
first `apply`.

gog runs its own git commands with `core.fsmonitor` and `core.hooksPath`
disabled, so a git configuration that `apply` has just linked cannot run a
command from them. Clean filters still run when `apply` stages what it linked,
because git-crypt and git-lfs depend on them. A repository that links a git
configuration defining a filter, and a `.gitattributes` that uses it, runs that
filter during `apply`.

### `gog ls`

```text
$ gog ls --status
linked   /home/example/.bashrc
missing  /home/example/.vimrc
replace  /home/example/.inputrc
conflict /home/example/.gitconfig
stale    /home/example/.old
```

| State | What `gog apply` would do |
| --- | --- |
| `linked` | Nothing. The link is already there |
| `missing` | Link it. Nothing is at that path |
| `replace` | Discard what is there, then link it |
| `conflict` | Report it and leave it alone |
| `stale` | Report it and leave it alone. It links to a file the repository no longer holds |

A path whose name holds a newline, a tab or another character that is not
printable is printed with that character escaped, as `\n` or `\x1b`, here and in
every message.

A repository's own `.git`, `.gitignore`, `LICENSE` and `README.md` are never
linked. `gog ls` leaves them out.

### `gog git`

Runs git in the repository's directory and exits with git's own status.

- A path argument is rewritten to the file inside the repository that its link
  points at, but only where git is certain to read an argument as a path: after
  a `--` separator, and for the operands of `add`, `check-ignore`, `clean`, `rm`
  and `stage` when one of those is the first argument. Everywhere else it is
  passed through, so `gog git commit -m .bashrc` records the message `.bashrc`
  and `gog git branch wip` creates a branch. A global flag before the subcommand
  leaves it unidentified, so the path in `gog git -C . add ~/.bashrc` is passed
  through, and only a `--` separator still converts one.
- `-r NAME` has to be the first argument. Anywhere else it belongs to git, so
  `gog git branch -r` and `gog git ls-tree -r HEAD` keep their meaning.
- `--help` reaches git. Run `gog help git` for gog's own.
- The `GIT_*` variables that bind git to a repository, an index, or a
  configuration source are removed from its environment, so an enclosing git
  invocation such as a hook cannot redirect it.

```bash
gog git add ~/.bashrc      # resolves to the repository's copy
gog git log -- ~/.bashrc   # the same, after the separator
```

### `gog rm`

- Restores each path as an ordinary file, then drops it from the repository and
  the index.
- A path whose link was deleted is given the file back. A path holding a file
  of your own, or another repository's link, is left alone.
- A symbolic link that the repository holds is restored as that link, not as a
  copy of what it points at.
- A path that cannot be examined, or that reaches the repository through a
  symbolic link, fails the command before the repository gives up its copy.
- Reports a path the repository never held (`Skipped: ...`) rather than failing.
- Validates the whole batch before restoring anything.
- Not `gog git rm`, which deletes the repository's copy and stages the deletion,
  leaving the link outside it pointing at nothing. `gog rm` is the one that
  hands the file back.

### `gog repository rm`

Deleting a repository cannot be undone by cloning again, so it is refused
while the repository holds work that exists nowhere else:

```text
$ gog repository rm dotfiles
Error: refusing to remove dotfiles: it holds 1 commit that no remote has and 2 uncommitted changes (pass --force to delete it anyway)
```

- Work that exists nowhere else is commits that no remote has (including ones
  reachable only from a tag or a detached `HEAD`), stash entries, uncommitted
  changes, and ignored files.
- A repository with no remote at all reports its whole history. Push it, or
  pass `--force`.
- Nothing is restored or deleted until this check passes.
- Once it does, every file the repository had linked is restored as an ordinary
  file, leaving alone any path whose link belongs to another repository.
- The name is given in full: unlike `-r`, a prefix is not accepted.

### Output

| Stream | Carries |
| --- | --- |
| stdout | What a caller consumes: everything `git`, `ls`, `repository ls` and `repository default` print |
| stderr | What gog did and what went wrong: the `Repository:`, `Linked:`, `Restored:`, `Skipped:`, `Added repository:` and `Removed repository:` lines, and `Note:`, `Warning:` and `Error:` |

So `gog ls | xargs` and `gog apply > log` each carry one kind of thing.

| Code | Meaning |
| --- | --- |
| 0 | It worked |
| 1 | It failed |
| 2 | It was typed wrong: no command, an unknown command or flag, or a missing or extra operand |

A wrong invocation prints the usage that would have been right; a command that
ran and failed does not. `gog --help` writes help to stdout and reports success.

`gog git` exits with git's status instead.

### Multiple repositories

`gog apply` operates on one repository at a time. Repositories may hold
overlapping paths. The one applied first owns the link, and the others report
it as a conflict. `--force` makes the one applied last own it instead:

```bash
for repoName in $(gog repository ls | sort -r); do
  gog apply --force --repository ${repoName}
done
```

## Configuration

`$HOME` must name a directory that exists.

| Variable | Description |
| --- | --- |
| `GOG_DEFAULT_REPOSITORY_NAME` | Repository to use when `-r` is not given. Default: the first repository, which `gog repository default` prints |
| `GOG_HOME` | Where gog stores repositories, as an absolute path. Default: `${XDG_DATA_HOME}/gog` if that is absolute, otherwise `${HOME}/.local/share/gog` |

Every path a repository holds under `root/` is linked. To keep a file out of the
linked tree, keep it out of the repository, or store it beside `root/` where
nothing is linked from.

## Releasing

```bash
git tag -a v0.1.0 -m 'Release v0.1.0'
git push origin v0.1.0
```

The [release workflow](.github/workflows/release.yml) builds every platform in
the table above, and publishes nothing until the tests pass.
[GoReleaser](https://goreleaser.com/) builds and publishes a tag's archives.
Every push to `main` republishes the `dev` release from the archives the workflow
built itself. Both carry a `checksums.txt`.

## Developing

| Command | Description |
| --- | --- |
| `make build` | Build `./gog` |
| `make test` | Run the tests with the race detector and coverage |
| `make coverage` | Report coverage |
| `make lint` | Run golangci-lint |
| `make fmt` | Rewrite the source with golangci-lint's formatter |
| `make clean` | Run `go clean`, and remove `dist/` and `coverage.txt` |
| `make install`, `make uninstall` | Copy to `/usr/local/bin` and remove it. Both use sudo |
