#!/usr/bin/env bash
# gog git passthrough and pathspec resolution
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

setup() {
  new_sandbox "$1"
  gogq repository add dots >/dev/null
  REPO="${HOME}/.local/share/gog/dots"
  printf 'original\n' >"${HOME}/.bashrc"
  gogq add ~/.bashrc >/dev/null
}

echo "=== 5.1 status, commit, log ==="
setup git-basic
out="$(gogq git status --porcelain 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "gog git status"
assert_contains "${out}" 'A  $HOME/.bashrc' "status shows the staged file"
out="$(gogq git commit -q -m 'add bashrc' 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "gog git commit"
out="$(gogq git log --oneline 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "gog git log"
assert_contains "${out}" "add bashrc" "log shows the commit"

echo
echo "=== 5.2 the Repository: banner and machine-readable output ==="
out="$(gogq git rev-parse HEAD 2>/dev/null)"
echo "--- stdout of 'gog git rev-parse HEAD':"
printf '%s\n' "${out}" | cat -A | head -4
lines="$(printf '%s\n' "${out}" | wc -l)"
[ "${lines}" = "1" ] && _ok "rev-parse stdout is one line" || _bad "rev-parse stdout is one line (got ${lines})"
err="$(gogq git rev-parse HEAD 2>&1 >/dev/null)"
assert_contains "${err}" "Repository: dots" "the banner is on stderr"
out="$(gogq apply 2>/dev/null)"
assert_not_contains "${out}" "Repository:" "gog apply keeps the banner off stdout too"
out="$(gogq repository list 2>/dev/null)"
assert_contains "${out}" "dots" "repository list still prints to stdout"

echo
echo "=== 5.3 a failing git command propagates git's exit code ==="
setup git-exit
out="$(gogq git rev-parse --verify HEAD 2>&1)"; rc=$?
echo "${out} [exit ${rc}]"
assert_rc 128 "${rc}" "gog git rev-parse on an empty repository exits 128"
assert_not_contains "${out}" "git exited with status" "gog does not restate git's failure"
assert_not_contains "${out}" "Usage:" "gog does not print usage on a git failure"
out="$(gogq git ls-files --error-unmatch nosuchfile 2>&1)"; rc=$?
echo "${out} [exit ${rc}]"
assert_rc 1 "${rc}" "gog git ls-files --error-unmatch exits 1"
out="$(gogq git no-such-subcommand 2>&1)"; rc=$?
echo "${out} [exit ${rc}]"
assert_rc 1 "${rc}" "an unknown git subcommand exits 1"
assert_contains "${out}" "not a git command" "git's own message is shown"

echo
echo "=== 5.4 stdin is passed through ==="
setup git-stdin
out="$(printf 'hello\n' | gogq git hash-object --stdin 2>&1 | tail -1)"; rc=$?
echo "${out}"
assert_contains "${out}" "ce013625030ba8dba906f756967f9e9ca394464a" "hash-object reads stdin"
out="$(printf 'from stdin\n' | gogq git commit -q -F - 2>&1)"; rc=$?
echo "${out} [exit ${rc}]"
assert_rc 0 "${rc}" "commit -F - reads the message from stdin"
out="$(gogq git log --format=%s 2>&1 | tail -1)"
assert_contains "${out}" "from stdin" "the commit message came from stdin"

echo
echo "=== 5.5 -r in both positions, and git's own -r ==="
setup git-flags
gogq git commit -q -m init >/dev/null 2>&1
out="$(gogq -r dots git log --oneline 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "gog -r NAME git log"
assert_contains "${out}" "init" "the named repository was used"
out="$(gogq git ls-tree -r --name-only HEAD 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "git's own -r is passed through"
assert_contains "${out}" '$HOME/.bashrc' "ls-tree -r listed the file"
out="$(gogq git -r dots log --oneline 2>&1)"; rc=$?
echo "${out} [exit ${rc}]"
echo "--- (note: 'gog git -r NAME' is git's -r, not gog's)"

