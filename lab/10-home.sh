#!/usr/bin/env bash
# $HOME unset, empty, missing, unwritable, relative or root
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

echo "=== 10.1 \$HOME unset ==="
new_sandbox home-unset
out="$(env -u HOME "${GOG_BIN}" repository list 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g"
assert_rc 1 "${rc}" "gog fails when \$HOME is unset"
assert_contains "${out}" "HOME" "the failure names \$HOME"

echo
echo "=== 10.2 \$HOME empty ==="
out="$(HOME= "${GOG_BIN}" repository list 2>&1)"; rc=$?
printf '%s\n' "${out}"
assert_rc 1 "${rc}" "gog fails when \$HOME is empty"

echo
echo "=== 10.3 the shape of that failure ==="
out="$(HOME= "${GOG_BIN}" repository list 2>&1)"
assert_contains "${out}" "Error:" "the message reads like gog's other errors"
printf '%s' "${out}" | grep -qE '^[0-9]{4}/[0-9]{2}/[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2} ' \
  && _bad "no timestamp from log.Fatal" || _ok "no timestamp from log.Fatal"

echo
echo "=== 10.4 \$HOME points somewhere that does not exist ==="
new_sandbox home-missing
MISSING="${SANDBOX}/no/such/home"
out="$(HOME="${MISSING}" "${GOG_BIN}" repository list 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g"
assert_rc 1 "${rc}" "gog refuses a home directory that does not exist"
assert_contains "${out}" "home directory does not exist" "the failure says which"
assert_not_exists "${SANDBOX}/no"
echo "--- and when \$HOME names a file rather than a directory:"
printf 'x\n' >"${SANDBOX}/notadir"
out="$(HOME="${SANDBOX}/notadir" "${GOG_BIN}" repository list 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g"
assert_rc 1 "${rc}" "gog refuses a home directory that is not a directory"

echo "=== 10.5 \$HOME points somewhere unwritable ==="
new_sandbox home-readonly
RO="${SANDBOX}/ro-home"
mkdir -p "${RO}"
chmod 500 "${RO}"
out="$(HOME="${RO}" "${GOG_BIN}" repository list 2>&1)"; rc=$?
chmod 700 "${RO}"
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g"
assert_rc 1 "${rc}" "gog fails when it cannot create its data directory"
assert_contains "${out}" "cannot create gog's data directory" "the failure names what it could not do"
assert_contains "${out}" "permission denied" "the failure names the reason"

echo
echo "=== 10.6 \$HOME unwritable, but GOG_HOME writable ==="
mkdir -p "${SANDBOX}/data"
chmod 500 "${RO}"
out="$(HOME="${RO}" GOG_HOME="${SANDBOX}/data/gog" "${GOG_BIN}" repository list 2>&1)"; rc=$?
chmod 700 "${RO}"
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g"
assert_rc 0 "${rc}" "GOG_HOME keeps gog off an unwritable home directory"

echo
echo "=== 10.7 \$HOME given as a relative path ==="
new_sandbox home-relative
mkdir -p "${SANDBOX}/relhome"
cd "${SANDBOX}" || exit 1
printf 'x\n' >"${SANDBOX}/relhome/.bashrc"
out="$(HOME=relhome "${GOG_BIN}" repository add dots 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g"
assert_rc 0 "${rc}" "a relative \$HOME is accepted"
out="$(HOME=relhome "${GOG_BIN}" add "${SANDBOX}/relhome/.bashrc" 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g"
assert_rc 0 "${rc}" "adding a file under a relative \$HOME"
echo "--- how it was stored:"
find "${SANDBOX}/relhome/.local/share/gog/dots" -name '.bashrc' | sed "s|${SANDBOX}|SANDBOX|g"
if [ -e "${SANDBOX}/relhome/.local/share/gog/dots/\$HOME/.bashrc" ]; then
  _ok "stored under \$HOME, so the repository stays portable"
else
  _bad "stored under \$HOME, so the repository stays portable"
fi
cd "${HOME}" || exit 1

echo
echo "=== 10.8 \$HOME is the root directory ==="
new_sandbox home-root
mkdir -p "${SANDBOX}/data" "${SANDBOX}/files"
printf 'rooted\n' >"${SANDBOX}/files/thing.conf"
export GOG_HOME="${SANDBOX}/data/gog"
out="$(HOME=/ "${GOG_BIN}" repository add dots 2>&1)"; rc=$?
assert_rc 0 "${rc}" "a repository can be added with \$HOME=/"
out="$(HOME=/ "${GOG_BIN}" add "${SANDBOX}/files/thing.conf" 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g"
assert_rc 0 "${rc}" "adding a file with \$HOME=/"
echo "--- how it was stored:"
find "${GOG_HOME}/dots" -name 'thing.conf' | sed "s|${SANDBOX}|SANDBOX|g"
assert_symlink_to "${SANDBOX}/files/thing.conf" "${GOG_HOME}/dots/\$HOME${SANDBOX}/files/thing.conf"
rm "${SANDBOX}/files/thing.conf"
out="$(HOME=/ "${GOG_BIN}" apply 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g"
assert_rc 0 "${rc}" "applying with \$HOME=/"
assert_symlink_to "${SANDBOX}/files/thing.conf" "${GOG_HOME}/dots/\$HOME${SANDBOX}/files/thing.conf"
out="$(HOME=/ "${GOG_BIN}" remove "${SANDBOX}/files/thing.conf" 2>&1)"; rc=$?
assert_rc 0 "${rc}" "removing with \$HOME=/"
assert_regular "${SANDBOX}/files/thing.conf"
assert_file_contents "${SANDBOX}/files/thing.conf" "rooted"
unset GOG_HOME

echo
echo "=== 10.9 GOG_HOME and XDG_DATA_HOME pointing somewhere unwritable ==="
new_sandbox home-datadir
mkdir -p "${SANDBOX}/locked"
chmod 500 "${SANDBOX}/locked"
out="$(GOG_HOME="${SANDBOX}/locked/gog" "${GOG_BIN}" repository list 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g"
assert_rc 1 "${rc}" "an unwritable GOG_HOME fails"
out="$(XDG_DATA_HOME="${SANDBOX}/locked/xdg" "${GOG_BIN}" repository list 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g"
assert_rc 1 "${rc}" "an unwritable XDG_DATA_HOME fails"
chmod 700 "${SANDBOX}/locked"

echo
echo "=== 10.10 \$HOME with a trailing slash ==="
new_sandbox home-trailing
printf 'x\n' >"${HOME}/.bashrc"
out="$(HOME="${HOME}/" "${GOG_BIN}" repository add dots 2>&1)"; rc=$?
assert_rc 0 "${rc}" "a trailing slash on \$HOME is accepted"
out="$(HOME="${SANDBOX}/home/" "${GOG_BIN}" add "${HOME}/.bashrc" 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g"
assert_rc 0 "${rc}" "adding with a trailing slash on \$HOME"
assert_symlink_to "${HOME}/.bashrc" "${HOME}/.local/share/gog/dots/\$HOME/.bashrc"

summary
