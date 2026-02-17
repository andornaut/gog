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

# Gog linked `~/.config/foorc` as above, while preserving any preexisting file at
# that location as ~/.config/.foorc.gog`
ls -l ~/.config/foorc | awk '{print $9,$10,$11}'
> /home/example/.config/foorc -> /home/example/.local/share/gog/dotfiles/$HOME/.config/foorc
```

## Usage

`gog --help`

```text
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

Use "gog [command] --help" for more information about a command..
```

`gog repository --help`

```text
Manage repositories

Usage:
  gog repository [command]

Available Commands:
  add         Add a git repository
  get-default Print the name or path of the default repository
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
  gog add [paths...]

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
  -r, --repository string   name of repository to apply
```

### Notes

#### `${HOME}` Variable Substitution

**gog** automatically handles home directory portability:

- When you run `gog add` on a file in your home directory (e.g., `~/.config/foorc`), gog stores it in the repository with a `${HOME}` prefix instead of your actual home path (e.g., `/home/alice`)
- This makes your dotfiles repository portable across different users and systems
- When you run `gog apply` on another machine, `${HOME}` is automatically expanded to that system's home directory

**Example:**

```bash
# On your machine (/home/alice)
gog add ~/.bashrc
# Stored in repository as: ${HOME}/.bashrc

# On another machine (/home/bob)
gog apply
# Creates symlink: /home/bob/.bashrc -> /home/bob/.local/share/gog/dotfiles/${HOME}/.bashrc
```

This ensures your dotfiles work seamlessly across different users, machines, and even operating systems.

#### `gog add`

If any of the path arguments to `gog add` begin with the current user's home
directory, then this prefix is replaced with an escaped `\${HOME}` path
component, and then the `${HOME}` variable is expanded when `gog apply` is run.

#### `gog apply`

`gog apply` operates on a single repository at a time, but you can apply
multiple repositories - even if they contain partially overlapping files.

```bash
for repoName in $(gog repository list | sort -r); do
  gog --repository ${repoName} apply
done
```

## Configuration

You can use environment variables to customize some settings.

Environment variable | Description
--- | ---
GOG_DEFAULT_REPOSITORY_NAME | The repository to use when `--repository NAME` is not specified (default: the first directory in `${HOME}/.local/share/gog`)
GOG_DO_NOT_CREATE_BACKUPS | Do not create .gog backup files
GOG_HOME | The directory where gog stores its files (default: `${HOME}/.local/share/gog`)
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
