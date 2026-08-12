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
- The container runs as **root**, so permission-denial scenarios prove nothing:
  root bypasses the checks. Any test that depends on a directory being
  unwritable must drop privileges first. This is why capability 3's
  unwritable-directory case is still listed as uncovered below.

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
  `add` used to leave a byte-identical hidden duplicate beside every adopted
  file, and the multi-repository loop in the README rewrote a `.gog` file for
  every overlapping path on every run, overwriting the user's genuine pre-gog
  backup with a pointer into whichever repository lost the race.
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
list, idempotency, backups, `GOG_DO_NOT_CREATE_BACKUPS`,
`GOG_IGNORE_FILES_REGEX`, directory and symlink conflicts, paths outside
`$HOME`, and the multi-repository overlay.

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

## What is left

### Capability 4 - `gog remove` and `gog git`

Not started. This is the next capability. Cover at least:

- `gog remove` on a file, on a directory, on a path that was never added, on a
  path added to a different repository, and on a path whose symlink the user
  has already replaced by hand.
- Whether `remove` restores the file's contents at its original location, and
  what happens to the `.gog` backup that may still be sitting beside it.
- `RemovePaths` mutates as it goes, exactly as `AddPaths` used to. Decide with
  the owner whether it should validate the batch up front for symmetry.
- `gog git` passthrough: `status`, `commit`, `push`, `log`, a command that
  fails, a command that needs stdin, flags that collide with gog's own (`-r`),
  and `--` handling.
- `resolveGitPaths` in `cmd/cmd.go` rewrites symlinked arguments to
  repository-relative paths. Exercise it with a path inside the repository, one
  outside it, a relative path, a flag-looking argument, and a nonexistent path.
- Whether `gog git` propagates git's exit code.

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
2. **`RemovePaths` does not validate its batch up front**, unlike `AddPaths`.
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
  unset XDG_DATA_HOME GOG_HOME GOG_DEFAULT_REPOSITORY_NAME \
        GOG_DO_NOT_CREATE_BACKUPS GOG_IGNORE_FILES_REGEX
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
