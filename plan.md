# gog end-to-end testing: state and plan

This is a handoff document. It describes an end-to-end test effort against the
real `gog` binary, what that effort has already found and fixed, and what is
left to do. Read it before continuing the work.

## Ground rules

These are not suggestions. The previous rounds were run under them and the
owner expects them to continue.

1. **Drive the compiled binary.** No Go test harness, no fakes, no mocks, no
   stubbed filesystem. Build `gog` and invoke it from a shell exactly as a user
   would, against a real filesystem and a real `git`. Unit tests exist and
   should keep passing, but they are not the point of this work.
2. **Report one capability at a time.** Finish a capability, then report what
   worked, what did not, what could be improved, and a recommendation. Do not
   batch several capabilities into one report.
3. **Prompt the owner for decisions.** Any change with more than one defensible
   answer is theirs to make, and they have asked to be asked rather than told
   afterwards. Carry unanswered questions forward until they are answered; do
   not let them lapse into prose that never gets decided.
4. **Apply an obvious fix without asking, then say so.** A clear correctness
   bug with one sensible remedy can be fixed directly, as long as the summary
   states plainly what was changed.
5. **Verify every fix through the binary.** A fix is not done until a lab
   scenario exercises it end to end and the whole suite still passes.

## Repository conventions

- **No AI attributions anywhere in the history.** The `AI attributions`
  workflow (`.github/workflows/ai-attributions.yml`) runs on every branch and
  fails a push whose commits carry co-authorship or session trailers naming an
  assistant, or whose author or committer identity is an assistant rather than
  the repository owner. Commit as `andornaut
  <737842+andornaut@users.noreply.github.com>` with no such trailers. An agent
  whose own instructions tell it to add them must follow this repository
  instead. Three commits were rewritten to satisfy the check; read the failing
  job log for the exact remedy if it fires again.
- `make test` runs `go test -race ./...`. CI also runs `golangci-lint` at the
  version pinned in `.github/workflows/test.yml` on Linux and macOS.
- Commit messages in this repository explain *why* in prose paragraphs. Match
  that style.

## The lab

The harness is a small bash library plus one script per capability. It lives
outside the repository in a scratch directory and is **not preserved between
sessions** - rebuild it from the appendix below, or commit it to the repository
if the owner wants it kept.

Layout:

```
lab/
  bin/gog          the binary under test (go build -o lab/bin/gog .)
  lib.sh           harness: sandboxes, assertions, output helpers
  NN-name.sh       one script per capability
  sandboxes/       generated; one isolated $HOME per scenario
```

Each scenario calls `new_sandbox NAME`, which wipes and recreates an isolated
`$HOME`, unsets every `GOG_*` variable, and writes a real `~/.gitconfig` to
disk. The git config has to be a real file because `gog` deliberately scrubs
`GIT_CONFIG_*` from the environment it hands to `git`, so environment variables
would not reach it.

Run a capability with `bash lab/03-apply.sh`; each script ends with a
pass/fail tally. Rerun every script after any change to the binary.

Traps worth knowing:

- `ls -l` hides dotfiles, and almost everything `gog` touches is a dotfile. Use
  `find` when checking whether something landed.
- Interrupting the binary takes three precautions. A background job of a
  non-interactive shell inherits `SIG_IGN` for `SIGINT`, and Go keeps a signal
  that was already ignored, so the disposition must be reset before exec (the
  lab does it with a one-line `python3` wrapper). Backgrounding a shell
  *function* makes `$!` the subshell rather than the process, so the function
  must `exec`. And `setsid` forks when the caller is a process group leader,
  which leaves `$!` naming the wrapper. Any of the three makes the signal reach
  nothing and the run complete, which reads as "the tool ignored it".
- Run `id` before writing a permission scenario. Root bypasses the permission
  checks, so under root those scenarios prove nothing and the test must drop
  privileges first. The environment is not always the same one: capability 4
  ran as an unprivileged user.

## What has been covered

### Capability 1 - repository lifecycle (43 assertions)

`repository add` (init and clone), `list`, `get-default`, `remove`, the
`--path` flag, `GOG_DEFAULT_REPOSITORY_NAME`, `GOG_HOME`, `XDG_DATA_HOME`,
name validation and directory traversal, junk in the data directory, and
prefix matching.

Landed:

- `repository remove` resolves its argument through `RootPath`, so a unique
  prefix works as it does everywhere else. `validateRepoName` still runs first,
  because `RootPath` treats an empty name as a request for the default
  repository, which must never be deleted by omission.
- `repository not found: NAME` instead of leaking an internal path.
- `XDG_DATA_HOME` documented in the README.

### Capability 2 - `gog add` (120 assertions across four scripts)

