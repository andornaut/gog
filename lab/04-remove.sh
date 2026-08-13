#!/usr/bin/env bash
# gog remove
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

echo "=== 4.1 remove a file: restores contents, drops the repository copy ==="
new_sandbox remove-file
gogq repository add dots >/dev/null
REPO="${HOME}/.local/share/gog/dots"
printf 'original\n' >"${HOME}/.bashrc"
gogq add ~/.bashrc >/dev/null
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
gog remove ~/.bashrc; rc=$?
assert_rc 0 "${rc}" "remove a linked file"
assert_regular "${HOME}/.bashrc"
assert_file_contents "${HOME}/.bashrc" "original"
assert_not_exists "${REPO}/\$HOME/.bashrc"
out="$(cd "${REPO}" && git status --porcelain)"
assert_not_contains "${out}" '.bashrc' "removal is staged, nothing left in git status"
echo "--- repository tree after remove:"
find "${REPO}" -not -path "${REPO}/.git/*" -not -name .git

echo
echo "=== 4.2 remove reports what it restored ==="
new_sandbox remove-output
gogq repository add dots >/dev/null
printf 'original\n' >"${HOME}/.bashrc"
gogq add ~/.bashrc >/dev/null
out="$(gogq remove ~/.bashrc 2>&1)"
echo "${out}"
assert_contains "${out}" "Restored" "remove reports the restored path"

echo
echo "=== 4.3 remove a directory ==="
new_sandbox remove-dir
gogq repository add dots >/dev/null
REPO="${HOME}/.local/share/gog/dots"
mkdir -p "${HOME}/.config/app/nested"
printf 'a\n' >"${HOME}/.config/app/a.conf"
printf 'b\n' >"${HOME}/.config/app/nested/b.conf"
gogq add ~/.config >/dev/null
assert_symlink_to "${HOME}/.config/app/nested/b.conf" "${REPO}/\$HOME/.config/app/nested/b.conf"
gog remove ~/.config; rc=$?
assert_rc 0 "${rc}" "remove a linked directory"
assert_regular "${HOME}/.config/app/a.conf"
assert_regular "${HOME}/.config/app/nested/b.conf"
assert_file_contents "${HOME}/.config/app/nested/b.conf" "b"
assert_dir "${HOME}/.config/app/nested"
assert_not_exists "${REPO}/\$HOME/.config"
echo "--- repository tree after remove:"
find "${REPO}" -not -path "${REPO}/.git/*" -not -name .git
out="$(cd "${REPO}" && git status --porcelain)"
echo "--- git status: '${out}'"
assert_contains "${out}" "" "git status recorded"

echo
echo "=== 4.4 remove one file out of an added directory: leftovers in the repository ==="
new_sandbox remove-nested
gogq repository add dots >/dev/null
REPO="${HOME}/.local/share/gog/dots"
mkdir -p "${HOME}/.config/app/nested"
printf 'a\n' >"${HOME}/.config/app/a.conf"
printf 'b\n' >"${HOME}/.config/app/nested/b.conf"
gogq add ~/.config >/dev/null
gog remove ~/.config/app/nested/b.conf; rc=$?
assert_rc 0 "${rc}" "remove a nested file"
assert_regular "${HOME}/.config/app/nested/b.conf"
assert_symlink_to "${HOME}/.config/app/a.conf" "${REPO}/\$HOME/.config/app/a.conf"
echo "--- repository tree after remove:"
find "${REPO}" -not -path "${REPO}/.git/*" -not -name .git
out="$(cd "${REPO}" && git status --porcelain)"
echo "--- git status: '${out}'"
assert_not_exists "${REPO}/\$HOME/.config/app/nested"

echo
echo "=== 4.5 remove a path that was never added ==="
new_sandbox remove-unknown
gogq repository add dots >/dev/null
printf 'mine\n' >"${HOME}/.vimrc"
out="$(gogq remove ~/.vimrc 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "remove a path that was never added"
assert_contains "${out}" "Not tracked by dots" "the repository says it never held the path"
assert_regular "${HOME}/.vimrc"
assert_file_contents "${HOME}/.vimrc" "mine"

echo
echo "=== 4.6 remove a path that does not exist at all ==="
new_sandbox remove-missing
gogq repository add dots >/dev/null
out="$(gogq remove ~/.nothing-here 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "remove a nonexistent path"
assert_contains "${out}" "Not tracked by dots" "a nonexistent path is reported the same way"

