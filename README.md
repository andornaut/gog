# gog - Go Overlay Git

Link files to Git repositories

- `gog` can be used to manage "dotfiles" in `${HOME}`, but it can also manage links to files elsewhere on the filesystem
- `gog` supports multiple git repositories, which can be useful, for instance, to separate personal and work files

## Installation

### Pre-compiled binary

Download one of the pre-compiled binaries from the
[releases page](https://github.com/andornaut/gog/releases), and then move it onto
your path: `chmod +x gog-linux-amd64 && sudo mv gog-linux-amd64 /usr/local/bin/gog`

### Compile and install from git

```
git clone ...
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
gog git commit -am 'Add sxhkd config'
gog git push

# Login to a remote machine and initialize the same git repository as above
ssh remote@example.com
gog repository add dotfiles https://example.com/user/dotfiles.git

gog apply

# Gog linked `~/.config/foorc` as above, while preserving any preexisting file at
# that location as ~/.config/.foorc.gog`
ls -l ~/.config/foorc | awk '{print $9,$10,$11}'
> /home/example/.config/foorc -> /home/example/.local/share/gog/dotfiles/$HOME/.config/foorc
```

## Usage

`gog --help`

```
Link files to Git repositories

Usage:
  gog [command]

Available Commands:
  add         Add files or directories to a repository
  apply       Link a repository's contents to the filesystem
  git         Run a git command in a repository's directory
  help        Help about any command
  remove      Remove files or directories from a repository
  repository  Manage repositories

Flags:
  -h, --help                help for gog
  -r, --repository string   name of repository

Use "gog [command] --help" for more information about a command.
```

`gog repository --help`

```
Manage repositories

Usage:
  gog repository [command]

Available Commands:
  add         Add a git repository
  get-default Print the name of the default repository
  list        Print the names of all repositories
  remove      Remove a repository

Flags:
  -h, --help   help for repository

Use "gog repository [command] --help" for more information about a command.
```

`gog add --help`

```
Add files or directories to a repository

Usage:
  gog add [paths...]

Flags:
  -h, --help                help for add
  -r, --repository string   name of repository to add to
```

`gog apply --help`

```
Link a repository's contents to the filesystem

Usage:
  gog apply

Flags:
  -h, --help                help for apply
  -r, --repository string   name of repository to apply
```

### Notes

#### `gog add`

If any of the path arguments to `gog add` begin with the current user's home
directory, then this prefix is replaced with an escaped `\$HOME` path
component, and then the `$HOME` variable is expanded when `gog apply` is run.

#### `gog apply`

`gog apply` does not support being run on multiple repositories at the same
time, because if multiple repositories link to the same files, then the order
in which they are applied may be significant. If you know that your
repositories do not overlap, then you can run `gog apply` on them all like so:

```bash
for repoName in $(gog repository list); do 
  gog apply --repository ${repoName}
done
```

## Configuration

You can set environment variables - typically by adding entries to `~/.bashrc`
or similar - to override a couple of defaults settings.

Environment variable | Description
---|---
GOG_DEFAULT_REPOSITORY_NAME | The repository to select when `--repository NAME` is not specified (default: the first directory in `${XDG_DATA_HOME}/gog`)
GOG_REPOSITORY_BASE_DIR | The directory which contains gog repositories (default: `${XDG_DATA_HOME}/gog`)

```bash
# ~/.bashrc
export GOG_DEFAULT_REPOSITORY_NAME="dotfiles"
export GOG_DEFAULT_REPOSITORY_NAME="${XDG_DATA_HOME}/gog
``` 

## Developing

```bash
# Install `dep`, ensure that `./vendor/` is up to date, and compile `./gog`
make

# Compile and install to /usr/local/bin/gog
make install

# Delete /usr/local/bin/gog
make uninstall

# Compile release binaries in `./dist/`
make release
```
