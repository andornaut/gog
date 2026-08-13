#!/usr/bin/env bash
# gog git attached to a terminal
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# onpty CMD: run a shell command with a pseudo-terminal on its standard output,
# which is what makes git page, colour and prompt. stdin is closed so that a
# pager waiting for a keypress cannot stall the run; every call is bounded.
onpty() { timeout 20 script -q -c "$1" /dev/null </dev/null 2>&1; }

setup() {
  new_sandbox "$1"
  gogq repository add dots >/dev/null 2>&1
  REPO="${HOME}/.local/share/gog/dots"
  printf 'bashrc\n' >"${HOME}/.bashrc"
  gogq add ~/.bashrc >/dev/null 2>&1
  gogq git commit -q -m 'first commit' >/dev/null 2>&1
}

echo "=== 13.1 git sees a terminal through gog ==="
setup tty-detect
out="$(onpty "'${GOG_BIN}' git config --get-color '' 'red' >/dev/null; '${GOG_BIN}' git status")"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|" | head -4
assert_rc 0 "${rc}" "gog git status on a terminal"
out="$(onpty "'${GOG_BIN}' git -c color.ui=always status --short --branch")"
if printf '%s' "${out}" | grep -q $'\033\['; then
  _ok "git emits colour through gog's stdout"
else
  _bad "git emits colour through gog's stdout"
fi
out="$(gogq git -c color.ui=auto status --short --branch 2>/dev/null)"
if printf '%s' "${out}" | grep -q $'\033\['; then
  _bad "colour is off when stdout is a pipe"
else
  _ok "colour is off when stdout is a pipe"
fi

echo
echo "=== 13.2 the pager runs when there is a terminal ==="
setup tty-pager
out="$(onpty "GIT_PAGER='sed -e s/^/PAGED:/' '${GOG_BIN}' git log --oneline")"; rc=$?
printf '%s\n' "${out}" | head -3
assert_rc 0 "${rc}" "gog git log through a pager"
assert_contains "${out}" "PAGED:" "git handed its output to the pager"
assert_contains "${out}" "first commit" "the commit reached the pager"
out="$(GIT_PAGER='sed -e s/^/PAGED:/' gogq git log --oneline 2>/dev/null)"
assert_not_contains "${out}" "PAGED:" "no pager runs when stdout is a pipe"

echo
echo "=== 13.3 the banner does not go through the pager ==="
out="$(onpty "GIT_PAGER='sed -e s/^/PAGED:/' '${GOG_BIN}' git log --oneline")"
printf '%s\n' "${out}" | head -3
assert_contains "${out}" "Repository: dots" "the banner is still shown"
if printf '%s' "${out}" | grep -q 'PAGED:Repository'; then
  _bad "the banner is not swallowed by the pager"
else
  _ok "the banner is not swallowed by the pager"
fi

echo
echo "=== 13.4 the default pager does not stall the run ==="
out="$(onpty "'${GOG_BIN}' git log")"; rc=$?
echo "--- exit ${rc} (124 would mean it waited for a keypress)"
assert_rc 0 "${rc}" "gog git log with git's own pager"
assert_contains "${out}" "first commit" "the log was shown"

echo
echo "=== 13.5 an editor is launched with the terminal ==="
setup tty-editor
cat >"${SANDBOX}/fake-editor" <<'EOF'
#!/usr/bin/env bash
printf 'written by the editor\n' >"$1"
EOF
chmod 0755 "${SANDBOX}/fake-editor"
printf 'changed\n' >"${REPO}/\$HOME/.bashrc"
out="$(onpty "GIT_EDITOR='${SANDBOX}/fake-editor' '${GOG_BIN}' git commit -a")"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|" | head -3
assert_rc 0 "${rc}" "gog git commit opens an editor on a terminal"
out="$(gogq git log -1 --format=%s 2>/dev/null | tail -1)"
echo "--- the recorded subject: '${out}'"
assert_contains "${out}" "written by the editor" "the editor's message was committed"

echo
echo "=== 13.6 an editor also works without a terminal ==="
printf 'again\n' >"${REPO}/\$HOME/.bashrc"
out="$(GIT_EDITOR="${SANDBOX}/fake-editor" gogq git commit -a 2>&1)"; rc=$?
assert_rc 0 "${rc}" "gog git commit with an editor and no terminal"
out="$(gogq git log -1 --format=%s 2>/dev/null | tail -1)"
assert_contains "${out}" "written by the editor" "the editor's message was committed"

echo
echo "=== 13.7 a prompt reads from the terminal ==="
setup tty-prompt
printf 'junk\n' >"${REPO}/untracked-file"
out="$(timeout 20 script -q -c "'${GOG_BIN}' git clean -i" /dev/null <<<$'\n' 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|" | head -6
echo "--- exit ${rc}"
assert_rc 0 "${rc}" "an interactive git command reads the terminal and returns"
assert_contains "${out}" "What now" "git prompted"

echo
echo "=== 13.8 a failing command still reports git's status on a terminal ==="
setup tty-exit
out="$(onpty "'${GOG_BIN}' git rev-parse --verify nosuchref; echo EXIT=\$?")"
printf '%s\n' "${out}" | sed "s|${HOME}|~|" | head -3
assert_contains "${out}" "EXIT=128" "git's exit status survives the terminal"
assert_not_contains "${out}" "git exited with status" "gog does not restate the failure"

echo
echo "=== 13.9 gog's own commands on a terminal ==="
setup tty-gog
out="$(onpty "'${GOG_BIN}' apply")"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|" | head -3
assert_rc 0 "${rc}" "gog apply on a terminal"
assert_contains "${out}" "Repository: dots" "the banner is shown"
out="$(onpty "'${GOG_BIN}' repository list")"
assert_contains "${out}" "dots" "repository list on a terminal"

summary
