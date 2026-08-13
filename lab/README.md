# lab

End-to-end tests that drive the compiled `gog` binary from a shell, exactly as
a user would, against a real filesystem and a real `git`. Nothing here is
mocked or stubbed. The Go tests under `internal/` cover units; these cover the
program.

## Running

```bash
bash lab/run-all.sh          # build the binary, run every script, print a tally
bash lab/04-remove.sh        # one script, with the full output of each scenario
```

`run-all.sh` exits non-zero if any script reports a failure. Run a single
script to see which assertion failed and what the binary printed around it.

Requires `bash`, `git`, `go`, `python3` (signal dispositions and unix sockets),
and `script` from util-linux (the terminal scenarios). Everything is written
under `lab/bin` and `lab/sandboxes`, both ignored by git.

**Do not run as root.** Root bypasses the permission checks, so those scenarios
would pass without proving anything; `09-permissions.sh` refuses to run.

## What each script covers

| Script | Covers |
| --- | --- |
| `01-repository.sh` | `repository add` by init and by clone, `list`, `get-default`, `--path`, `GOG_DEFAULT_REPOSITORY_NAME`, name validation, directory traversal, junk in the data directory, prefix matching, `GOG_HOME` and `XDG_DATA_HOME` |
| `02-add.sh` | `$HOME` substitution, directories, paths outside `$HOME`, relative paths, file modes, awkward filenames, batching across git invocations, the guard rails, symbolic links, `-r` in every position |
| `03-apply.sh` | The fresh-machine flow through a bare remote, the repository-root skip list, idempotency, `GOG_IGNORE_FILES_REGEX`, paths outside `$HOME`, the multi-repository overlay |
| `04-remove.sh` | Restoring a file, a directory and a nested path, paths another repository holds, links the user replaced or deleted, batch validation, modes |
| `05-git.sh` | Passthrough of `status`, `commit`, `log` and `push`, exit-code propagation, stdin, `--` handling, which arguments are resolved to repository paths, `GIT_*` scrubbing |
| `06-conflicts.sh` | Paths already in the way: refused and reported, never backed up or overwritten, and what may be replaced without asking |
| `07-repo-remove.sh` | `repository remove` restoring what it held, refusing unsaved work, `--force`, exact names, and a restore that cannot be written |
| `08-concurrency.sh` | Two processes against one repository, a 3000-file tree, an interrupted and a killed run, a locked index, a repository on another filesystem |
| `09-permissions.sh` | Unreadable files, unsearchable directories, unwritable repositories and destinations, and a restore that cannot be written |
| `10-home.sh` | `$HOME` unset, empty, missing, not a directory, unwritable, relative, root, and with a trailing slash |
| `11-repo-add.sh` | What a failed `repository add` leaves behind, and the directories it may reuse or refuse |
| `12-special-files.sh` | Named pipes, sockets and device nodes given directly and found inside a tree, hard links, and a file where a directory must be |
| `13-terminal.sh` | `gog git` with a pseudo-terminal: paging, colour, an editor, a prompt, and exit codes |

## Writing a scenario

Each scenario calls `new_sandbox NAME`, which wipes and recreates an isolated
`$HOME`, unsets every `GOG_*` variable, and writes a real `~/.gitconfig`. The
git config has to be a real file because gog scrubs `GIT_CONFIG_*` from the
environment it hands to git, so environment variables would not reach it.

`lib.sh` holds the sandbox and the assertions. `gog` runs the binary and echoes
the command; `gogq` runs it quietly. Every assertion prints `PASS` or `FAIL`
and feeds the tally that `summary` prints at the end.

Three things that will otherwise cost an afternoon:

- `ls -l` hides dotfiles, and almost everything gog touches is a dotfile. Use
  `find` when checking whether something landed.
- Interrupting the binary needs care. A background job of a non-interactive
  shell inherits `SIG_IGN` for `SIGINT`, and Go keeps a signal that was already
  ignored; backgrounding a shell *function* makes `$!` the subshell rather than
  the process; and `setsid` forks when the caller is a process group leader.
  Any of the three sends the signal nowhere and lets the run finish, which
  reads as though the tool ignored it. See `run_interruptible` in
  `08-concurrency.sh`.
- A path under `lab/sandboxes` is longer than the 108 bytes an `AF_UNIX`
  address allows, so a socket has to be bound from inside its directory. See
  `make_socket` in `12-special-files.sh`.