echo
echo "=== 4.7 remove a path that belongs to a different repository ==="
new_sandbox remove-other-repo
gogq repository add one >/dev/null
gogq repository add two >/dev/null
ONE="${HOME}/.local/share/gog/one"
printf 'original\n' >"${HOME}/.bashrc"
gogq -r one add ~/.bashrc >/dev/null
out="$(gogq -r two remove ~/.bashrc 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "remove from a repository that does not hold the path"
assert_contains "${out}" "Not tracked by two" "the repository that was asked names itself"
assert_symlink_to "${HOME}/.bashrc" "${ONE}/\$HOME/.bashrc"
assert_exists "${ONE}/\$HOME/.bashrc"

echo
echo "=== 4.8 remove after the user replaced the symlink with their own file ==="
new_sandbox remove-replaced
gogq repository add dots >/dev/null
REPO="${HOME}/.local/share/gog/dots"
printf 'original\n' >"${HOME}/.bashrc"
gogq add ~/.bashrc >/dev/null
rm "${HOME}/.bashrc"; printf 'mine now\n' >"${HOME}/.bashrc"
gog remove ~/.bashrc; rc=$?
assert_rc 0 "${rc}" "remove a path the user replaced by hand"
assert_regular "${HOME}/.bashrc"
assert_file_contents "${HOME}/.bashrc" "mine now"
assert_not_exists "${REPO}/\$HOME/.bashrc"
out="$(cd "${REPO}" && git status --porcelain)"
assert_not_contains "${out}" '??' "no untracked leftovers"

echo
echo "=== 4.9 remove after the user deleted the symlink ==="
new_sandbox remove-deleted
gogq repository add dots >/dev/null
REPO="${HOME}/.local/share/gog/dots"
printf 'original\n' >"${HOME}/.bashrc"
gogq add ~/.bashrc >/dev/null
rm "${HOME}/.bashrc"
gog remove ~/.bashrc; rc=$?
assert_rc 0 "${rc}" "remove a path whose link the user deleted"
assert_not_exists "${HOME}/.bashrc"
assert_not_exists "${REPO}/\$HOME/.bashrc"
out="$(cd "${REPO}" && git -c core.pager=cat log --oneline 2>&1)"
echo "--- log: ${out}"

echo
echo "=== 4.10 a .gog file left by an older version is not gog's business ==="
new_sandbox remove-backup
gogq repository add dots >/dev/null
REPO="${HOME}/.local/share/gog/dots"
printf 'from repo\n' >"${HOME}/.bashrc"
gogq add ~/.bashrc >/dev/null
gogq git commit -q -m init >/dev/null 2>&1
# What an older version would have left behind when it displaced the user's file
printf 'pre-gog\n' >"${HOME}/.bashrc.gog"
gog remove ~/.bashrc; rc=$?
assert_rc 0 "${rc}" "remove a path with an old .gog file beside it"
assert_file_contents "${HOME}/.bashrc" "from repo"
assert_file_contents "${HOME}/.bashrc.gog" "pre-gog"
echo "--- home after remove:"
find "${HOME}" -maxdepth 1 -name '.bashrc*'

echo
echo "=== 4.11 guard rails ==="
new_sandbox remove-guards
gogq repository add dots >/dev/null
REPO="${HOME}/.local/share/gog/dots"
printf 'x\n' >"${HOME}/.x"
gogq add ~/.x >/dev/null
out="$(gogq remove "${HOME}/.x.gog" 2>&1)"; rc=$?
echo "${out}"
assert_rc 1 "${rc}" "remove refuses a .gog path"
assert_contains "${out}" "backup files cannot be managed" "explains why a .gog path is refused"
out="$(gogq remove "${REPO}/\$HOME/.x" 2>&1)"; rc=$?
echo "${out}"
assert_rc 1 "${rc}" "remove refuses a path inside gog's data directory"
assert_contains "${out}" "gog's own directory" "explains why a data-directory path is refused"
assert_symlink_to "${HOME}/.x" "${REPO}/\$HOME/.x"

