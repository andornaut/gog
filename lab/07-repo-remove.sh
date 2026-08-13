#!/usr/bin/env bash
# gog repository remove against live links
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# Builds a repository holding one file and one directory tree, all linked, and
# pushed to a bare remote so that removal is not refused
build_pushed_repo() {
  new_sandbox "$1"
  REMOTE="$(make_remote origin)"
  gogq repository add dots >/dev/null 2>&1
  REPO="${HOME}/.local/share/gog/dots"
  printf 'bashrc\n' >"${HOME}/.bashrc"
  mkdir -p "${HOME}/.config/app/nested"
  printf 'a\n' >"${HOME}/.config/app/a.conf"
  printf 'b\n' >"${HOME}/.config/app/nested/b.conf"
  gogq add ~/.bashrc ~/.config >/dev/null 2>&1
  gogq git commit -q -m init >/dev/null 2>&1
  gogq git remote add origin "${REMOTE}" >/dev/null 2>&1
  gogq git push -q -u origin main >/dev/null 2>&1
}

count_links() {
  find "${HOME}" -path "${HOME}/.local" -prune -o -type l -print | wc -l
}

echo "=== 7.1 removal restores every file it had linked ==="
build_pushed_repo repo-remove-live
before="$(count_links)"
echo "--- ${before} links into the repository before removal"
gog repository remove dots; rc=$?
assert_rc 0 "${rc}" "repository remove with live links"
assert_not_exists "${REPO}"
after="$(count_links)"
echo "--- ${after} links left in \$HOME"
[ "${after}" = "0" ] && _ok "no link is left dangling" || _bad "no link is left dangling (got ${after})"
assert_regular "${HOME}/.bashrc"
assert_file_contents "${HOME}/.bashrc" "bashrc"
assert_regular "${HOME}/.config/app/a.conf"
assert_file_contents "${HOME}/.config/app/nested/b.conf" "b"
assert_dir "${HOME}/.config/app/nested"

echo
echo "=== 7.2 removal reports what it restored ==="
build_pushed_repo repo-remove-report
out="$(gogq repository remove dots 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "removal of a pushed repository"
assert_contains "${out}" "Restored" "the output names what was restored"
assert_contains "${out}" "Removed repository" "the output confirms the removal"

echo
echo "=== 7.3 unpushed commits are refused ==="
new_sandbox repo-remove-pending
REMOTE="$(make_remote origin)"
gogq repository add dots "${REMOTE}" >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
printf 'one\n' >"${HOME}/.bashrc"
gogq add ~/.bashrc >/dev/null 2>&1
gogq git commit -q -m pushed >/dev/null 2>&1
gogq git push -q -u origin main >/dev/null 2>&1
printf 'two\n' >"${HOME}/.vimrc"
gogq add ~/.vimrc >/dev/null 2>&1
gogq git commit -q -m unpushed >/dev/null 2>&1
out="$(gogq repository remove dots 2>&1)"; rc=$?
echo "${out}"
assert_rc 1 "${rc}" "removal is refused while a commit is unpushed"
assert_contains "${out}" "1 commit that no remote has" "the refusal counts the commits"
assert_contains "${out}" "--force" "the refusal names the way out"
assert_exists "${REPO}"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"

echo
echo "=== 7.4 uncommitted changes are refused, and counted together ==="
printf 'work in progress\n' >"${REPO}/\$HOME/.bashrc"
printf 'junk\n' >"${REPO}/untracked"
out="$(gogq repository remove dots 2>&1)"; rc=$?
echo "${out}"
assert_rc 1 "${rc}" "removal is refused while the work tree is dirty"
assert_contains "${out}" "1 commit that no remote has and 2 uncommitted changes" "both kinds are counted"

echo
echo "=== 7.5 --force deletes anyway, and still restores ==="
out="$(gogq repository remove dots --force 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "--force removes a repository holding unsaved work"
assert_not_exists "${REPO}"
assert_regular "${HOME}/.bashrc"
assert_file_contents "${HOME}/.bashrc" "work in progress"
assert_regular "${HOME}/.vimrc"
assert_file_contents "${HOME}/.vimrc" "two"

echo
echo "=== 7.6 a repository with no remote holds everything only once ==="
new_sandbox repo-remove-noremote
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
printf 'x\n' >"${HOME}/.x"
gogq add ~/.x >/dev/null 2>&1
gogq git commit -q -m only >/dev/null 2>&1
out="$(gogq repository remove dots 2>&1)"; rc=$?
echo "${out}"
assert_rc 1 "${rc}" "a repository with no remote is refused"
assert_contains "${out}" "1 commit that no remote has" "its whole history is what would be lost"
out="$(gogq repository remove dots -f 2>&1)"; rc=$?
assert_rc 0 "${rc}" "-f is the short form of --force"
assert_regular "${HOME}/.x"

echo
echo "=== 7.7 an empty repository is removed without complaint ==="
new_sandbox repo-remove-empty
gogq repository add dots >/dev/null 2>&1
out="$(gogq repository remove dots 2>&1)"; rc=$?
echo "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "an empty repository has nothing to lose"

