#!/usr/bin/env bash
# repository add, list, default, names, prefixes and the data directory
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

DATA_REL=".local/share/gog"

echo "=== 1.1 repository add, by init ==="
new_sandbox repo-init
DATA="${HOME}/${DATA_REL}"
out="$(gogq repository add dots 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|" | tail -1
assert_rc 0 "${rc}" "repository add creates a repository"
assert_contains "${out}" "Added repository" "the output confirms it"
assert_dir "${DATA}/dots"
assert_exists "${DATA}/dots/.git"
out="$(cd "${DATA}/dots" && git rev-parse --is-bare-repository)"
assert_contains "${out}" "false" "the repository has a work tree"
out="$(gogq repository list 2>&1)"
assert_contains "${out}" "dots" "it is listed"

echo
echo "=== 1.2 repository add, by clone ==="
new_sandbox repo-clone
DATA="${HOME}/${DATA_REL}"
REMOTE="$(make_remote origin)"
git clone -q "${REMOTE}" "${SANDBOX}/seed"
mkdir -p "${SANDBOX}/seed/\$HOME"
printf 'from the remote\n' >"${SANDBOX}/seed/\$HOME/.bashrc"
(cd "${SANDBOX}/seed" && git add -A && git commit -q -m seed && git push -q origin main)
out="$(gogq repository add dots "${REMOTE}" 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|" | tail -1
assert_rc 0 "${rc}" "repository add clones a remote"
assert_file_contents "${DATA}/dots/\$HOME/.bashrc" "from the remote"
out="$(cd "${DATA}/dots" && git remote get-url origin)"
assert_contains "${out}" "origin.git" "the remote is configured"

echo
echo "=== 1.3 list, with and without --path ==="
new_sandbox repo-list
DATA="${HOME}/${DATA_REL}"
gogq repository add alpha >/dev/null 2>&1
gogq repository add beta >/dev/null 2>&1
out="$(gogq repository list 2>&1)"
echo "--- list: $(printf '%s' "${out}" | tr '\n' ' ')"
assert_contains "${out}" "alpha" "alpha is listed"
assert_contains "${out}" "beta" "beta is listed"
assert_not_contains "${out}" "/" "names are printed, not paths"
out="$(gogq repository list --path 2>&1)"
echo "--- list --path: $(printf '%s' "${out}" | tr '\n' ' ' | sed "s|${HOME}|~|g")"
assert_contains "${out}" "${DATA}/alpha" "--path prints the path"
out="$(gogq repository list -p 2>&1)"
assert_contains "${out}" "${DATA}/beta" "-p is the short form"

echo
echo "=== 1.4 default ==="
out="$(gogq repository default 2>&1)"; rc=$?
echo "--- default: ${out}"
assert_rc 0 "${rc}" "default succeeds"
assert_contains "${out}" "alpha" "the first repository is the default"
out="$(gogq repository default --path 2>&1)"
assert_contains "${out}" "${DATA}/alpha" "--path prints the path"
out="$(GOG_DEFAULT_REPOSITORY_NAME=beta gogq repository default 2>&1)"
echo "--- with GOG_DEFAULT_REPOSITORY_NAME=beta: ${out}"
assert_contains "${out}" "beta" "the environment variable chooses the default"
out="$(GOG_DEFAULT_REPOSITORY_NAME=b gogq repository default 2>&1)"
assert_contains "${out}" "beta" "a prefix in the environment variable resolves"
out="$(GOG_DEFAULT_REPOSITORY_NAME=nope gogq repository default 2>&1)"; rc=$?
echo "--- with GOG_DEFAULT_REPOSITORY_NAME=nope: ${out}"
assert_rc 1 "${rc}" "an unknown default fails"
assert_contains "${out}" "repository not found" "the failure says so"

echo
echo "=== 1.5 the default repository is what add and apply use ==="
printf 'x\n' >"${HOME}/.x"
gogq add ~/.x >/dev/null 2>&1
assert_symlink_to "${HOME}/.x" "${DATA}/alpha/\$HOME/.x"
printf 'y\n' >"${HOME}/.y"
GOG_DEFAULT_REPOSITORY_NAME=beta gogq add ~/.y >/dev/null 2>&1
assert_symlink_to "${HOME}/.y" "${DATA}/beta/\$HOME/.y"

echo
echo "=== 1.6 name validation and directory traversal ==="
new_sandbox repo-names
DATA="${HOME}/${DATA_REL}"
for name in "with space" "dot.name" "sub/dir" "../escape" "/absolute" "back\\slash" '$var' "" "-dash"; do
  out="$(gogq repository add "${name}" 2>&1)"; rc=$?
  printf '  %-12s -> exit %s: %s\n' "'${name}'" "${rc}" "$(printf '%s' "${out}" | head -1 | sed "s|${HOME}|~|")"
  if [ "${name}" = "-dash" ]; then continue; fi
  assert_rc 1 "${rc}" "repository add refuses '${name}'"
done
assert_not_exists "${SANDBOX}/escape"
assert_not_exists "${HOME}/.local/share/escape"
echo "--- the data directory holds:"
find "${DATA}" -mindepth 1 -maxdepth 1 | sed "s|${HOME}|~|"
out="$(gogq repository add valid-name_1 2>&1)"; rc=$?
assert_rc 0 "${rc}" "letters, digits, dashes and underscores are accepted"

