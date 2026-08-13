#!/usr/bin/env bash
# permission-denied paths, as an unprivileged user
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

if [ "$(id -u)" = "0" ]; then
  echo "REFUSING TO RUN AS ROOT: every permission check below would be bypassed"
  exit 1
fi

setup() {
  new_sandbox "$1"
  gogq repository add dots >/dev/null 2>&1
  REPO="${HOME}/.local/share/gog/dots"
}

echo "=== 9.1 add a file that cannot be read ==="
setup perm-add-unreadable
printf 'secret\n' >"${HOME}/.netrc"
chmod 000 "${HOME}/.netrc"
out="$(gogq add ~/.netrc 2>&1)"; rc=$?
chmod 600 "${HOME}/.netrc"
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "add of an unreadable file fails"
assert_contains "${out}" "permission denied" "the failure names the reason"
assert_not_exists "${REPO}/\$HOME/.netrc"
assert_regular "${HOME}/.netrc"

echo
echo "=== 9.2 add a file in a directory that cannot be searched ==="
setup perm-add-nosearch
mkdir -p "${HOME}/.config/locked"
printf 'x\n' >"${HOME}/.config/locked/x.conf"
chmod 000 "${HOME}/.config/locked"
out="$(gogq add ~/.config/locked/x.conf 2>&1)"; rc=$?
chmod 700 "${HOME}/.config/locked"
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "add through an unsearchable directory fails"
assert_contains "${out}" "permission denied" "the failure names the reason"

echo
echo "=== 9.3 add a tree holding one unreadable file ==="
setup perm-add-tree
mkdir -p "${HOME}/.config/app"
for i in 1 2 3 4; do printf 'f%d\n' "${i}" >"${HOME}/.config/app/f${i}.conf"; done
chmod 000 "${HOME}/.config/app/f3.conf"
out="$(gogq add ~/.config 2>&1)"; rc=$?
chmod 600 "${HOME}/.config/app/f3.conf"
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "add of a tree with an unreadable file fails"
echo "--- what reached the repository:"
find "${REPO}/\$HOME" -type f 2>/dev/null | sed "s|${HOME}|~|"
echo "--- links created in \$HOME: $(find "${HOME}/.config" -type l | wc -l)"
out="$(cd "${REPO}" && git status --porcelain)"
echo "--- git status: '${out}'"
[ "$(find "${REPO}/\$HOME" -type f 2>/dev/null | wc -l)" = "0" ] \
  && _ok "the partial copy was discarded" || _bad "the partial copy was discarded"
[ -z "${out}" ] && _ok "the repository is left as it was" || _bad "the repository is left as it was"
echo "--- rerunning after the permission is fixed:"
out="$(gogq add ~/.config 2>&1)"; rc=$?
assert_rc 0 "${rc}" "the same add converges once the file is readable"
[ "$(find "${HOME}/.config" -type l | wc -l)" = "4" ] && _ok "all four files linked after the rerun" \
  || _bad "all four files linked after the rerun (got $(find "${HOME}/.config" -type l | wc -l))"
untracked="$(cd "${REPO}" && git status --porcelain | grep -c '^??')"
[ "${untracked}" = "0" ] && _ok "nothing left untracked" || _bad "nothing left untracked (got ${untracked})"

echo
echo "=== 9.3b a failed add into a repository that already held the tree ==="
new_sandbox perm-add-tree-held
REMOTE="$(make_remote origin)"
gogq repository add source >/dev/null 2>&1
mkdir -p "${HOME}/.config/app"
for i in 1 2 3 4; do printf 'f%d\n' "${i}" >"${HOME}/.config/app/f${i}.conf"; done
gogq -r source add ~/.config >/dev/null 2>&1
gogq -r source git commit -q -m init >/dev/null 2>&1
gogq -r source git remote add origin "${REMOTE}" >/dev/null 2>&1
gogq -r source git push -q -u origin main >/dev/null 2>&1
gogq repository add dots "${REMOTE}" >/dev/null 2>&1
DOTS="${HOME}/.local/share/gog/dots"
gogq -r source repository remove source >/dev/null 2>&1
gogq repository remove source -f >/dev/null 2>&1
assert_regular "${HOME}/.config/app/f1.conf"
assert_exists "${DOTS}/\$HOME/.config/app/f1.conf"
printf 'changed\n' >"${HOME}/.config/app/f1.conf"
chmod 000 "${HOME}/.config/app/f3.conf"
out="$(gogq -r dots add ~/.config 2>&1)"; rc=$?
chmod 600 "${HOME}/.config/app/f3.conf"
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "the add fails"
assert_contains "${out}" "partial copy" "the failure says the repository holds a partial copy"
assert_exists "${DOTS}/\$HOME/.config/app/f1.conf"
out="$(gogq -r dots add ~/.config 2>&1)"; rc=$?
assert_rc 0 "${rc}" "the add converges on a rerun"
assert_file_contents "${DOTS}/\$HOME/.config/app/f1.conf" "changed"