echo
echo "=== 7.8 a prefix no longer selects a repository to delete ==="
new_sandbox repo-remove-prefix
gogq repository add dotfiles >/dev/null 2>&1
out="$(gogq repository remove d 2>&1)"; rc=$?
echo "${out}"
assert_rc 1 "${rc}" "a prefix is refused"
assert_contains "${out}" "repository not found: d" "the refusal names what was not found"
assert_exists "${HOME}/.local/share/gog/dotfiles"
out="$(gogq repository remove dotfiles 2>&1)"; rc=$?
assert_rc 0 "${rc}" "the exact name works"
assert_not_exists "${HOME}/.local/share/gog/dotfiles"

echo
echo "=== 7.9 the default repository cannot be deleted by omission ==="
new_sandbox repo-remove-omission
gogq repository add dotfiles >/dev/null 2>&1
out="$(gogq repository remove 2>&1)"; rc=$?
echo "${out}" | head -2
assert_rc 1 "${rc}" "remove with no argument fails"
out="$(gogq repository remove '' 2>&1)"; rc=$?
echo "${out}" | head -2
assert_rc 1 "${rc}" "remove with an empty name fails"
assert_exists "${HOME}/.local/share/gog/dotfiles"

echo
echo "=== 7.10 removal does not create files that were never applied ==="
new_sandbox repo-remove-unapplied
REMOTE="$(make_remote origin)"
gogq repository add source >/dev/null 2>&1
printf 'x\n' >"${HOME}/.x"
gogq add -r source ~/.x >/dev/null 2>&1
gogq git -r source commit -q -m init >/dev/null 2>&1
gogq git -r source remote add origin "${REMOTE}" >/dev/null 2>&1
gogq git -r source push -q -u origin main >/dev/null 2>&1
# A second repository holding the same file, cloned but never applied
gogq repository add clone "${REMOTE}" >/dev/null 2>&1
out="$(gogq repository remove clone 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "removing a repository that was never applied"
assert_not_contains "${out}" "Restored" "nothing was restored"
assert_symlink_to "${HOME}/.x" "${HOME}/.local/share/gog/source/\$HOME/.x"

echo
echo "=== 7.11 removing one repository of an overlay ==="
new_sandbox repo-remove-overlay
REMOTE_ONE="$(make_remote one)"; REMOTE_TWO="$(make_remote two)"
gogq repository add one >/dev/null 2>&1
gogq repository add two >/dev/null 2>&1
ONE="${HOME}/.local/share/gog/one"; TWO="${HOME}/.local/share/gog/two"
printf 'a\n' >"${HOME}/.a"
gogq add -r one ~/.a >/dev/null 2>&1
gogq git -r one commit -q -m one >/dev/null 2>&1
gogq git -r one remote add origin "${REMOTE_ONE}" >/dev/null 2>&1
gogq git -r one push -q -u origin main >/dev/null 2>&1
printf 'b\n' >"${HOME}/.b"
gogq add -r two ~/.b >/dev/null 2>&1
out="$(gogq repository remove one 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "removing one repository of an overlay"
assert_regular "${HOME}/.a"
assert_file_contents "${HOME}/.a" "a"
assert_symlink_to "${HOME}/.b" "${TWO}/\$HOME/.b"
out="$(gogq repository list 2>&1)"
echo "--- repository list: ${out}"
assert_not_contains "${out}" "one" "the removed repository is gone from the list"

echo
echo "=== 7.12 a path both repositories track is left to the one that owns the link ==="
new_sandbox repo-remove-shared
REMOTE_ONE="$(make_remote one)"
gogq repository add one >/dev/null 2>&1
gogq repository add two >/dev/null 2>&1
ONE="${HOME}/.local/share/gog/one"; TWO="${HOME}/.local/share/gog/two"
printf 'shared\n' >"${HOME}/.bashrc"
gogq add -r one ~/.bashrc >/dev/null 2>&1
gogq git -r one commit -q -m one >/dev/null 2>&1
gogq git -r one remote add origin "${REMOTE_ONE}" >/dev/null 2>&1
gogq git -r one push -q -u origin main >/dev/null 2>&1
gogq add -r two ~/.bashrc >/dev/null 2>&1
assert_symlink_to "${HOME}/.bashrc" "${TWO}/\$HOME/.bashrc"
out="$(gogq repository remove one 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "removing the repository that does not own the link"
assert_not_contains "${out}" "Restored" "the other repository's link is left alone"
assert_symlink_to "${HOME}/.bashrc" "${TWO}/\$HOME/.bashrc"

echo
echo "=== 7.13 a restore that cannot be written keeps the repository ==="
build_pushed_repo repo-remove-readonly
chmod 500 "${HOME}/.config/app"
out="$(gogq repository remove dots 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
chmod 700 "${HOME}/.config/app"
assert_rc 1 "${rc}" "removal fails when a file cannot be restored"
assert_exists "${REPO}"
assert_exists "${REPO}/\$HOME/.bashrc"

echo
echo "=== 7.14 the other commands afterwards ==="
build_pushed_repo repo-remove-after
gogq repository remove dots >/dev/null 2>&1
out="$(gogq apply 2>&1)"; rc=$?
echo "--- gog apply: ${out} [exit ${rc}]" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "apply after the last repository is removed"
assert_contains "${out}" "no valid git repositories" "apply explains that there is none"
assert_regular "${HOME}/.bashrc"

summary
