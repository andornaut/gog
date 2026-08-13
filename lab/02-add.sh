#!/usr/bin/env bash
# gog add
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

setup() {
  new_sandbox "$1"
  gogq repository add dots >/dev/null 2>&1
  REPO="${HOME}/.local/share/gog/dots"
}

staged() { (cd "${REPO}" && git status --porcelain "$@"); }

echo "=== 2.1 \$HOME substitution ==="
setup add-home
printf 'bashrc\n' >"${HOME}/.bashrc"
out="$(gogq add ~/.bashrc 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "add a file in the home directory"
assert_exists "${REPO}/\$HOME/.bashrc"
assert_file_contents "${REPO}/\$HOME/.bashrc" "bashrc"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
assert_contains "$(staged)" 'A  $HOME/.bashrc' "the file is staged"
assert_contains "${out}" '\$HOME' "the output escapes the literal \$HOME"

echo
echo "=== 2.2 a directory ==="
setup add-dir
mkdir -p "${HOME}/.config/app/nested"
printf 'a\n' >"${HOME}/.config/app/a.conf"
printf 'b\n' >"${HOME}/.config/app/nested/b.conf"
out="$(gogq add ~/.config 2>&1)"; rc=$?
assert_rc 0 "${rc}" "add a directory"
assert_dir "${HOME}/.config"
assert_dir "${HOME}/.config/app/nested"
assert_symlink_to "${HOME}/.config/app/a.conf" "${REPO}/\$HOME/.config/app/a.conf"
assert_symlink_to "${HOME}/.config/app/nested/b.conf" "${REPO}/\$HOME/.config/app/nested/b.conf"
assert_file_contents "${REPO}/\$HOME/.config/app/nested/b.conf" "b"
[ "$(staged | wc -l)" = "2" ] && _ok "both files are staged" || _bad "both files are staged"

echo
echo "=== 2.3 a path outside \$HOME ==="
setup add-outside
mkdir -p "${SANDBOX}/etc"
printf 'sys\n' >"${SANDBOX}/etc/thing.conf"
out="$(gogq add "${SANDBOX}/etc/thing.conf" 2>&1)"; rc=$?
assert_rc 0 "${rc}" "add a path outside the home directory"
assert_exists "${REPO}${SANDBOX}/etc/thing.conf"
assert_symlink_to "${SANDBOX}/etc/thing.conf" "${REPO}${SANDBOX}/etc/thing.conf"
echo "--- stored as: $(staged | sed "s|${SANDBOX}|SANDBOX|")"
assert_not_contains "$(staged)" '$HOME' "an outside path is stored by its absolute name"

echo
echo "=== 2.4 relative and ../ paths ==="
setup add-relative
mkdir -p "${HOME}/sub"
printf 'r\n' >"${HOME}/.relative"
printf 's\n' >"${HOME}/sub/inner.conf"
cd "${HOME}/sub" || exit 1
out="$(gogq add ../.relative ./inner.conf 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "add a ../ path and a ./ path"
assert_symlink_to "${HOME}/.relative" "${REPO}/\$HOME/.relative"
assert_symlink_to "${HOME}/sub/inner.conf" "${REPO}/\$HOME/sub/inner.conf"
cd "${HOME}" || exit 1

echo
echo "=== 2.5 several paths in one invocation, and an empty argument ==="
setup add-multi
for n in a b c; do printf '%s\n' "${n}" >"${HOME}/.${n}"; done
out="$(gogq add ~/.a ~/.b ~/.c 2>&1)"; rc=$?
assert_rc 0 "${rc}" "add three paths at once"
for n in a b c; do assert_symlink_to "${HOME}/.${n}" "${REPO}/\$HOME/.${n}"; done
[ "$(staged | wc -l)" = "3" ] && _ok "all three are staged" || _bad "all three are staged"
printf 'd\n' >"${HOME}/.d"
out="$(gogq add "" ~/.d "   " 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "an empty argument fails the command"
assert_contains "${out}" "a path cannot be empty" "the empty argument is named"
assert_not_exists "${REPO}/\$HOME/.d" "nothing is added when an argument is unusable"

echo
echo "=== 2.6 idempotency, and re-adding changed contents ==="
setup add-again
printf 'one\n' >"${HOME}/.bashrc"
gogq add ~/.bashrc >/dev/null 2>&1
out="$(gogq add ~/.bashrc 2>&1)"; rc=$?
assert_rc 0 "${rc}" "adding an already linked path"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
assert_file_contents "${REPO}/\$HOME/.bashrc" "one"
echo "--- home holds: $(find "${HOME}" -maxdepth 1 -name '.bashrc*' | sed "s|${HOME}|~|" | tr '\n' ' ')"
[ "$(find "${HOME}" -maxdepth 1 -name '.bashrc*' | wc -l)" = "1" ] \
  && _ok "no second copy appears beside it" || _bad "no second copy appears beside it"
