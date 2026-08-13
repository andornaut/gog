#!/usr/bin/env bash
# concurrency, scale and interruption
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# make_tree DIR FILES: a nested tree holding the given number of files
make_tree() {
  local root="$1" count="$2" i dir
  for ((i = 0; i < count; i++)); do
    dir="${root}/d$((i / 100))/d$((i / 10))"
    mkdir -p "${dir}"
    printf 'file %d\n' "${i}" >"${dir}/f${i}.conf"
  done
}

count_files() { find "$1" -type f 2>/dev/null | wc -l; }
count_links() { find "$1" -type l 2>/dev/null | wc -l; }

# run_interruptible ARGS...: start gog in the background with SIGINT restored to
# its default disposition. setsid is deliberately not used: it forks when the
# caller is a process group leader, which leaves $! naming the wrapper rather
# than gog, and the signal then reaches nothing. A background job of a non-interactive shell inherits
# SIG_IGN for SIGINT, and Go keeps a signal that was already ignored, so without
# this the process runs to completion however often it is signalled.
run_interruptible() {
  # exec, so that the backgrounded subshell running this function becomes the
  # process itself and $! names gog rather than the subshell
  exec python3 -c \
    'import signal,os,sys; signal.signal(signal.SIGINT, signal.SIG_DFL); os.execv(sys.argv[1], sys.argv[1:])' \
    "${GOG_BIN}" "$@"
}

echo "=== 8.1 two 'gog add' processes against one repository ==="
new_sandbox conc-add
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
for i in 1 2 3 4 5 6; do printf 'a%d\n' "${i}" >"${HOME}/.a${i}"; done
gogq add ~/.a1 ~/.a2 ~/.a3 >"${SANDBOX}/one.log" 2>&1 &
p1=$!
gogq add ~/.a4 ~/.a5 ~/.a6 >"${SANDBOX}/two.log" 2>&1 &
p2=$!
wait ${p1}; rc1=$?
wait ${p2}; rc2=$?
echo "--- exit codes: ${rc1} ${rc2}"
grep -h 'ERROR\|error\|fatal' "${SANDBOX}"/one.log "${SANDBOX}"/two.log | head -4
echo "--- git status after the race:"
(cd "${REPO}" && git status --porcelain)
echo "--- rerunning both adds sequentially:"
gogq add ~/.a1 ~/.a2 ~/.a3 >/dev/null 2>&1; r1=$?
gogq add ~/.a4 ~/.a5 ~/.a6 >/dev/null 2>&1; r2=$?
assert_rc 0 "${r1}" "the first add converges on a rerun"
assert_rc 0 "${r2}" "the second add converges on a rerun"
for i in 1 2 3 4 5 6; do assert_symlink_to "${HOME}/.a${i}" "${REPO}/\$HOME/.a${i}"; done
out="$(cd "${REPO}" && git status --porcelain)"
echo "--- git status now: '${out}'"
assert_not_contains "${out}" "??" "every file is staged, none left untracked"

echo
echo "=== 8.2 two 'gog apply' processes against one repository ==="
new_sandbox conc-apply
REMOTE="$(make_remote origin)"
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
make_tree "${HOME}/.config" 200
gogq add ~/.config >/dev/null 2>&1
gogq git commit -q -m init >/dev/null 2>&1
rm -rf "${HOME}/.config"
gogq apply >"${SANDBOX}/a1.log" 2>&1 &
p1=$!
gogq apply >"${SANDBOX}/a2.log" 2>&1 &
p2=$!
wait ${p1}; rc1=$?
wait ${p2}; rc2=$?
echo "--- exit codes: ${rc1} ${rc2}"
grep -h 'ERROR\|fatal' "${SANDBOX}"/a1.log "${SANDBOX}"/a2.log | head -4
links="$(count_links "${HOME}/.config")"
echo "--- ${links} links created (want 200)"
[ "${links}" = "200" ] && _ok "concurrent applies link everything" || _bad "concurrent applies link everything (got ${links})"
out="$(gogq apply 2>&1)"; rc=$?
assert_rc 0 "${rc}" "a third apply is clean"

echo
echo "=== 8.3 a large tree ==="
new_sandbox scale
REMOTE="$(make_remote origin)"
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
make_tree "${HOME}/.config" 3000
echo "--- ${is}$(count_files "${HOME}/.config") files to add"
start=${SECONDS}
gogq add ~/.config >"${SANDBOX}/add.log" 2>&1; rc=$?
echo "--- gog add took $((SECONDS - start))s, exit ${rc}"
assert_rc 0 "${rc}" "add a 3000-file tree"
[ "$(count_files "${REPO}/\$HOME/.config")" = "3000" ] && _ok "3000 files in the repository" \
  || _bad "3000 files in the repository (got $(count_files "${REPO}/\$HOME/.config"))"