echo
echo "=== 1.7 junk in the data directory ==="
new_sandbox repo-junk
DATA="${HOME}/${DATA_REL}"
gogq repository add real >/dev/null 2>&1
printf 'not a repository\n' >"${DATA}/afile"
mkdir -p "${DATA}/plaindir"
printf 'x\n' >"${DATA}/plaindir/x"
git init -q --bare "${DATA}/barerepo"
mkdir -p "${DATA}/empty"
mkdir -p "${DATA}/bad name"
out="$(gogq repository list 2>&1)"
echo "--- list: $(printf '%s' "${out}" | tr '\n' ' ')"
assert_contains "${out}" "real" "the real repository is listed"
assert_not_contains "${out}" "afile" "a plain file is not listed"
assert_not_contains "${out}" "plaindir" "a directory that is not a repository is not listed"
assert_not_contains "${out}" "barerepo" "a bare repository is not listed"
assert_not_contains "${out}" "empty" "an empty directory is not listed"
assert_not_contains "${out}" "bad name" "a directory with an invalid name is not listed"
out="$(gogq repository default 2>&1)"
assert_contains "${out}" "real" "the default repository skips the junk"
out="$(gogq apply -r plaindir 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "a directory that is not a repository cannot be used"
assert_contains "${out}" "must be initialized as a git repository" "the failure says what is wrong"

echo
echo "=== 1.8 prefix matching for --repository ==="
new_sandbox repo-prefix
DATA="${HOME}/${DATA_REL}"
gogq repository add dotfiles >/dev/null 2>&1
gogq repository add dotlocal >/dev/null 2>&1
gogq repository add other >/dev/null 2>&1
printf 'x\n' >"${HOME}/.x"
out="$(gogq add -r oth ~/.x 2>&1)"; rc=$?
assert_rc 0 "${rc}" "a unique prefix resolves"
assert_symlink_to "${HOME}/.x" "${DATA}/other/\$HOME/.x"
printf 'y\n' >"${HOME}/.y"
out="$(gogq add -r dot ~/.y 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "an ambiguous prefix is refused"
assert_contains "${out}" "ambiguous" "the failure says why"
out="$(gogq add -r nosuch ~/.y 2>&1)"; rc=$?
assert_rc 1 "${rc}" "an unknown name is refused"
assert_contains "${out}" "repository not found: nosuch" "the failure names it"
out="$(gogq add -r dotfiles ~/.y 2>&1)"; rc=$?
assert_rc 0 "${rc}" "an exact name resolves even when it is a prefix of another"
assert_symlink_to "${HOME}/.y" "${DATA}/dotfiles/\$HOME/.y"

echo
echo "=== 1.9 GOG_HOME and XDG_DATA_HOME ==="
new_sandbox repo-datadir
out="$(GOG_HOME="${SANDBOX}/gogdata" gogq repository add dots 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${SANDBOX}|SANDBOX|" | tail -1
assert_rc 0 "${rc}" "GOG_HOME chooses the data directory"
assert_exists "${SANDBOX}/gogdata/dots/.git"
assert_not_exists "${HOME}/${DATA_REL}/dots"
out="$(XDG_DATA_HOME="${SANDBOX}/xdg" gogq repository add dots 2>&1)"; rc=$?
assert_rc 0 "${rc}" "XDG_DATA_HOME chooses the data directory"
assert_exists "${SANDBOX}/xdg/gog/dots/.git"
out="$(GOG_HOME="${SANDBOX}/gogdata" XDG_DATA_HOME="${SANDBOX}/xdg" gogq repository list 2>&1)"
echo "--- with both set, list shows: $(printf '%s' "${out}" | tr '\n' ' ')"
assert_contains "${out}" "dots" "GOG_HOME wins over XDG_DATA_HOME"
out="$(GOG_HOME="${SANDBOX}/gogdata/" gogq repository list 2>&1)"
assert_contains "${out}" "dots" "a trailing slash on GOG_HOME is handled"
cd "${SANDBOX}" || exit 1
out="$(GOG_HOME=gogdata gogq repository list 2>&1)"
assert_contains "${out}" "dots" "a relative GOG_HOME resolves against the current directory"
cd "${HOME}" || exit 1

echo
echo "=== 1.10 argument handling ==="
new_sandbox repo-args
out="$(gogq repository add 2>&1)"; rc=$?
assert_rc 1 "${rc}" "repository add with no name fails"
out="$(gogq repository add one two three 2>&1)"; rc=$?
assert_rc 1 "${rc}" "repository add with too many arguments fails"
out="$(gogq repository 2>&1)"; rc=$?
echo "--- bare 'gog repository' exits ${rc}:"
printf '%s\n' "${out}" | head -3
out="$(gogq repository list extra 2>&1)"; rc=$?
assert_rc 1 "${rc}" "repository list takes no arguments"

echo
echo "=== 1.11 a repository whose path is a file ==="
new_sandbox repo-isfile
DATA="${HOME}/${DATA_REL}"
mkdir -p "${DATA}"
printf 'x\n' >"${DATA}/afile"
out="$(gogq apply -r afile 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "a data-directory file cannot be used as a repository"

echo
echo "=== 1.12 no repositories at all ==="
new_sandbox repo-none
out="$(gogq repository list 2>&1)"; rc=$?
assert_rc 0 "${rc}" "list of an empty data directory succeeds"
[ -z "${out}" ] && _ok "it prints nothing" || _bad "it prints nothing (got '${out}')"
out="$(gogq repository default 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "the default fails when there is none"
assert_contains "${out}" "no valid git repositories found" "the failure says so"
assert_contains "${out}" "gog repository add" "and says what to do"

summary