gogq git commit -q -m init >/dev/null 2>&1
printf 'two\n' >"${REPO}/\$HOME/.bashrc"
out="$(gogq add ~/.bashrc 2>&1)"; rc=$?
assert_rc 0 "${rc}" "adding a path whose contents changed in the repository"
assert_file_contents "${HOME}/.bashrc" "two"

echo
echo "=== 2.7 file modes ==="
setup add-modes
printf '#!/bin/sh\n' >"${HOME}/.script"; chmod 0755 "${HOME}/.script"
printf 'secret\n' >"${HOME}/.netrc"; chmod 0600 "${HOME}/.netrc"
printf 'plain\n' >"${HOME}/.plain"; chmod 0644 "${HOME}/.plain"
mkdir -p "${HOME}/.private"; chmod 0700 "${HOME}/.private"
printf 'p\n' >"${HOME}/.private/p.conf"
out="$(gogq add ~/.script ~/.netrc ~/.plain ~/.private 2>&1)"; rc=$?
printf '%s\n' "${out}" | grep -i warning | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "add paths with assorted modes"
[ "$(stat -c %a "${REPO}/\$HOME/.script")" = "755" ] && _ok "the executable bit is stored" \
  || _bad "the executable bit is stored (got $(stat -c %a "${REPO}/\$HOME/.script"))"
[ "$(stat -c %a "${REPO}/\$HOME/.netrc")" = "600" ] && _ok "the mode is copied locally" \
  || _bad "the mode is copied locally (got $(stat -c %a "${REPO}/\$HOME/.netrc"))"
assert_contains "${out}" ".netrc has mode 0600" "a 0600 file is warned about"
assert_contains "${out}" ".private has mode 0700" "a 0700 directory is warned about"
assert_not_contains "${out}" ".plain has mode" "a 0644 file is not warned about"
assert_not_contains "${out}" ".script has mode" "a 0755 file is not warned about"

echo
echo "=== 2.8 awkward filenames ==="
setup add-names
names=('with space.conf' "quote'.conf" 'double".conf' 'ünïcödé.conf' 'star*.conf' 'brack[et].conf' 'semi;colon.conf' 'dollar$sign.conf')
mkdir -p "${HOME}/.odd"
for n in "${names[@]}"; do printf 'x\n' >"${HOME}/.odd/${n}"; done
printf 'y\n' >"${HOME}/.odd/-leading-dash.conf"
out="$(gogq add ~/.odd 2>&1)"; rc=$?
assert_rc 0 "${rc}" "add a directory of awkward filenames"
for n in "${names[@]}"; do
  assert_symlink_to "${HOME}/.odd/${n}" "${REPO}/\$HOME/.odd/${n}"
done
assert_symlink_to "${HOME}/.odd/-leading-dash.conf" "${REPO}/\$HOME/.odd/-leading-dash.conf"
count="$(staged | wc -l)"
echo "--- ${count} paths staged"
[ "${count}" = "9" ] && _ok "every awkward name is staged" || _bad "every awkward name is staged (got ${count})"
echo "--- a name holding a newline:"
nl_name=$'new\nline.conf'
printf 'z\n' >"${HOME}/.odd/${nl_name}"
out="$(gogq add ~/.odd 2>&1)"; rc=$?
assert_rc 0 "${rc}" "add a directory holding a newline in a filename"
[ -L "${HOME}/.odd/${nl_name}" ] && _ok "the newline name is linked" || _bad "the newline name is linked"
[ "$(staged | wc -l)" = "10" ] && _ok "the newline name is staged" || _bad "the newline name is staged"

echo
echo "=== 2.9 more paths than fit in one git invocation ==="
setup add-batch
mkdir -p "${HOME}/.many"
for ((i = 0; i < 1500; i++)); do printf 'f%d\n' "${i}" >"${HOME}/.many/f${i}.conf"; done
out="$(gogq add ~/.many 2>&1)"; rc=$?
assert_rc 0 "${rc}" "add 1500 files, which spans two git invocations"
[ "$(find "${HOME}/.many" -type l | wc -l)" = "1500" ] && _ok "1500 links created" \
  || _bad "1500 links created (got $(find "${HOME}/.many" -type l | wc -l))"
count="$(staged | wc -l)"
[ "${count}" = "1500" ] && _ok "1500 paths staged" || _bad "1500 paths staged (got ${count})"
assert_not_contains "$(staged)" "??" "nothing is left untracked"

echo
echo "=== 2.10 guard rails ==="
setup add-guards
printf 'x\n' >"${HOME}/.x"
gogq add ~/.x >/dev/null 2>&1
printf 'b\n' >"${HOME}/.b.gog"
out="$(gogq add "${HOME}/.b.gog" 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "a .gog path is refused"
out="$(gogq add "${REPO}/\$HOME/.x" 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "a path inside gog's data directory is refused"
assert_contains "${out}" "gog's own directory" "the refusal says why"
out="$(gogq add "${HOME}/.local/share/gog" 2>&1)"; rc=$?
assert_rc 1 "${rc}" "the data directory itself is refused"
out="$(gogq add ~/.nothing-here 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "a path that does not exist is refused"
out="$(gogq add 2>&1)"; rc=$?
assert_rc 1 "${rc}" "add with no arguments fails"