[ "$(count_links "${HOME}/.config")" = "3000" ] && _ok "3000 links in \$HOME" \
  || _bad "3000 links in \$HOME (got $(count_links "${HOME}/.config"))"
out="$(cd "${REPO}" && git status --porcelain | grep -c '^A ')"
echo "--- ${out} paths staged"
[ "${out}" = "3000" ] && _ok "3000 paths staged in one run" || _bad "3000 paths staged in one run (got ${out})"
gogq git commit -q -m big >/dev/null 2>&1
gogq git remote add origin "${REMOTE}" >/dev/null 2>&1
gogq git push -q -u origin main >/dev/null 2>&1

echo "--- applying the same tree to a second home"
rm -rf "${HOME}/.config"
start=${SECONDS}
gogq apply >"${SANDBOX}/apply.log" 2>&1; rc=$?
echo "--- gog apply took $((SECONDS - start))s, exit ${rc}"
assert_rc 0 "${rc}" "apply a 3000-file tree"
[ "$(count_links "${HOME}/.config")" = "3000" ] && _ok "3000 links after apply" \
  || _bad "3000 links after apply (got $(count_links "${HOME}/.config"))"

echo "--- removing the repository restores all 3000"
start=${SECONDS}
gogq repository remove dots >"${SANDBOX}/rm.log" 2>&1; rc=$?
echo "--- repository remove took $((SECONDS - start))s, exit ${rc}"
assert_rc 0 "${rc}" "remove a repository holding 3000 files"
[ "$(count_links "${HOME}/.config")" = "0" ] && _ok "no dangling links left" \
  || _bad "no dangling links left (got $(count_links "${HOME}/.config"))"
[ "$(count_files "${HOME}/.config")" = "3000" ] && _ok "3000 real files restored" \
  || _bad "3000 real files restored (got $(count_files "${HOME}/.config"))"

echo
echo "=== 8.4 interrupting an apply, then rerunning it ==="
new_sandbox interrupt
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
make_tree "${HOME}/.config" 4000
gogq add ~/.config >/dev/null 2>&1
gogq git commit -q -m init >/dev/null 2>&1
rm -rf "${HOME}/.config"
run_interruptible apply >"${SANDBOX}/int.log" 2>&1 &
pid=$!
# Interrupt the whole group as a terminal would, once the walk is part-way
# through. filepath.Walk is lexical, so d3 of d0..d39 lands around 60%.
until [ -e "${HOME}/.config/d3" ] || ! kill -0 "${pid}" 2>/dev/null; do :; done
kill -INT "${pid}" 2>/dev/null
wait ${pid} 2>/dev/null; rc=$?
partial="$(count_links "${HOME}/.config")"
echo "--- interrupted after ${partial} links, exit ${rc} (130 is SIGINT)"
assert_rc 130 "${rc}" "the run died of the signal"
[ "${partial}" -lt 4000 ] && _ok "the run was genuinely interrupted" || _bad "the run was genuinely interrupted"
echo "--- leftovers in the repository:"
ls "${REPO}/.git/index.lock" 2>&1 | sed "s|${HOME}|~|"
out="$(gogq apply 2>&1)"; rc=$?
echo "--- rerun exit ${rc}"
printf '%s\n' "${out}" | grep -i 'error\|fatal' | head -3
assert_rc 0 "${rc}" "apply converges after an interrupted run"
[ "$(count_links "${HOME}/.config")" = "4000" ] && _ok "all 4000 links present after the rerun" \
  || _bad "all 4000 links present after the rerun (got $(count_links "${HOME}/.config"))"

echo
echo "=== 8.5 a run killed mid-stage ==="
new_sandbox killed
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
make_tree "${HOME}/.config" 4000
gogq add ~/.config >/dev/null 2>&1
gogq git commit -q -m init >/dev/null 2>&1
rm -rf "${HOME}/.config"
run_interruptible apply >"${SANDBOX}/kill.log" 2>&1 &
pid=$!
until [ -e "${HOME}/.config/d3" ] || ! kill -0 "${pid}" 2>/dev/null; do :; done
kill -KILL "${pid}" 2>/dev/null
wait ${pid} 2>/dev/null; rc=$?
echo "--- killed after $(count_links "${HOME}/.config") links, exit ${rc} (137 is SIGKILL)"
echo "--- index.lock left behind: $([ -e "${REPO}/.git/index.lock" ] && echo yes || echo no)"
echo "    (a git child outlives gog, so whether the lock is still held is a race)"
# Let any orphaned git finish before the repository is inspected
until [ ! -e "${REPO}/.git/index.lock" ]; do :; done
out="$(gogq apply 2>&1)"; rc=$?
echo "--- rerun exit ${rc}"
assert_rc 0 "${rc}" "apply converges after a killed run"
[ "$(count_links "${HOME}/.config")" = "4000" ] && _ok "all 4000 links present after the rerun" \
  || _bad "all 4000 links present after the rerun (got $(count_links "${HOME}/.config"))"