echo
echo "=== 4.12 a bad path fails the batch before anything is restored ==="
new_sandbox remove-batch
gogq repository add dots >/dev/null
REPO="${HOME}/.local/share/gog/dots"
printf 'a\n' >"${HOME}/.a"; printf 'b\n' >"${HOME}/.b"
gogq add ~/.a ~/.b >/dev/null
out="$(gogq remove ~/.a "${HOME}/.b.gog" 2>&1)"; rc=$?
echo "${out}"
assert_rc 1 "${rc}" "batch with an invalid path fails"
assert_symlink_to "${HOME}/.a" "${REPO}/\$HOME/.a"
assert_symlink_to "${HOME}/.b" "${REPO}/\$HOME/.b"

echo
echo "=== 4.13 multiple paths, relative paths, and -r in both positions ==="
new_sandbox remove-args
gogq repository add dots >/dev/null
REPO="${HOME}/.local/share/gog/dots"
printf 'a\n' >"${HOME}/.a"; printf 'b\n' >"${HOME}/.b"; printf 'c\n' >"${HOME}/.c"
gogq add ~/.a ~/.b ~/.c >/dev/null
gog remove .a ./.b; rc=$?
assert_rc 0 "${rc}" "remove relative paths"
assert_regular "${HOME}/.a"; assert_regular "${HOME}/.b"
assert_symlink_to "${HOME}/.c" "${REPO}/\$HOME/.c"
gog -r dots remove ~/.c; rc=$?
assert_rc 0 "${rc}" "gog -r NAME remove"
assert_regular "${HOME}/.c"
printf 'd\n' >"${HOME}/.d"; gogq add ~/.d >/dev/null
gog remove -r dots ~/.d; rc=$?
assert_rc 0 "${rc}" "gog remove -r NAME"
assert_regular "${HOME}/.d"

echo
echo "=== 4.14 remove with no arguments, and remove against an unknown repository ==="
new_sandbox remove-usage
gogq repository add dots >/dev/null
out="$(gogq remove 2>&1)"; rc=$?
echo "${out}"
assert_rc 1 "${rc}" "remove with no arguments fails"
out="$(gogq -r nope remove ~/.bashrc 2>&1)"; rc=$?
echo "${out}"
assert_rc 1 "${rc}" "remove against an unknown repository fails"
assert_contains "${out}" "repository not found" "names the missing repository"

echo
echo "=== 4.15 remove a path outside \$HOME ==="
new_sandbox remove-outside
gogq repository add dots >/dev/null
REPO="${HOME}/.local/share/gog/dots"
mkdir -p "${SANDBOX}/etc"
printf 'sys\n' >"${SANDBOX}/etc/thing.conf"
gogq add "${SANDBOX}/etc/thing.conf" >/dev/null
assert_symlink_to "${SANDBOX}/etc/thing.conf" "${REPO}${SANDBOX}/etc/thing.conf"
gog remove "${SANDBOX}/etc/thing.conf"; rc=$?
assert_rc 0 "${rc}" "remove a path outside \$HOME"
assert_regular "${SANDBOX}/etc/thing.conf"
assert_file_contents "${SANDBOX}/etc/thing.conf" "sys"

echo
echo "=== 4.16 remove is idempotent ==="
new_sandbox remove-twice
gogq repository add dots >/dev/null
printf 'original\n' >"${HOME}/.bashrc"
gogq add ~/.bashrc >/dev/null
gogq remove ~/.bashrc >/dev/null
out="$(gogq remove ~/.bashrc 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "second remove of the same path"
assert_contains "${out}" "Not tracked by dots" "the second removal says the path is no longer held"
assert_regular "${HOME}/.bashrc"
assert_file_contents "${HOME}/.bashrc" "original"

echo
echo "=== 4.17 removed file's mode survives the round trip ==="
new_sandbox remove-mode
gogq repository add dots >/dev/null
printf '#!/bin/sh\n' >"${HOME}/.script"; chmod 0755 "${HOME}/.script"
printf 'secret\n' >"${HOME}/.netrc"; chmod 0600 "${HOME}/.netrc"
gogq add ~/.script ~/.netrc >/dev/null 2>&1
gogq remove ~/.script ~/.netrc >/dev/null
mode_script="$(stat -c %a "${HOME}/.script")"
mode_netrc="$(stat -c %a "${HOME}/.netrc")"
echo "--- .script ${mode_script}, .netrc ${mode_netrc}"
[ "${mode_script}" = "755" ] && _ok "executable bit survives remove" || _bad "executable bit survives remove (got ${mode_script})"
[ "${mode_netrc}" = "600" ] && _ok ".netrc stays 0600 after remove" || _bad ".netrc stays 0600 after remove (got ${mode_netrc})"

summary
