# gog - Go Overlay Git

Link files to Git repositories

## Installation

```
git clone ...
cd gog
make install
```

## Getting started

```bash
gog repository add dotfiles https://example.com/user/dotfiles.git
gog add ~/.config/sxhkd
gog git commit -am 'Add sxhkd config'
gog git push
file .config/sxhkd/sxhkdrc 
#> .config/sxhkd/sxhkdrc: symbolic link to \
#> /home/user/.local/share/gog/dotfiles/$HOME/.config/sxhkd/sxhkdrc

ssh remote@example.com
gog repository add dotfiles https://example.com/user/dotfiles.git
gog apply
file .config/sxhkd/sxhkdrc 
#> .config/sxhkd/sxhkdrc: symbolic link to \
#> /home/user/.local/share/gog/dotfiles/$HOME/.config/sxhkd/sxhkdrc
```

## Usage

`gog --help`

```
NAME:
   gog - Go Overlay Git

USAGE:
   gog command [options] [arguments...]

DESCRIPTION:
   Link files to Git repositories

COMMANDS:
     repository  Manage repositories
     add         Add files or directories to a repository
     remove      Remove files or directories from a repository
     apply       Create symbolic links from a repository's files to the root filesystem
     git         Run a git command in a repository
```

`gog repository --help`

```
NAME:
   gog repository - Manage repositories

USAGE:
   gog repository command [options] [arguments...]

COMMANDS:
     add          Add and initialize a git repository
     remove       Remove a repository
     get-default  Print the name of the default repository
     list         Print the names of all repositories
```

`gog add --help`

```
NAME:
   gog add - Add files or directories to a repository

USAGE:
   gog add [--repository NAME] <path> [paths...]

OPTIONS:
   --repository NAME, -r NAME  NAME of the target repository
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
for repoName in $(gog repository list --path); do 
  gog apply --repository ${repoName}
done
```

## Configuration

You can set a `GOG_DEFAULT_REPOSITORY_PATH` environment variable in order to
configure the default repository path to use when the `--repository NAME` option
is not specified. If `$GOG_DEFAULT_REPOSITORY_PATH` is empty, then the first
directory in `${XDG_DATA_HOME}/gog/` will be selected automatically.

```bash
# ~/.bashrc
export GOG_DEFAULT_REPOSITORY_PATH="${XDG_DATA_HOME}/gog/dotfiles"
``` 