echo
echo "=== 9.4 add when the repository cannot be written ==="
setup perm-add-ro-repo
printf 'x\n' >"${HOME}/.x"
chmod 500 "${REPO}"
out="$(gogq add ~/.x 2>&1)"; rc=$?
chmod 700 "${REPO}"
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "add into a read-only repository fails"
assert_contains "${out}" "permission denied" "the failure names the reason"
assert_regular "${HOME}/.x"
assert_file_contents "${HOME}/.x" "x"

echo
echo "=== 9.5 apply into a directory that cannot be written ==="
setup perm-apply-ro
mkdir -p "${HOME}/.config/app"
printf 'a\n' >"${HOME}/.config/app/a.conf"
printf 'b\n' >"${HOME}/.bashrc"
gogq add ~/.config ~/.bashrc >/dev/null 2>&1
gogq git commit -q -m init >/dev/null 2>&1
rm -rf "${HOME}/.config/app" "${HOME}/.bashrc"
mkdir -p "${HOME}/.config/app"
chmod 500 "${HOME}/.config/app"
out="$(gogq apply 2>&1)"; rc=$?
chmod 700 "${HOME}/.config/app"
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "apply into an unwritable directory fails"
assert_contains "${out}" "failed to create symlink" "the failure says what could not be done"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
echo "--- (the other path was still linked: one failure does not stop the run)"
out="$(gogq apply 2>&1)"; rc=$?
assert_rc 0 "${rc}" "apply converges once the directory is writable"
assert_symlink_to "${HOME}/.config/app/a.conf" "${REPO}/\$HOME/.config/app/a.conf"

echo
echo "=== 9.6 apply when a directory cannot be created ==="
setup perm-apply-nomkdir
mkdir -p "${HOME}/.config/app"
printf 'a\n' >"${HOME}/.config/app/a.conf"
gogq add ~/.config >/dev/null 2>&1
gogq git commit -q -m init >/dev/null 2>&1
rm -rf "${HOME}/.config"
chmod 500 "${HOME}"
out="$(gogq apply 2>&1)"; rc=$?
chmod 700 "${HOME}"
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "apply fails when a directory cannot be created"
assert_contains "${out}" "failed to create directory" "the failure says what could not be done"

echo
echo "=== 9.7 apply when the repository cannot be read ==="
setup perm-apply-noread
mkdir -p "${HOME}/.config/app"
printf 'a\n' >"${HOME}/.config/app/a.conf"
gogq add ~/.config >/dev/null 2>&1
gogq git commit -q -m init >/dev/null 2>&1
rm -rf "${HOME}/.config"
chmod 000 "${REPO}/\$HOME/.config"
out="$(gogq apply 2>&1)"; rc=$?
chmod 700 "${REPO}/\$HOME/.config"
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "apply fails when part of the repository cannot be read"
assert_contains "${out}" "permission denied" "the failure names the reason"

echo
echo "=== 9.8 remove when the file cannot be restored ==="
setup perm-remove-ro
mkdir -p "${HOME}/.config/app"
printf 'a\n' >"${HOME}/.config/app/a.conf"
gogq add ~/.config >/dev/null 2>&1
gogq git commit -q -m init >/dev/null 2>&1
chmod 500 "${HOME}/.config/app"
out="$(gogq remove ~/.config 2>&1)"; rc=$?
chmod 700 "${HOME}/.config/app"
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "remove fails when the file cannot be restored"
assert_contains "${out}" "permission denied" "the failure names the reason"
echo "--- is the repository's copy still there?"
assert_exists "${REPO}/\$HOME/.config/app/a.conf"
assert_symlink_to "${HOME}/.config/app/a.conf" "${REPO}/\$HOME/.config/app/a.conf"
echo "--- (nothing was untracked before the restore failed)"
out="$(cd "${REPO}" && git status --porcelain)"
echo "--- git status: '${out}'"
out="$(gogq remove ~/.config 2>&1)"; rc=$?
assert_rc 0 "${rc}" "remove converges once the directory is writable"
assert_regular "${HOME}/.config/app/a.conf"
assert_file_contents "${HOME}/.config/app/a.conf" "a"

echo
echo "=== 9.9 a symbolic link that cannot be created at all ==="
echo "--- a filesystem without symlink support needs a mount, which needs root;"
echo "--- the closest reachable case is a destination that refuses the call:"
setup perm-symlink-refused
printf 'a\n' >"${HOME}/.bashrc"
gogq add ~/.bashrc >/dev/null 2>&1
gogq git commit -q -m init >/dev/null 2>&1
rm "${HOME}/.bashrc"
chmod 500 "${HOME}"
out="$(gogq apply 2>&1)"; rc=$?
chmod 700 "${HOME}"
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "apply fails when the symlink call is refused"
assert_contains "${out}" "failed to create symlink" "the failure names the operation"
assert_not_exists "${HOME}/.bashrc"

summary
