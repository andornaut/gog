#!/usr/bin/env bash
# gog apply
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# publish NAME: a repository built in this sandbox, pushed to a bare remote
# whose path is echoed, so that a fresh home can clone it
publish() {
  local name="$1" remote
  remote="$(make_remote "${name}")"
  gogq -r "${name}" git remote add origin "${remote}" >/dev/null 2>&1
  gogq -r "${name}" git commit -q -m published >/dev/null 2>&1
  gogq -r "${name}" git push -q -u origin main >/dev/null 2>&1
  echo "${remote}"
}

echo "=== 3.1 the fresh-machine flow ==="
new_sandbox apply-fresh
gogq repository add dots >/dev/null 2>&1
printf 'bashrc\n' >"${HOME}/.bashrc"
mkdir -p "${HOME}/.config/app/nested"
printf 'a\n' >"${HOME}/.config/app/a.conf"
printf 'b\n' >"${HOME}/.config/app/nested/b.conf"
printf '#!/bin/sh\n' >"${HOME}/.script"; chmod 0755 "${HOME}/.script"
gogq add ~/.bashrc ~/.config ~/.script >/dev/null 2>&1
REMOTE="$(publish dots)"

# A second machine: a new home directory, the same remote
OLD_HOME="${HOME}"
export HOME="${SANDBOX}/home2"
mkdir -p "${HOME}"
cp "${OLD_HOME}/.gitconfig" "${HOME}/.gitconfig"
REPO="${HOME}/.local/share/gog/dots"
out="$(gogq repository add dots "${REMOTE}" 2>&1)"; rc=$?
assert_rc 0 "${rc}" "clone the repository on the second machine"
out="$(gogq apply 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|" | head -6
assert_rc 0 "${rc}" "apply on the second machine"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
assert_file_contents "${HOME}/.bashrc" "bashrc"
assert_file_contents "${HOME}/.config/app/nested/b.conf" "b"
assert_dir "${HOME}/.config/app/nested"
[ -x "${HOME}/.script" ] && _ok "the executable bit survived the round trip" \
  || _bad "the executable bit survived the round trip"
echo "--- git status after apply: '$(cd "${REPO}" && git status --porcelain)'"
[ -z "$(cd "${REPO}" && git status --porcelain)" ] && _ok "apply leaves the repository clean" \
  || _bad "apply leaves the repository clean"

echo
echo "=== 3.2 apply is idempotent ==="
out="$(gogq apply 2>&1)"; rc=$?
assert_rc 0 "${rc}" "a second apply"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
[ "$(find "${HOME}" -name '*.gog' | wc -l)" = "0" ] && _ok "nothing is left beside the links" \
  || _bad "nothing is left beside the links"
[ -z "$(cd "${REPO}" && git status --porcelain)" ] && _ok "the repository is still clean" \
  || _bad "the repository is still clean"
export HOME="${OLD_HOME}"

echo
echo "=== 3.3 the repository-root skip list ==="
new_sandbox apply-skiplist
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
printf 'ignore\n' >"${REPO}/.gitignore"
printf 'license\n' >"${REPO}/LICENSE"
printf 'readme\n' >"${REPO}/README.md"
mkdir -p "${REPO}/\$HOME/.config"
printf 'inner readme\n' >"${REPO}/\$HOME/.config/README.md"
printf 'x\n' >"${REPO}/\$HOME/.bashrc"
out="$(gogq apply 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "apply with a skip list at the repository root"
assert_not_exists "/.gitignore"
assert_not_exists "/LICENSE"
assert_not_exists "/README.md"
assert_symlink_to "${HOME}/.bashrc" "${REPO}/\$HOME/.bashrc"
assert_symlink_to "${HOME}/.config/README.md" "${REPO}/\$HOME/.config/README.md"
echo "--- (a README.md below the root is linked; only the repository's own is skipped)"

echo
echo "=== 3.4 a file at the repository root goes to the filesystem root ==="
# By design: gog manages absolute paths, and $HOME is a component within that
# rather than the whole of it, so repo/Makefile means /Makefile just as
# repo/etc/thing.conf means /etc/thing.conf. The three names above are skipped
# because a git repository conventionally carries them, not because a file at
# the root is wrong. This run fails only because the lab is not root.
printf 'make\n' >"${REPO}/Makefile"
out="$(gogq apply 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|" | grep -i 'error' | head -2
assert_rc 1 "${rc}" "an unprivileged user cannot write to the filesystem root"
assert_contains "${out}" "/Makefile" "the target is the filesystem root"
assert_not_exists "/Makefile"
rm "${REPO}/Makefile"

echo
echo "=== 3.5 .git is never linked ==="
assert_not_exists "${HOME}/.git"
assert_not_exists "/.git"
out="$(gogq apply 2>&1)"; rc=$?
assert_rc 0 "${rc}" "apply once the stray root file is gone"
assert_not_exists "${HOME}/.git"