echo
echo "=== 5.6 resolveGitPaths: a symlinked path inside the repository ==="
setup git-resolve
gogq git commit -q -m init >/dev/null 2>&1
out="$(gogq git log --oneline -- ~/.bashrc 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "log limited to a symlinked path"
assert_contains "${out}" "init" "the symlink was rewritten to a repository path"
printf 'changed\n' >"${REPO}/\$HOME/.bashrc"
out="$(gogq git diff --name-only -- ~/.bashrc 2>&1)"
echo "${out}"
assert_contains "${out}" '$HOME/.bashrc' "diff limited to a symlinked path"
out="$(gogq git checkout -- ~/.bashrc 2>&1)"; rc=$?
echo "${out} [exit ${rc}]"
assert_rc 0 "${rc}" "checkout -- <symlinked path>"
assert_file_contents "${HOME}/.bashrc" "original"

echo
echo "=== 5.7 resolveGitPaths: relative paths ==="
setup git-relative
gogq git commit -q -m init >/dev/null 2>&1
cd "${HOME}" || exit 1
out="$(gogq git log --oneline -- .bashrc 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "log with a relative symlinked path"
assert_contains "${out}" "init" "a relative path was resolved"
mkdir -p "${HOME}/sub"; cd "${HOME}/sub" || exit 1
out="$(gogq git log --oneline -- ../.bashrc 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "log with a ../ relative path from a subdirectory"
cd "${HOME}" || exit 1

echo
echo "=== 5.8 resolveGitPaths: a path outside the repository, and a nonexistent path ==="
setup git-outside
gogq git commit -q -m init >/dev/null 2>&1
printf 'mine\n' >"${HOME}/.vimrc"
out="$(gogq git log --oneline -- ~/.vimrc 2>&1)"; rc=$?
echo "${out} [exit ${rc}]"
echo "--- (an unmanaged real file is passed through as an absolute path)"
out="$(gogq git log --oneline -- ~/.nothing 2>&1)"; rc=$?
echo "${out} [exit ${rc}]"
echo "--- (a nonexistent path is passed through verbatim)"

echo
echo "=== 5.9 resolveGitPaths: an argument that is not a path at all ==="
setup git-nonpath
out="$(gogq git commit -q -m .bashrc 2>&1)"; rc=$?
echo "${out} [exit ${rc}]"
msg="$(gogq git log --format=%s 2>/dev/null | tail -1)"
echo "--- recorded commit subject: '${msg}'"
[ "${msg}" = ".bashrc" ] && _ok "a commit message that names an existing file is left alone" \
                         || _bad "a commit message that names an existing file is left alone (got '${msg}')"

echo
echo "=== 5.10 a branch name that collides with a managed path ==="
setup git-branch
gogq git commit -q -m init >/dev/null 2>&1
mkdir -p "${HOME}/wip"
out="$(gogq git checkout -b wip 2>&1)"; rc=$?
echo "${out} [exit ${rc}]"
br="$(gogq git rev-parse --abbrev-ref HEAD 2>/dev/null | tail -1)"
echo "--- current branch: '${br}'"
[ "${br}" = "wip" ] && _ok "a branch name matching a directory is left alone" \
                    || _bad "a branch name matching a directory is left alone (got '${br}')"

echo
echo "=== 5.11 push to a real remote ==="
setup git-push
REMOTE="$(make_remote origin)"
gogq git commit -q -m init >/dev/null 2>&1
out="$(gogq git remote add origin "${REMOTE}" 2>&1)"; rc=$?
assert_rc 0 "${rc}" "gog git remote add"
out="$(gogq git push -q -u origin main 2>&1)"; rc=$?
echo "${out} [exit ${rc}]"
assert_rc 0 "${rc}" "gog git push"
out="$(git --git-dir="${REMOTE}" log --oneline 2>&1)"
assert_contains "${out}" "init" "the commit reached the remote"

echo
echo "=== 5.12 a push that the remote rejects ==="
# Diverge the remote behind gog's back, so the next push is a non-fast-forward
git clone -q "${REMOTE}" "${SANDBOX}/work"
(cd "${SANDBOX}/work" && printf 'theirs\n' >other && git add other \
   && git commit -q -m theirs && git push -q origin main)
printf 'changed\n' >"${REPO}/\$HOME/.bashrc"
gogq git commit -q -a -m second >/dev/null 2>&1
out="$(gogq git push origin main 2>&1)"; rc=$?
echo "${out}" | head -3
assert_rc 1 "${rc}" "a rejected push propagates git's exit code"
assert_contains "${out}" "rejected" "git's rejection message is shown"
out="$(gogq git push nosuchremote main 2>&1)"; rc=$?
assert_rc 128 "${rc}" "a push to an unknown remote exits 128"