echo
echo "=== 8.5b a locked index, with nothing else running ==="
new_sandbox locked
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
: >"${REPO}/.git/index.lock"
printf 'new\n' >"${HOME}/.newfile"
out="$(gogq add ~/.newfile 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "add reports a failure when the index is locked"
assert_contains "${out}" "remove the file manually" "git explains the remedy"
assert_exists "${REPO}/\$HOME/.newfile"
assert_symlink_to "${HOME}/.newfile" "${REPO}/\$HOME/.newfile"
rm -f "${REPO}/.git/index.lock"
out="$(gogq add ~/.newfile 2>&1)"; rc=$?
assert_rc 0 "${rc}" "the same add succeeds once the lock is cleared"
out="$(cd "${REPO}" && git status --porcelain)"
echo "--- status of the recovered file: '${out}'"
assert_contains "${out}" "A " "the file is staged after the rerun"

echo
echo "=== 8.5c interrupting an add, whose copy and staging are separate phases ==="
new_sandbox interrupt-add
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
make_tree "${HOME}/.config" 4000
run_interruptible add ~/.config >"${SANDBOX}/add-int.log" 2>&1 &
pid=$!
until [ -e "${REPO}/\$HOME/.config/d3" ] || ! kill -0 "${pid}" 2>/dev/null; do :; done
kill -INT "${pid}" 2>/dev/null
wait ${pid} 2>/dev/null; rc=$?
copied="$(count_files "${REPO}/\$HOME/.config")"
untracked="$(cd "${REPO}" && git status --porcelain | grep -c '^??')"
echo "--- interrupted after ${copied} files were copied, exit ${rc}"
echo "--- $(count_links "${HOME}/.config") links, ${untracked} untracked entries in the repository"
[ "${copied}" -lt 4000 ] && _ok "the add was genuinely interrupted" || _bad "the add was genuinely interrupted"
out="$(gogq add ~/.config 2>&1)"; rc=$?
assert_rc 0 "${rc}" "add converges on a rerun"
[ "$(count_files "${REPO}/\$HOME/.config")" = "4000" ] && _ok "all 4000 files copied after the rerun" \
  || _bad "all 4000 files copied after the rerun (got $(count_files "${REPO}/\$HOME/.config"))"
[ "$(count_links "${HOME}/.config")" = "4000" ] && _ok "all 4000 links present after the rerun" \
  || _bad "all 4000 links present after the rerun (got $(count_links "${HOME}/.config"))"
untracked="$(cd "${REPO}" && git status --porcelain | grep -c '^??')"
[ "${untracked}" = "0" ] && _ok "nothing left untracked" || _bad "nothing left untracked (got ${untracked})"

echo
echo "=== 8.6 a repository on a different filesystem from \$HOME ==="
new_sandbox crossfs
XFS_ROOT="/dev/shm/gog-lab-$$"
rm -rf "${XFS_ROOT}"; mkdir -p "${XFS_ROOT}"
export GOG_HOME="${XFS_ROOT}/gog"
echo "--- \$HOME is on $(df -T "${HOME}" | awk 'NR==2 {print $2}'), the repository on $(df -T /dev/shm | awk 'NR==2 {print $2}')"
gogq repository add dots >/dev/null 2>&1
REPO="${GOG_HOME}/dots"
printf 'bashrc\n' >"${HOME}/.bashrc"
mkdir -p "${HOME}/.config/app"
printf 'a\n' >"${HOME}/.config/app/a.conf"
out="$(gogq add ~/.bashrc ~/.config 2>&1)"; rc=$?
assert_rc 0 "${rc}" "add across filesystems"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
assert_file_contents "${REPO}/\$HOME/.config/app/a.conf" "a"
gogq git commit -q -m init >/dev/null 2>&1
rm "${HOME}/.bashrc"
out="$(gogq apply 2>&1)"; rc=$?
assert_rc 0 "${rc}" "apply across filesystems"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
out="$(gogq remove ~/.bashrc 2>&1)"; rc=$?
assert_rc 0 "${rc}" "remove across filesystems"
assert_regular "${HOME}/.bashrc"
assert_file_contents "${HOME}/.bashrc" "bashrc"
out="$(gogq repository remove dots -f 2>&1)"; rc=$?
assert_rc 0 "${rc}" "repository remove across filesystems"
assert_regular "${HOME}/.config/app/a.conf"
assert_file_contents "${HOME}/.config/app/a.conf" "a"
unset GOG_HOME
rm -rf "${XFS_ROOT}"

summary
