#!/usr/bin/env bash
LAB_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOG_BIN="${LAB_ROOT}/bin/gog"
SANDBOX=""; PASS=0; FAIL=0; FAILURES=()

new_sandbox() {
  local name="$1"
  SANDBOX="${LAB_ROOT}/sandboxes/${name}"
  rm -rf "${SANDBOX}"; mkdir -p "${SANDBOX}/home" "${SANDBOX}/remotes" "${SANDBOX}/etc"
  export HOME="${SANDBOX}/home"
  unset XDG_DATA_HOME GOG_HOME GOG_DEFAULT_REPOSITORY_NAME \
        GOG_DO_NOT_CREATE_BACKUPS GOG_IGNORE_FILES_REGEX
  # A real file: gog scrubs GIT_CONFIG_* from git's environment
  cat >"${HOME}/.gitconfig" <<'EOF'
[user]
	name = Lab Tester
	email = lab@example.invalid
[init]
	defaultBranch = main
[commit]
	gpgsign = false
EOF
  cd "${HOME}" || exit 1
  echo "### sandbox: ${SANDBOX}"
}

gog()  { echo "\$ gog $*"; "${GOG_BIN}" "$@"; local rc=$?; echo "[exit ${rc}]"; return ${rc}; }
gogq() { "${GOG_BIN}" "$@"; }

_ok()  { PASS=$((PASS+1)); echo "  PASS: $1"; }
_bad() { FAIL=$((FAIL+1)); FAILURES+=("$1"); echo "  FAIL: $1"; }

assert_symlink_to() {
  local link="$1" want="$2" got
  if [ ! -L "${link}" ]; then _bad "symlink ${link} exists"; return 1; fi
  got="$(readlink "${link}")"
  [ "${got}" = "${want}" ] && _ok "${link} -> ${want}" || _bad "${link} -> ${want} (actual: ${got})"
}
assert_file_contents() {
  local p="$1" want="$2" got
  if [ ! -e "${p}" ]; then _bad "file ${p} exists"; return 1; fi
  got="$(cat "${p}")"
  [ "${got}" = "${want}" ] && _ok "contents of ${p} == '${want}'" \
                           || _bad "contents of ${p} == '${want}' (actual: '${got}')"
}
assert_exists()     { [ -e "$1" ] && _ok "exists: $1" || _bad "exists: $1"; }
assert_not_exists() { [ ! -e "$1" ] && _ok "absent: $1" || _bad "absent: $1"; }
assert_regular()    { { [ -f "$1" ] && [ ! -L "$1" ]; } && _ok "regular file: $1" || _bad "regular file: $1"; }
assert_dir()        { { [ -d "$1" ] && [ ! -L "$1" ]; } && _ok "real dir: $1" || _bad "real dir: $1"; }
assert_rc()         { [ "$1" = "$2" ] && _ok "$3 (exit $2)" || _bad "$3 (want exit $1, got $2)"; }
assert_contains()     { case "$1" in *"$2"*) _ok "$3";; *) _bad "$3";; esac; }
assert_not_contains() { case "$1" in *"$2"*) _bad "$3";; *) _ok "$3";; esac; }

make_remote() {
  local p="${SANDBOX}/remotes/$1.git"
  git init --quiet --bare --initial-branch=main "${p}"
  echo "${p}"
}

summary() {
  echo; echo "=============================================="
  echo "RESULT: ${PASS} passed, ${FAIL} failed"
  [ "${FAIL}" -gt 0 ] && printf '  - %s\n' "${FAILURES[@]}"
  echo "=============================================="
}