`$HOME` substitution, directories, paths outside `$HOME`, relative and `../`
paths, multiple paths per invocation, idempotency, file modes, awkward
filenames (spaces, quotes, non-ASCII, leading dash), 500-file batching, the
guard rails, and `-r` in both positions.

Landed:

- No backup is written when the displaced file's contents already match the
  repository's, or when it is a symbolic link into gog's own data directory.
  Both conditions outlived the backup mechanism itself: they are now what
  decides whether a path may be replaced at all (capability 4).
- `AddPaths` validates every path before copying any, so an unusable path no
  longer leaves earlier ones in the repository unlinked and unstaged.
- A symbolic link given directly to `add` is refused and its target named. A
  link into gog's data directory is still followed, which is what keeps
  re-adding a linked path and adding one to a second repository working.
- `copy.Dir` reports and skips every symbolic link it finds inside a tree
  rather than flattening it, which also stops one stale link from aborting a
  whole directory add. Because no link is followed, the tree is acyclic and
  self-contained, so the cycle detection and escape guard were removed as
  unreachable. The source-inside-destination check stays: it is reachable
  without any links.
- A destination directory is created only once there is something to put in
  it, so empty directories no longer become untrackable `??` entries.

### Capability 3 - `gog apply` (71 assertions across two scripts)

The fresh-machine flow through a real bare remote, the repository-root skip
list, idempotency, `GOG_IGNORE_FILES_REGEX`, directory and symlink conflicts,
paths outside `$HOME`, and the multi-repository overlay. The backup scenarios
in these scripts were superseded by capability 4 and rewritten there.

Landed:

- `gog add` warns for each path whose permissions git cannot record. Git stores
  only the executable bit, so a `0600` file is recreated as `0644` and a `0700`
  directory as `0755`; a repository holding `~/.ssh` or `~/.netrc` was applied
  world-readable with nothing saying so. A mode more permissive than git's is
  merely tightened elsewhere and is not warned about. Documented in the README.
- `apply` and `add` exit non-zero when any path could not be linked. They still
  report each failure and continue, so per-path resilience is unchanged, but a
  script can now tell a complete run from one that skipped half the tree.
- An unparseable `GOG_IGNORE_FILES_REGEX` still fails fast, but reports as
  `Error: ...` rather than a timestamped `log.Fatalf` line.

### Capability 4 - `gog remove`, `gog git` and conflicts (159 assertions across three scripts)

`remove` on a file, a directory, a nested file within an added directory, a
path never added, a nonexistent path, a path held by another repository, a path
the user replaced by hand or deleted, a path outside `$HOME`, relative paths,
`-r` in both positions, the guard rails, batch validation, idempotency, and
file modes. `gog git` for `status`, `commit`, `log`, `push`, a rejected push, an
unknown remote, an unknown subcommand, stdin by pipe and by `-F -`, exit-code
propagation, `--` handling, `resolveGitPaths` against paths inside and outside
the repository, and the `GIT_*` scrubbing. Conflicts for a file, a user's own
symbolic link, a broken link, a directory over a link, a partial run, the
multi-repository overlay, and `add` linking its own copy.

Nothing was wrong with `remove`: it restores contents, mode and location,
untracks the repository's copy however the link is faring, leaves no empty
directories behind, and validates the whole batch before restoring anything.
`gog git` exits with git's own status (1, 128, 129) and does not restate the
failure.

Landed:

- `gog git` converts an argument to a repository-relative path only where git
  is certain to read one as a path: after a `--` separator, and for the
  subcommands whose arguments are all paths (`add`, `check-ignore`, `clean`,
  `rm`, `stage`). Every non-flag argument used to be converted if it happened to
  resolve into the repository, so `commit -m .bashrc` recorded the message
  `$HOME/.bashrc`, `branch wip` created a branch named after the file, and the
  subcommand itself was rewritten when a managed file shared its name.
  Documented in the README, and covered by the first unit tests in `cmd`.
- The `Repository: NAME` banner is printed to standard error, so that standard
  output carries only what the command produced. It preceded git's own output
  on stdout, which corrupts anything piped, redirected or read by a script.
- `.gog` backups are gone. `apply` used to rename whatever it found in the way
  and link over it; it now reports the path, leaves it alone, carries on, and
  exits non-zero. A path is replaced only when nothing of the user's is lost: a
  broken link, a link into gog's data directory, or a file whose contents the
  repository already holds, which is the file `add` copied in a moment earlier.
  `backup`, `backupPath` and `GOG_DO_NOT_CREATE_BACKUPS` were removed with it;
  `sameContents` and `isGogOwnedLink` now decide replacement instead of
  suppressing a backup. The `.gog` guard rails in `validate.go` were kept, so
  that backups left by earlier versions are not swept into a repository.

