#!/usr/bin/env bash
# paths already in the way: refused, never backed up
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# Builds a repository holding $HOME/.bashrc with the given contents, committed,
# then clears $HOME of the link so a later apply meets whatever the test leaves
build_repo() {
  new_sandbox "$1"
  gogq repository add dots >/dev/null 2>&1
  REPO="${HOME}/.local/share/gog/dots"
  printf 'from repo\n' >"${HOME}/.bashrc"
  gogq add ~/.bashrc >/dev/null 2>&1
  gogq git commit -q -m init >/dev/null 2>&1
  rm "${HOME}/.bashrc"
}

no_backups_anywhere() {
  local found
  found="$(find "${HOME}" -name '*.gog' 2>/dev/null)"
  [ -z "${found}" ] && _ok "no .gog file was created" || _bad "no .gog file was created (found: ${found})"
}

echo "=== 6.1 apply refuses a file the user already has ==="
build_repo conflict-file
printf 'pre-gog\n' >"${HOME}/.bashrc"
out="$(gogq apply 2>&1)"; rc=$?
echo "${out}"
assert_rc 1 "${rc}" "apply exits non-zero when a path is refused"
assert_contains "${out}" "already exists" "the refusal names the conflict"
assert_contains "${out}" "${HOME}/.bashrc" "the refusal names the path"
assert_regular "${HOME}/.bashrc"
assert_file_contents "${HOME}/.bashrc" "pre-gog"
no_backups_anywhere

echo
echo "=== 6.2 the user clears the conflict and applies again ==="
mv "${HOME}/.bashrc" "${HOME}/bashrc.mine"
out="$(gogq apply 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "apply succeeds once the conflict is cleared"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
assert_file_contents "${HOME}/bashrc.mine" "pre-gog"

echo
echo "=== 6.3 one refusal does not stop the other paths ==="
build_repo conflict-partial
printf 'a\n' >"${HOME}/.a"; printf 'b\n' >"${HOME}/.b"
gogq add ~/.a ~/.b >/dev/null 2>&1
gogq git commit -q -m more >/dev/null 2>&1
rm "${HOME}/.a" "${HOME}/.b"
printf 'mine\n' >"${HOME}/.a"
out="$(gogq apply 2>&1)"; rc=$?
echo "${out}"
assert_rc 1 "${rc}" "apply reports an incomplete run"
assert_file_contents "${HOME}/.a" "mine"
assert_symlink_to "${HOME}/.b" "${REPO}/\$HOME/.b"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
no_backups_anywhere

echo
echo "=== 6.4 apply replaces a link of its own ==="
build_repo conflict-relink
gogq apply >/dev/null 2>&1
out="$(gogq apply 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "apply is idempotent"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
no_backups_anywhere

echo
echo "=== 6.5 the multi-repository overlay still works ==="
new_sandbox conflict-overlay
gogq repository add one >/dev/null 2>&1
gogq repository add two >/dev/null 2>&1
ONE="${HOME}/.local/share/gog/one"; TWO="${HOME}/.local/share/gog/two"
printf 'one\n' >"${HOME}/.bashrc"
gogq -r one add ~/.bashrc >/dev/null 2>&1
gogq -r one git commit -q -m one >/dev/null 2>&1
rm "${HOME}/.bashrc"; printf 'two\n' >"${HOME}/.bashrc"
gogq -r two add ~/.bashrc >/dev/null 2>&1
gogq -r two git commit -q -m two >/dev/null 2>&1
out="$(gogq -r one apply 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "applying the second repository over the first"
assert_symlink_to "${HOME}/.bashrc" "${ONE}/\$HOME/.bashrc"
out="$(gogq -r two apply 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "applying the first repository back over the second"
assert_symlink_to "${HOME}/.bashrc" "${TWO}/\$HOME/.bashrc"
no_backups_anywhere

echo
echo "=== 6.6 a broken link is replaced ==="
build_repo conflict-broken
ln -s "${HOME}/gone" "${HOME}/.bashrc"
out="$(gogq apply 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "apply replaces a broken symbolic link"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
no_backups_anywhere

echo
echo "=== 6.7 a link of the user's own is refused ==="
build_repo conflict-userlink
printf 'elsewhere\n' >"${SANDBOX}/etc/real"
ln -s "${SANDBOX}/etc/real" "${HOME}/.bashrc"
out="$(gogq apply 2>&1)"; rc=$?
echo "${out}"
assert_rc 1 "${rc}" "apply refuses the user's own symbolic link"
assert_symlink_to "${HOME}/.bashrc" "${SANDBOX}/etc/real"
assert_file_contents "${SANDBOX}/etc/real" "elsewhere"
no_backups_anywhere

echo
echo "=== 6.8 a directory conflicting with the user's link is refused ==="
new_sandbox conflict-dir
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
mkdir -p "${HOME}/.config/app"
printf 'a\n' >"${HOME}/.config/app/a.conf"
gogq add ~/.config >/dev/null 2>&1
gogq git commit -q -m init >/dev/null 2>&1
rm -rf "${HOME}/.config"
mkdir -p "${SANDBOX}/etc/theirs"
printf 'theirs\n' >"${SANDBOX}/etc/theirs/keep"
ln -s "${SANDBOX}/etc/theirs" "${HOME}/.config"
out="$(gogq apply 2>&1)"; rc=$?
echo "${out}"
assert_rc 1 "${rc}" "apply refuses to write through the user's directory link"
assert_symlink_to "${HOME}/.config" "${SANDBOX}/etc/theirs"
assert_not_exists "${SANDBOX}/etc/theirs/app"
assert_file_contents "${SANDBOX}/etc/theirs/keep" "theirs"
no_backups_anywhere

echo
echo "=== 6.9 add still links its own copy without complaint ==="
new_sandbox conflict-add
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
printf 'mine\n' >"${HOME}/.bashrc"
mkdir -p "${HOME}/.config/app"
printf 'a\n' >"${HOME}/.config/app/a.conf"
out="$(gogq add ~/.bashrc ~/.config 2>&1)"; rc=$?
echo "${out}"
assert_rc 0 "${rc}" "add links the file it just copied"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
assert_symlink_to "${HOME}/.config/app/a.conf" "${REPO}/\$HOME/.config/app/a.conf"
assert_file_contents "${HOME}/.bashrc" "mine"
no_backups_anywhere
out="$(gogq add ~/.bashrc 2>&1)"; rc=$?
assert_rc 0 "${rc}" "re-adding an already linked path"
no_backups_anywhere

echo
echo "=== 6.10 GOG_DO_NOT_CREATE_BACKUPS no longer does anything ==="
build_repo conflict-envvar
printf 'pre-gog\n' >"${HOME}/.bashrc"
out="$(GOG_DO_NOT_CREATE_BACKUPS=1 gogq apply 2>&1)"; rc=$?
echo "${out}"
assert_rc 1 "${rc}" "the retired variable does not force an overwrite"
assert_file_contents "${HOME}/.bashrc" "pre-gog"

summary
