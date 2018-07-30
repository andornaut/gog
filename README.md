# gog - Go Overlay Git

Link files to Git repositories

## Installation

### Pre-compiled binary

Download one of the pre-compiled binaries from the
[releases page](https://github.com/andornaut/gog/releases), and then copy it to
your path: `chmod +x gog-linux-amd64 && cp gog-linux-amd64 /usr/local/bin/gog`

### Compile and install from git

```
git clone ...
cd gog
make install
```

## Getting started

```bash
gog repository add dotfiles https://example.com/user/dotfiles.git
gog add ~/.config/foorc
#> REPOSITORY: dotfiles
#> /home/user/.config/foorc -> /home/user/.local/share/gog/dotfiles/\$HOME/.config/foorc
gog git commit -am 'Add foorc'
gog git push

ssh remote@example.com
gog repository add dotfiles https://example.com/user/dotfiles.git
gog apply
#> REPOSITORY: dotfiles
#> /home/user/.config/foorc -> /home/user/.local/share/gog/dotfiles/\$HOME/.config/foorc
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

You can set the `GOG_DEFAULT_REPOSITORY_PATH` environment variable in order to
configure the default repository path to use when the `--repository NAME` flag
is omitted. If `$GOG_DEFAULT_REPOSITORY_PATH` is empty, then the first
directory in `${XDG_DATA_HOME}/gog/` will be selected automatically.

```bash
# ~/.bashrc
export GOG_DEFAULT_REPOSITORY_PATH="${XDG_DATA_HOME}/gog/dotfiles"
``` 

## Developing

```bash
# Install `dep`, ensure that `./vendor/` is up to date, and compile `./gog`
make

# Compile and install to /usr/local/bin/gog
make install

# Delete /usr/local/bin/gog
make uninstall

# Run tests
make test

# Compile release binaries in `./dist/`
make release
```