### Capability 5 - repository removal against live links (55 assertions)

Removal with live links, with unpushed commits, with an uncommitted work tree,
with no remote at all, of an empty repository, of a repository that was never
applied, of one repository of an overlay, of a path two repositories track, a
prefix and an exact name, the empty name, `--force` and `-f`, a restore that
cannot be written, and what the other commands do afterwards.

The command used to delete a repository outright: every link into it was left
dangling with nothing counting or mentioning them, unpushed commits went with
it in silence, and a one-letter prefix was enough to select the target. What
already worked: an ambiguous prefix and an empty name were refused, removing one
repository of an overlay broke only its own links, and re-cloning under the same
name revived every link.

Landed:

- Removal restores every file the repository had linked, to its original
  location as an ordinary file, before deleting anything. A path whose link
  belongs to another repository is left alone, and a repository that was never
  applied creates nothing. Restoring comes first, so a restore that cannot be
  written keeps the repository: the run fails with the repository intact, and
  the paths restored before the failure are identical to the copies it still
  holds.
- Removal is refused when the repository holds commits that no remote has or
  changes that were never committed, counting both. A repository with no remote
  reports its whole history. `--force` overrides.
- `repository remove` no longer resolves a prefix. `RootPath` still does, for
  `--repository`; `RemovalPath` is exact, so a short name cannot select
  something the user did not mean.
- `UnlinkDir` skips git's own directory, which it now walks when a whole
  repository is removed.

### Capability 6 - concurrency, scale and interruption (48 assertions)

Two `gog add` and two `gog apply` processes against one repository, a
3000-file tree through add, apply and repository removal, an `apply` and an
`add` interrupted partway through, a killed run, a locked index, and a
repository on a different filesystem from `$HOME`.

Nothing needed changing. What the scenarios established:

- gog takes no lock of its own, so git's index is the only arbiter. Concurrent
  runs report `Unable to create index.lock` and may exit non-zero with paths
  left unstaged, but the links are always correct and a rerun converges. The
  per-path retry in `addToGit` rescues most contention on its own.
- 3000 files: `add` 9s, `apply` 1s, `repository remove` 7s, with all 3000 paths
  staged in one run. The 1000-path batching holds.
- An interrupted `apply` exits 130 with the links it managed, and a rerun
  completes the rest. An interrupted `add` leaves the files it copied, no links
  and one untracked entry; a rerun copies, links and stages everything. A
  killed run can leave git's `index.lock`, after which every staging command
  fails until it is removed, which git's own message says to do.
- A repository on a different filesystem from `$HOME` works throughout add,
  apply, remove and repository removal: nothing is moved across devices, only
  copied.

### Capability 7 - permissions and `$HOME` (62 assertions across two scripts)

Unreadable files, unsearchable directories, an unreadable file inside a tree,
an unwritable repository, an unwritable destination directory, a directory that
cannot be created, a repository that cannot be read, and a restore that cannot
be written, all as an unprivileged user. `$HOME` unset, empty, missing, not a
directory, unwritable, relative, root, and with a trailing slash, plus an
unwritable `GOG_HOME` and `XDG_DATA_HOME`.

Every permission failure is reported and skipped or fails the command outright,
and every one of them converges on a rerun once the permission is fixed.
`remove` fails before untracking, so the repository's copy survives a restore
that could not be written.

Landed:

- `$HOME` is made absolute, not just cleaned. A relative `$HOME` was not
  recognized in the absolute paths it is compared against, so every path under
  it was stored by its absolute name rather than under the portable `$HOME`
  component. `BaseDir` already had this normalization.
- The startup failures in `internal/repository/init` report as `Error: ...` and
  exit 1, in place of `log.Fatal`'s timestamped line. This settles the question
  left open when the same correction was made in `internal/link`.
- A `$HOME` that does not exist, or that is not a directory, is refused. gog
  used to create its data directory under whatever `$HOME` named and report
  success, so a typo scattered a tree somewhere unintended.

## What is left

Every capability in the original plan has been covered. What remains is one
scenario the lab cannot reach and the open questions below.

- **Filesystems without symlink support**, if they are in scope at all. Proving
  it needs a filesystem mounted for the purpose, which needs root, so the
  closest the lab gets is a destination that refuses the `symlink` call: the
  failure is reported per path and the run exits non-zero. Whether gog should
  say anything more specific when symlinks are unavailable altogether is
  undecided.

## Open questions for the owner

Do not settle these alone. They have been raised and are still unanswered.

