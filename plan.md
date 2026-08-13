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

Two traps worth knowing:

- `ls -l` hides dotfiles, and almost everything `gog` touches is a dotfile. Use
  `find` when checking whether something landed.
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

## What is left

### Capability 5 - repository removal against live links

`gog repository remove` deletes a repository that may have hundreds of
symlinks pointing into it. Determine what is left behind on the filesystem and
whether anything warns the user. This is the evidence needed for the open
question about guarding that command.

### Capability 6 - concurrency, scale and interruption

Two `gog` processes against one repository; a very large tree; interrupting an
`apply` partway through and rerunning it; a repository on a different
filesystem from `$HOME`.

### Capability 7 - the pieces the lab has not reached

- Permission-denied paths, run as an unprivileged user rather than root.
- A repository whose git remote rejects a push, and what `gog git push`
  reports.
- Filesystems without symlink support, if that is in scope at all.
- `gog` invoked with `$HOME` unset or pointing somewhere unwritable.

## Open questions for the owner

Do not settle these alone. They have been raised and are still unanswered.

1. **`repository remove` is an unguarded `rm -rf`** of a git repository that
   may hold unpushed commits, and it now accepts a fuzzy prefix. The owner had
   no preference when asked and the command was left as it was. Capability 5
   should produce the evidence to ask again with.
2. **`gog remove` says nothing when the repository does not hold the path.** A
   path that was never added, or that belongs to another repository, exits 0
   with no output, which is indistinguishable from a successful removal.
3. **`internal/repository/repository.go` still calls `log.Fatal` twice in
   `init()`** (home directory lookup, data directory creation), in the same
   timestamped format that was just corrected in `internal/link`. Left alone
   because the decision taken was scoped to the ignore regex.
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
