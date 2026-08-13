#!/usr/bin/env bash
# what a failed repository add leaves behind
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

echo "=== 11.1 a clone from a URL that does not resolve ==="
new_sandbox add-badurl
DATA="${HOME}/.local/share/gog"
out="$(gogq repository add dots "file://${SANDBOX}/nowhere.git" 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g" | head -3
assert_rc 1 "${rc}" "a failed clone fails the command"
assert_not_exists "${DATA}/dots"
echo "--- the data directory now holds:"
find "${DATA}" -mindepth 1 -maxdepth 1 | sed "s|${SANDBOX}|SANDBOX|g"
[ -z "$(find "${DATA}" -mindepth 1 -maxdepth 1)" ] && _ok "nothing was left behind" \
  || _bad "nothing was left behind"

echo
echo "=== 11.2 a clone from an empty directory that is not a repository ==="
mkdir -p "${SANDBOX}/notarepo"
out="$(gogq repository add dots "${SANDBOX}/notarepo" 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g" | head -3
assert_rc 1 "${rc}" "cloning something that is not a repository fails"
assert_not_exists "${HOME}/.local/share/gog/dots"

echo
echo "=== 11.3 retrying with a good URL after a failure ==="
REMOTE="$(make_remote origin)"
git clone -q "${REMOTE}" "${SANDBOX}/seed"
printf 'x\n' >"${SANDBOX}/seed/file"
(cd "${SANDBOX}/seed" && git add file && git commit -q -m seed && git push -q origin main)
out="$(gogq repository add dots "${REMOTE}" 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g" | tail -1
assert_rc 0 "${rc}" "the retry succeeds"
assert_exists "${HOME}/.local/share/gog/dots/.git"
out="$(gogq repository list 2>&1)"
assert_contains "${out}" "dots" "the repository is listed"

echo
echo "=== 11.4 an empty directory the user left in the data directory ==="
new_sandbox add-emptydir
DATA="${HOME}/.local/share/gog"
mkdir -p "${DATA}/mine"
out="$(gogq repository add mine 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g" | tail -1
assert_rc 0 "${rc}" "an empty directory is reused"
assert_exists "${DATA}/mine/.git"

echo
echo "=== 11.5 a non-empty directory is refused, and left alone ==="
new_sandbox add-nonempty
DATA="${HOME}/.local/share/gog"
mkdir -p "${DATA}/notmine"
printf 'important\n' >"${DATA}/notmine/data"
out="$(gogq repository add notmine 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g" | tail -1
assert_rc 1 "${rc}" "a non-empty directory is refused"
assert_contains "${out}" "not a gog repository" "the refusal says why"
assert_file_contents "${DATA}/notmine/data" "important"

echo
echo "=== 11.6 an existing repository is refused, and left alone ==="
gogq repository add dots >/dev/null 2>&1
printf 'x\n' >"${HOME}/.x"
gogq -r dots add ~/.x >/dev/null 2>&1
out="$(gogq repository add dots 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g" | tail -1
assert_rc 1 "${rc}" "an existing repository is refused"
assert_contains "${out}" "already exists" "the refusal says why"
assert_symlink_to "${HOME}/.x" "${DATA}/dots/\$HOME/.x"

summary