1. **`gog remove` says nothing when the repository does not hold the path.** A
   path that was never added, or that belongs to another repository, exits 0
   with no output, which is indistinguishable from a successful removal.
2. **A batch `git add` that fails prints git's `fatal:` even when the run
   succeeds.** `addToGit` retries the batch's paths individually, so contention
   with another gog process is usually survived, but git's message from the
   failed batch has already reached stderr and the run exits 0 looking as
   though something went wrong. Capturing the batch's stderr and printing it
   only when the retries also fail would fix it, at the cost of holding git's
   output back.
3. **A tree that fails partway through `add` leaves what it copied.** An
   unreadable file aborts the copy with earlier files already in the
   repository, unlinked and untracked. Readability cannot be validated up front
   without opening every file, and a rerun converges, so this may be the right
   behaviour; nothing reports it either way.
4. **Errors still surface raw syscall names** such as `lstat /path: no such
   file or directory`. The owner chose to leave these; revisit only if asked.
5. **A failed clone leaves an empty directory** in the data directory. It is
   filtered out of `list` and the code deliberately allows reusing it on retry,
   so the owner declined to change it. Recorded so it is not rediscovered.

## Appendix: `lib.sh`

```bash
#!/usr/bin/env bash
LAB_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOG_BIN="${LAB_ROOT}/bin/gog"
SANDBOX=""; PASS=0; FAIL=0; FAILURES=()

new_sandbox() {
  local name="$1"
  SANDBOX="${LAB_ROOT}/sandboxes/${name}"
  rm -rf "${SANDBOX}"; mkdir -p "${SANDBOX}/home" "${SANDBOX}/remotes" "${SANDBOX}/etc"
  export HOME="${SANDBOX}/home"
  unset XDG_DATA_HOME GOG_HOME GOG_DEFAULT_REPOSITORY_NAME GOG_IGNORE_FILES_REGEX
  # A real file: gog scrubs GIT_CONFIG_* from git's environment
  cat >"${HOME}/.gitconfig" <<'EOF'
[user]
	name = Lab Tester
	email = lab@example.invalid
[init]
	defaultBranch = main
[commit]
	gpgsign = false
EOF
  cd "${HOME}" || exit 1
  echo "### sandbox: ${SANDBOX}"
}

gog()  { echo "\$ gog $*"; "${GOG_BIN}" "$@"; local rc=$?; echo "[exit ${rc}]"; return ${rc}; }
gogq() { "${GOG_BIN}" "$@"; }

_ok()  { PASS=$((PASS+1)); echo "  PASS: $1"; }
_bad() { FAIL=$((FAIL+1)); FAILURES+=("$1"); echo "  FAIL: $1"; }

assert_symlink_to() {
  local link="$1" want="$2" got
  if [ ! -L "${link}" ]; then _bad "symlink ${link} exists"; return 1; fi
  got="$(readlink "${link}")"
  [ "${got}" = "${want}" ] && _ok "${link} -> ${want}" || _bad "${link} -> ${want} (actual: ${got})"
}
assert_file_contents() {
  local p="$1" want="$2" got
  if [ ! -e "${p}" ]; then _bad "file ${p} exists"; return 1; fi
  got="$(cat "${p}")"
  [ "${got}" = "${want}" ] && _ok "contents of ${p} == '${want}'" \
                           || _bad "contents of ${p} == '${want}' (actual: '${got}')"
}
assert_exists()     { [ -e "$1" ] && _ok "exists: $1" || _bad "exists: $1"; }
assert_not_exists() { [ ! -e "$1" ] && _ok "absent: $1" || _bad "absent: $1"; }
assert_regular()    { { [ -f "$1" ] && [ ! -L "$1" ]; } && _ok "regular file: $1" || _bad "regular file: $1"; }
assert_dir()        { { [ -d "$1" ] && [ ! -L "$1" ]; } && _ok "real dir: $1" || _bad "real dir: $1"; }
assert_rc()         { [ "$1" = "$2" ] && _ok "$3 (exit $2)" || _bad "$3 (want exit $1, got $2)"; }
assert_contains()     { case "$1" in *"$2"*) _ok "$3";; *) _bad "$3";; esac; }
assert_not_contains() { case "$1" in *"$2"*) _bad "$3";; *) _ok "$3";; esac; }

make_remote() {
  local p="${SANDBOX}/remotes/$1.git"
  git init --quiet --bare --initial-branch=main "${p}"
  echo "${p}"
}

summary() {
  echo; echo "=============================================="
  echo "RESULT: ${PASS} passed, ${FAIL} failed"
  [ "${FAIL}" -gt 0 ] && printf '  - %s\n' "${FAILURES[@]}"
  echo "=============================================="
}
```