echo
echo "=== 3.6 GOG_IGNORE_FILES_REGEX ==="
new_sandbox apply-ignore
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
mkdir -p "${REPO}/\$HOME/.config" "${REPO}/\$HOME/.cache"
printf 'keep\n' >"${REPO}/\$HOME/.config/keep.conf"
printf 'swap\n' >"${REPO}/\$HOME/.config/notes.swp"
printf 'tmp\n' >"${REPO}/\$HOME/.config/scratch.tmp"
printf 'cached\n' >"${REPO}/\$HOME/.cache/thing"
out="$(GOG_IGNORE_FILES_REGEX='\.swp$|\.tmp$' gogq apply 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "apply with an ignore pattern"
assert_symlink_to "${HOME}/.config/keep.conf" "${REPO}/\$HOME/.config/keep.conf"
assert_not_exists "${HOME}/.config/notes.swp"
assert_not_exists "${HOME}/.config/scratch.tmp"
assert_symlink_to "${HOME}/.cache/thing" "${REPO}/\$HOME/.cache/thing"
out="$(GOG_IGNORE_FILES_REGEX='\.cache/' gogq apply 2>&1)"; rc=$?
assert_rc 0 "${rc}" "apply with a directory pattern"
rm -rf "${HOME}/.cache"
out="$(GOG_IGNORE_FILES_REGEX='\.cache/' gogq apply 2>&1)"; rc=$?
assert_not_exists "${HOME}/.cache/thing"
echo "--- (the pattern matches repository-relative paths, so \$HOME/.cache/thing is skipped)"
out="$(GOG_IGNORE_FILES_REGEX='[' gogq apply 2>&1)"; rc=$?
printf '%s\n' "${out}"
assert_rc 1 "${rc}" "an unparseable pattern fails"
assert_contains "${out}" "Error: invalid GOG_IGNORE_FILES_REGEX" "the failure reads like gog's other errors"
printf '%s' "${out}" | grep -qE '^[0-9]{4}/[0-9]{2}/[0-9]{2} ' && _bad "no timestamp" || _ok "no timestamp"

echo
echo "=== 3.7 a path outside \$HOME ==="
new_sandbox apply-outside
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
mkdir -p "${SANDBOX}/etc"
printf 'sys\n' >"${SANDBOX}/etc/thing.conf"
gogq add "${SANDBOX}/etc/thing.conf" >/dev/null 2>&1
gogq git commit -q -m init >/dev/null 2>&1
rm "${SANDBOX}/etc/thing.conf"
out="$(gogq apply 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|g"
assert_rc 0 "${rc}" "apply restores a path outside the home directory"
assert_symlink_to "${SANDBOX}/etc/thing.conf" "${REPO}${SANDBOX}/etc/thing.conf"
assert_file_contents "${SANDBOX}/etc/thing.conf" "sys"

echo
echo "=== 3.8 the multi-repository overlay from the README ==="
new_sandbox apply-overlay
gogq repository add base >/dev/null 2>&1
gogq repository add local >/dev/null 2>&1
BASE="${HOME}/.local/share/gog/base"; LOCAL="${HOME}/.local/share/gog/local"
mkdir -p "${BASE}/\$HOME" "${LOCAL}/\$HOME"
printf 'from base\n' >"${BASE}/\$HOME/.bashrc"
printf 'from base\n' >"${BASE}/\$HOME/.vimrc"
printf 'from local\n' >"${LOCAL}/\$HOME/.bashrc"
out="$(for repoName in $(gogq repository list | sort -r); do gogq --repository "${repoName}" apply; done 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "the README loop applies every repository"
assert_symlink_to "${HOME}/.bashrc" "${BASE}/\$HOME/.bashrc"
assert_file_contents "${HOME}/.bashrc" "from base"
assert_symlink_to "${HOME}/.vimrc" "${BASE}/\$HOME/.vimrc"
echo "--- (reverse order applies 'local' first, so 'base' wins the overlapping path)"
[ "$(find "${HOME}" -maxdepth 1 -name '*.gog' | wc -l)" = "0" ] \
  && _ok "the loop leaves nothing beside the links" || _bad "the loop leaves nothing beside the links"
out="$(for repoName in $(gogq repository list | sort -r); do gogq --repository "${repoName}" apply; done 2>&1)"; rc=$?
assert_rc 0 "${rc}" "running the loop twice changes nothing"
assert_symlink_to "${HOME}/.bashrc" "${BASE}/\$HOME/.bashrc"

echo
echo "=== 3.9 an empty repository, and apply with arguments ==="
new_sandbox apply-empty
gogq repository add dots >/dev/null 2>&1
out="$(gogq apply 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "apply of an empty repository succeeds"
out="$(gogq apply extra 2>&1)"; rc=$?
assert_rc 1 "${rc}" "apply takes no arguments"

echo
echo "=== 3.10 apply stages what it links ==="
new_sandbox apply-stages
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
mkdir -p "${REPO}/\$HOME"
printf 'x\n' >"${REPO}/\$HOME/.bashrc"
echo "--- a file copied into the repository by hand is untracked:"
echo "    '$(cd "${REPO}" && git status --porcelain)'"
out="$(gogq apply 2>&1)"; rc=$?
assert_rc 0 "${rc}" "apply links it"
out="$(cd "${REPO}" && git status --porcelain)"
echo "--- after apply: '${out}'"
assert_contains "${out}" 'A  $HOME/.bashrc' "apply stages what it linked"

summary