echo
echo "=== 5.13 gog git with no arguments ==="
setup git-noargs
out="$(gogq git 2>&1)"; rc=$?
echo "${out}" | head -5
echo "[exit ${rc}]"

echo
echo "=== 5.14 gog git against an unknown repository ==="
out="$(gogq -r nope git status 2>&1)"; rc=$?
echo "${out} [exit ${rc}]"
assert_rc 1 "${rc}" "gog git against an unknown repository exits 1"
assert_contains "${out}" "repository not found" "names the missing repository"

echo
echo "=== 5.15 gog git runs in the repository, not the current directory ==="
setup git-cwd
gogq git commit -q -m init >/dev/null 2>&1
mkdir -p "${HOME}/elsewhere" && cd "${HOME}/elsewhere" || exit 1
out="$(gogq git rev-parse --show-toplevel 2>&1 | tail -1)"
echo "${out}"
assert_contains "${out}" "gog/dots" "git ran inside the repository"
cd "${HOME}" || exit 1

echo
echo "=== 5.16 GIT_* environment variables do not redirect gog's git ==="
setup git-env
gogq git commit -q -m init >/dev/null 2>&1
mkdir -p "${HOME}/decoy" && git init -q "${HOME}/decoy"
out="$(GIT_DIR="${HOME}/decoy/.git" GIT_WORK_TREE="${HOME}/decoy" gogq git rev-parse --show-toplevel 2>&1 | tail -1)"
echo "${out}"
assert_contains "${out}" "gog/dots" "GIT_DIR and GIT_WORK_TREE are scrubbed"
out="$(GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=user.name GIT_CONFIG_VALUE_0=Injected \
       gogq git config user.name 2>&1 | tail -1)"
echo "${out}"
assert_contains "${out}" "Lab Tester" "GIT_CONFIG_* injection is scrubbed"

echo
echo "=== 5.17 arguments that are not pathspecs must not be rewritten ==="
new_sandbox git-rewrite
gogq repository add dots >/dev/null
mkdir -p "${HOME}/bin"
printf 'x\n' >"${HOME}/bin/status"
printf 'x\n' >"${HOME}/bin/wip"
gogq add ~/bin/status ~/bin/wip >/dev/null
cd "${HOME}/bin" || exit 1
out="$(gogq git status --porcelain 2>&1)"; rc=$?
echo "${out}" | head -2
assert_rc 0 "${rc}" "the git subcommand name is not rewritten"
gogq git commit -q -m init >/dev/null 2>&1
out="$(gogq git branch wip 2>&1)"; rc=$?
br="$(gogq git branch --list 2>/dev/null | tail -n +2)"
echo "--- branches: ${br}"
assert_not_contains "${br}" '$HOME' "a branch name is not rewritten"
gogq git config user.name wip >/dev/null 2>&1
name="$(gogq git config user.name 2>/dev/null | tail -1)"
echo "--- user.name: '${name}'"
[ "${name}" = "wip" ] && _ok "a config value is not rewritten" \
                      || _bad "a config value is not rewritten (got '${name}')"
cd "${HOME}" || exit 1

echo
echo "=== 5.19 pathspec subcommands still resolve their operands without -- ==="
setup git-pathspec-subcommands
gogq git commit -q -m init >/dev/null 2>&1
printf 'edited\n' >"${REPO}/\$HOME/.bashrc"
out="$(gogq git add ~/.bashrc 2>&1)"; rc=$?
echo "${out} [exit ${rc}]"
assert_rc 0 "${rc}" "gog git add <symlinked path>"
out="$(gogq git diff --cached --name-only 2>/dev/null)"
assert_contains "${out}" '$HOME/.bashrc' "git add staged the repository's file"
out="$(gogq git check-ignore -v ~/.bashrc 2>&1)"; rc=$?
echo "check-ignore exit ${rc} (1 means not ignored)"
assert_rc 1 "${rc}" "gog git check-ignore <symlinked path>"

echo
echo "=== 5.18 an explicit -- still resolves pathspecs ==="
setup git-dashdash
gogq git commit -q -m init >/dev/null 2>&1
out="$(gogq git log --oneline -- ~/.bashrc 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "log -- <symlinked path>"
assert_contains "${out}" "init" "a pathspec after -- is resolved"

summary
