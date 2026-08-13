# gog - Go Overlay Git

Link files to Git repositories

- `gog` can be used to manage "dotfiles" in `${HOME}` or elsewhere on the filesystem
- `gog` supports multiple Git repositories, which can be useful to separate personal and work files

## Installation

### Pre-compiled binary

Download one of the pre-compiled binaries from the
[releases page](https://github.com/andornaut/gog/releases), and then move it onto
your path: `chmod +x gog-linux-amd64 && sudo mv gog-linux-amd64 /usr/local/bin/gog`

### Compile from source

Install dependencies:

- [Go](https://golang.org/doc/install)
- [Make](https://www.gnu.org/software/make/)

```bash
git clone https://github.com/andornaut/gog.git
cd gog
make install
```

## Getting started

```bash
# Clone a git repository and add a file to it
gog repository add dotfiles https://example.com/user/dotfiles.git
gog add ~/.config/foorc

# Gog moved `~/.config/foorc` into the default git repository ("dotfiles") and
# then created a symlink to it at its original location
ls -l ~/.config/foorc | awk '{print $9,$10,$11}'
> /home/example/.config/foorc -> /home/example/.local/share/gog/dotfiles/$HOME/.config/foorc

# Commit and push the changeset to make it available from elsewhere
gog git commit -am 'Add foo config'
gog git push

# Login to a remote machine and initialize the same git repository as above
ssh remote@example.com
gog repository add dotfiles https://example.com/user/dotfiles.git

gog apply

# Gog linked `~/.config/foorc` as above. Had a file of your own already been
# there, gog would have reported it and left it alone, exiting non-zero.
ls -l ~/.config/foorc | awk '{print $9,$10,$11}'
> /home/example/.config/foorc -> /home/example/.local/share/gog/dotfiles/$HOME/.config/foorc
```

## Usage

`gog --help`

```text
Link files to Git repositories

Usage:
  gog [flags]
  gog [command]

Available Commands:
  add         Add files or directories to a repository
  apply       Link a repository's contents to the filesystem
  git         Run a git command in a repository's directory
  help        Help about any command
  list        Print the paths that a repository holds
  remove      Remove files or directories from a repository
  repository  Manage repositories

Flags:
  -h, --help      help for gog
  -v, --version   version for gog

Use "gog [command] --help" for more information about a command.
```

`gog repository --help`

```text
Manage repositories

Usage:
  gog repository [flags]
  gog repository [command]

Available Commands:
  add         Add a git repository
  default     Print the name or path of the default repository
  list        Print the names or paths of all repositories
  remove      Remove a repository

Flags:
  -h, --help   help for repository

Use "gog repository [command] --help" for more information about a command.
```

`gog add --help`

```text
Add files or directories to a repository

Usage:
  gog add <paths>...

Flags:
  -h, --help                help for add
  -r, --repository string   name of repository
```

`gog apply --help`

```text
Link a repository's contents to the filesystem

Usage:
  gog apply

Flags:
  -h, --help                help for apply
  -r, --repository string   name of repository
```

`gog list --help`

```text
Print the paths that `gog apply` would link, as they appear outside the
repository. The files a repository keeps for itself and whatever
GOG_IGNORE_FILES_REGEX names are left out.

Usage:
  gog list

Flags:
  -h, --help                help for list
  -r, --repository string   name of repository
  -s, --status              print what applying would do to each path
```

### Notes

#### `$HOME` Variable Substitution

**gog** automatically handles home directory portability:

- When you run `gog add` on a file in your home directory (e.g., `~/.config/foorc`), gog stores it in the repository with a literal `$HOME` path component instead of your actual home path (e.g., `/home/alice`)
- This makes your dotfiles repository portable across different users and systems
- When you run `gog apply` on another machine, `$HOME` is automatically expanded to that system's home directory

**Example:**

```bash
# On your machine (/home/alice)
gog add ~/.bashrc
# Stored in repository as: $HOME/.bashrc

# On another machine (/home/bob)
gog apply
# Creates symlink: /home/bob/.bashrc -> /home/bob/.local/share/gog/dotfiles/$HOME/.bashrc
```

This ensures your dotfiles work seamlessly across different users, machines, and even operating systems.

#### File permissions

Git records only whether a file is executable, so **gog cannot carry any other
permission to another machine**. A file added as `0600` is recreated as `0644`
by the clone, and a directory added as `0700` is recreated as `0755`.

`gog add` warns whenever it stores a path whose permissions will be widened
this way:

```text
Warning: /home/example/.netrc has mode 0600, which git does not record; it will be applied as 0644 on another machine
```

Track secrets such as `~/.ssh` keys or `~/.netrc` only if you are content for
them to be world-readable wherever the repository is applied, or keep them out
of the repository and manage them separately.

#### `gog add`

If any of the path arguments to `gog add` begin with the current user's home
directory, then this prefix is replaced with a literal `$HOME` path
component, which is expanded to the home directory when `gog apply` is run.

gog manages files and directories. A named pipe, socket or device node has
nothing git can store, so one given directly to `gog add` is refused, and one
found inside a directory is reported and skipped while the rest of the tree is
added. This is what lets a directory such as `~/.gnupg` be added while the
agent sockets in it are left alone. Symbolic links are handled the same way: a
link given directly names its target instead, and a link inside a directory is
skipped rather than followed.

A file with more than one name is copied once per name. Git records contents
per path, so a hard link is not preserved: the repository holds two independent
files, and changing one no longer changes the other.

#### `gog git`

`gog git` runs git in the repository's directory and exits with git's own exit
status.

Because the files in your home directory are symbolic links, a path argument
has to be rewritten to the file inside the repository that it points at. gog
does this only where git is certain to read an argument as a path: after a `--`
separator, and for the subcommands whose arguments are all paths (`add`,
`check-ignore`, `clean`, `rm`, `stage`). Everywhere else the argument is passed
through untouched, so a branch name, a remote, or a commit message that happens
to match a managed file keeps its meaning.

```bash
# Both resolve to the repository's copy of ~/.bashrc
gog git add ~/.bashrc
gog git log -- ~/.bashrc

# Records the message ".bashrc", even though ~/.bashrc is managed
gog git commit -m .bashrc
```

Use `--` to limit any other subcommand to a path.

Because every argument is handed to git, the flag that selects the repository
has to lead: `gog git -r work status`. Written anywhere else it belongs to git,
which is what makes `gog git ls-tree -r HEAD` and `gog git branch -r` mean what
they say. For the same reason `gog git --help` reaches git; run `gog help git`
for gog's own.

gog prints the repository it selected (`Repository: dotfiles`) to standard
error, so that standard output carries only what the command itself produced
and `gog git` output can be piped or redirected.

#### `gog repository remove`

Removing a repository deletes its directory, which is the one thing gog does
that cloning again cannot undo. Two things happen first.

Every file the repository had linked is restored to its original location as an
ordinary file, so nothing is left pointing at a directory that no longer
exists. A path whose link belongs to another repository is left alone.

The repository is then checked for work that exists nowhere else, and removal is
refused if it finds any:

```text
$ gog repository remove dotfiles
Error: refusing to remove dotfiles: it holds 1 commit that no remote has and 2 uncommitted changes (pass --force to delete it anyway)
```

A repository with no remote at all reports its whole history, because that is
what would be lost. Push it, or pass `--force`. Unlike `--repository`, this
command does not accept a name prefix: the repository to delete is named in
full.

#### Files that are already there

gog never deletes a file it did not put somewhere. When `gog apply` finds
something of yours where a link belongs, it reports the path, leaves it alone,
carries on with the rest of the repository, and exits non-zero:

```text
Error: /home/example/.bashrc already exists (move or remove it, then run the command again)
Error: some paths could not be linked
```

Move or delete the file and apply again. Applying a repository to a machine
that is already configured is therefore a two-step operation: the first run
lists every conflict, and the second links what you cleared.

A path is replaced without asking only when nothing of yours is lost: a broken
symbolic link, a link into gog's own data directory (left by an earlier run or
by another repository that tracks the same path), or a file whose contents the
repository already holds, which is what `gog add` leaves behind after copying it
in.

#### `gog apply`

`gog apply` operates on a single repository at a time, but you can apply
multiple repositories - even if they contain partially overlapping files.

```bash
for repoName in $(gog repository list | sort -r); do
  gog apply --repository ${repoName}
done
```

`gog list` prints what a repository holds without touching anything, and
`--status` says where each path stands:

```text
$ gog list --status
linked   /home/example/.bashrc
missing  /home/example/.vimrc
replace  /home/example/.inputrc
conflict /home/example/.gitconfig
```

The state is what the next `apply` would do: `linked` is already done,
`missing` and `replace` are paths it would link, and `conflict` is one it would
report and leave alone. A path is `replace` when what is in the way is
something gog may discard, which is the same set of cases listed under
[Files that are already there](#files-that-are-already-there).

## Configuration

`$HOME` must name a directory that exists: gog stores paths under a literal
`$HOME` component and refuses to run when it cannot resolve one.

You can use environment variables to customize some settings.

Environment variable | Description
--- | ---
GOG_DEFAULT_REPOSITORY_NAME | The repository to use when `--repository NAME` is not specified (default: the first repository in gog's data directory, which `gog repository default` prints)
GOG_HOME | The directory where gog stores its files (default: `${XDG_DATA_HOME}/gog` if `XDG_DATA_HOME` is set, otherwise `${HOME}/.local/share/gog`)
GOG_IGNORE_FILES_REGEX | Do not link repository-relative file paths that match this regular expression

### GOG_IGNORE_FILES_REGEX Examples

Use regular expressions to skip specific files when running `gog apply`:

```bash
# Skip all .swp and .tmp files (Vim temporary files)
export GOG_IGNORE_FILES_REGEX='\.swp$|\.tmp$'
gog apply

# Skip everything in .cache directories
export GOG_IGNORE_FILES_REGEX='\.cache/'
gog apply

# Skip specific files by name
export GOG_IGNORE_FILES_REGEX='^secrets\.env$|^private\.key$'
gog apply

# Skip all files with "local" in the name
export GOG_IGNORE_FILES_REGEX='local'
gog apply
```

**Note:** The regex matches against repository-relative paths (after the repository root).

## Releasing

This project uses [GoReleaser](https://goreleaser.com/) to automate the release process.

To release a new version:

1. Ensure you're on the main branch with the latest changes:

   ```bash
   git checkout main
   git pull
   ```

2. Create and push a new version tag:

   ```bash
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
   ```

3. The [release workflow](.github/workflows/release.yml) will automatically:
   - Build binaries for Linux and macOS (amd64 and arm64)
   - Create a new GitHub Release with the built artifacts
   - Generate checksums for all binaries

## Developing

See the [Makefile](./Makefile).