echo
echo "=== 2.11 the whole home directory ==="
setup add-whole-home
printf 'bashrc\n' >"${HOME}/.bashrc"
mkdir -p "${HOME}/.config"
printf 'c\n' >"${HOME}/.config/c.conf"
out="$(gogq add ~ 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|" | head -5
assert_rc 0 "${rc}" "add the home directory itself"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
echo "--- did gog copy its own data directory into the repository?"
assert_not_exists "${REPO}/\$HOME/.local/share/gog"
assert_dir "${HOME}/.local/share/gog/dots"

echo
echo "=== 2.12 symbolic links given to add ==="
setup add-symlink
printf 'real\n' >"${HOME}/.real"
ln -s "${HOME}/.real" "${HOME}/.alias"
out="$(gogq add ~/.alias 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "a symbolic link given directly is refused"
assert_contains "${out}" "add that path instead" "the refusal names the target"
assert_not_exists "${REPO}/\$HOME/.alias"
ln -s "${HOME}/.gone" "${HOME}/.broken"
out="$(gogq add ~/.broken 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "a broken symbolic link is refused"
assert_contains "${out}" "symbolic link" "it is named as a link, not as its missing target"
mkdir -p "${HOME}/.tree"
printf 't\n' >"${HOME}/.tree/t.conf"
ln -s "${HOME}/.real" "${HOME}/.tree/link"
out="$(gogq add ~/.tree 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "a link inside a tree is skipped, not followed"
assert_contains "${out}" "skipping symbolic link" "the skip is reported"
assert_not_exists "${REPO}/\$HOME/.tree/link"
assert_symlink_to "${HOME}/.tree/t.conf" "${REPO}/\$HOME/.tree/t.conf"

echo
echo "=== 2.13 adding a linked path to a second repository ==="
new_sandbox add-second-repo
gogq repository add one >/dev/null 2>&1
gogq repository add two >/dev/null 2>&1
ONE="${HOME}/.local/share/gog/one"; TWO="${HOME}/.local/share/gog/two"
printf 'shared\n' >"${HOME}/.bashrc"
gogq add -r one ~/.bashrc >/dev/null 2>&1
out="$(gogq add -r two ~/.bashrc 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "adding a path that another repository already holds"
assert_file_contents "${TWO}/\$HOME/.bashrc" "shared"
assert_symlink_to "${HOME}/.bashrc" "${TWO}/\$HOME/.bashrc"
assert_exists "${ONE}/\$HOME/.bashrc"
[ "$(find "${HOME}" -maxdepth 1 -name '.bashrc*' | wc -l)" = "1" ] \
  && _ok "no copy is left beside it" || _bad "no copy is left beside it"

echo
echo "=== 2.14 -r in every position ==="
new_sandbox add-repoflag
gogq repository add alpha >/dev/null 2>&1
gogq repository add beta >/dev/null 2>&1
printf 'x\n' >"${HOME}/.x"; printf 'y\n' >"${HOME}/.y"
out="$(gogq -r beta add ~/.x 2>&1)"; rc=$?
assert_rc 0 "${rc}" "the flag may precede the command that owns it"
assert_symlink_to "${HOME}/.x" "${HOME}/.local/share/gog/beta/\$HOME/.x"
out="$(gogq add -r beta ~/.y 2>&1)"; rc=$?
assert_rc 0 "${rc}" "gog add -r NAME"
assert_symlink_to "${HOME}/.y" "${HOME}/.local/share/gog/beta/\$HOME/.y"
printf 'z\n' >"${HOME}/.z"
out="$(gogq add ~/.z --repository beta 2>&1)"; rc=$?
assert_rc 0 "${rc}" "the flag may follow the paths"
assert_symlink_to "${HOME}/.z" "${HOME}/.local/share/gog/beta/\$HOME/.z"

echo
echo "=== 2.15 empty files and empty directories ==="
setup add-empty
: >"${HOME}/.empty"
mkdir -p "${HOME}/.emptydir/deeper"
mkdir -p "${HOME}/.mixed/full"
printf 'f\n' >"${HOME}/.mixed/full/f.conf"
mkdir -p "${HOME}/.mixed/hollow"
out="$(gogq add ~/.empty ~/.emptydir ~/.mixed 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "add an empty file and empty directories"
assert_symlink_to "${HOME}/.empty" "${REPO}/\$HOME/.empty"
assert_not_exists "${REPO}/\$HOME/.emptydir"
assert_not_exists "${REPO}/\$HOME/.mixed/hollow"
assert_exists "${REPO}/\$HOME/.mixed/full/f.conf"
echo "--- git status: '$(staged)'"
assert_not_contains "$(staged)" "??" "no untracked entries are left"

summary
