# gog - Go Overlay Git

[![CI](https://github.com/andornaut/gog/actions/workflows/release.yml/badge.svg)](https://github.com/andornaut/gog/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

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

Requires [Go](https://golang.org/doc/install) and
[Make](https://www.gnu.org/software/make/).

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
| `gog list` | Print the paths the repository holds |
| `gog rm <paths>...` | Restore paths as ordinary files and untrack them |
| `gog git ...` | Run git in the repository's directory |
| `gog repository add <name> [url]` | Create a repository, by clone if a URL is given |
| `gog repository default` | Print the default repository |
| `gog repository list` | Print every repository |
| `gog repository rm <name>` | Restore what the repository holds, then delete it |

| Flag | Commands | Description |
| --- | --- | --- |
| `-r, --repository NAME` | `add`, `apply`, `list`, `rm`, `git` | Repository to use. A unique prefix of the name is accepted |
| `-s, --status` | `list` | Print what applying would do to each path |
| `--path` | `repository default`, `repository list` | Print paths instead of names |
| `--force` | `add` | Take a path over from the repository that manages it |
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

A repository with no `root/` has nothing to link, and `gog apply` and `gog list`
say so rather than exiting silently.

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

- A symbolic link that gog created is followed, so a path the repository already
  holds can be added again.
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
  which is the one `gog add` and `gog rm` mean.
- Skipping the irregular entries lets a directory such as `~/.gnupg` be added
  while the agent sockets in it are left alone.
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
- a link into gog's data directory, left by an earlier run or by another
  repository that tracks the same path
- a file whose contents the repository already holds, which is what `gog add`
  leaves behind after copying it in

### `gog list`

```text
$ gog list --status
linked   /home/example/.bashrc
missing  /home/example/.vimrc
replace  /home/example/.inputrc
conflict /home/example/.gitconfig
```

| State | What `gog apply` would do |
| --- | --- |
| `linked` | Nothing. The link is already there |
| `missing` | Link it. Nothing is at that path |
| `replace` | Discard what is there, then link it |
| `conflict` | Report it and leave it alone |

A repository's own `.git`, `.gitignore`, `LICENSE` and `README.md` are never
linked, and neither is anything `GOG_IGNORE_FILES_REGEX` matches. `gog list`
leaves them out.

### `gog git`

Runs git in the repository's directory and exits with git's own status.

- A path argument is rewritten to the file inside the repository that its link
  points at, but only where git is certain to read an argument as a path: after
  a `--` separator, and for `add`, `check-ignore`, `clean`, `rm` and `stage`.
  Everywhere else it is passed through, so `gog git commit -m .bashrc` records
  the message `.bashrc` and `gog git branch wip` creates a branch.
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
- Behaves the same whether the link is still there, was replaced with a file
  of your own, or was deleted.
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

- A repository with no remote at all reports its whole history. Push it, or
  pass `--force`.
- Nothing is restored or deleted until this check passes.
- Once it does, every file the repository had linked is restored as an ordinary
  file, leaving alone any path whose link belongs to another repository.
- The name is given in full: unlike `-r`, a prefix is not accepted.

### Output

| Stream | Carries |
| --- | --- |
| stdout | What a caller consumes: everything `git`, `list`, `repository list` and `repository default` print |
| stderr | What gog did and what went wrong: the `Repository:`, `Linked:`, `Restored:`, `Skipped:`, `Added repository:` and `Removed repository:` lines, and `Warning:` and `Error:` |

So `gog list | xargs` and `gog apply > log` each carry one kind of thing.

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
overlapping paths, and the one applied last owns the link.

```bash
for repoName in $(gog repository list | sort -r); do
  gog apply --repository ${repoName}
done
```

## Configuration

`$HOME` must name a directory that exists.

| Variable | Description |
| --- | --- |
| `GOG_DEFAULT_REPOSITORY_NAME` | Repository to use when `-r` is not given. Default: the first repository, which `gog repository default` prints |
| `GOG_HOME` | Where gog stores repositories. Default: `${XDG_DATA_HOME}/gog` if set, otherwise `${HOME}/.local/share/gog` |
| `GOG_IGNORE_FILES_REGEX` | Do not link paths that match this regular expression, named as they sit under `root/` |

```bash
export GOG_IGNORE_FILES_REGEX='\.swp$|\.tmp$'   # Vim temporary files
export GOG_IGNORE_FILES_REGEX='\.cache/'        # everything under .cache
export GOG_IGNORE_FILES_REGEX='^secrets\.env$'  # one file, by name
```

The expression is matched against paths relative to `root/`, so `^secrets\.env$`
names a file at the top of the linked tree. It is read only by the commands that
link: `add`, `apply` and `list`.

## Releasing

```bash
git tag -a v0.1.0 -m 'Release v0.1.0'
git push origin v0.1.0
```

The [release workflow](.github/workflows/release.yml) builds every platform in
the table above, and [GoReleaser](https://goreleaser.com/) publishes the
archives and their checksums. Every push to `main` republishes the `dev`
release from the same builds.

## Developing

| Command | Description |
| --- | --- |
| `make build` | Build `./gog` |
| `make test` | Run the tests with the race detector and coverage |
| `make coverage` | Report coverage |
| `make lint` | Run golangci-lint |
| `make fmt` | Rewrite the source with golangci-lint's formatter |
| `make install`, `make uninstall` | Copy to `/usr/local/bin` and remove it. Both use sudo |
