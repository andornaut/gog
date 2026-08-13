#!/usr/bin/env bash
# The command line itself: unknown commands, argument validation, --version,
# and the form gog's failures take
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

echo "=== 14.1 an unknown command fails ==="
new_sandbox cli-unknown
out="$(gogq bogus 2>&1)"; rc=$?
printf '%s\n' "${out}" | head -2
assert_rc 1 "${rc}" "an unknown command at the top level"
assert_contains "${out}" 'unknown command "bogus"' "it names what was not understood"
assert_contains "${out}" "gog --help" "and points at the listing of what is available"
out="$(gogq repository bogus 2>&1)"; rc=$?
assert_rc 1 "${rc}" "an unknown repository subcommand"
assert_contains "${out}" 'unknown command "bogus"' "it is reported the same way"
out="$(gogq 2>&1)"; rc=$?
assert_rc 0 "${rc}" "no command at all still prints help"
assert_contains "${out}" "Available Commands" "the help is the whole listing"

echo
echo "=== 14.2 version ==="
new_sandbox cli-version
out="$(gogq --version 2>&1)"; rc=$?
printf '%s\n' "${out}"
assert_rc 0 "${rc}" "--version"
assert_contains "${out}" "gog version" "it reports a version"
out="$(gogq -v 2>&1)"; rc=$?
assert_rc 0 "${rc}" "-v is the same flag"

echo
echo "=== 14.3 missing operands print usage ==="
new_sandbox cli-args
for c in add remove; do
  out="$(gogq "${c}" 2>&1)"; rc=$?
  printf '%s\n' "${out}" | head -1
  assert_rc 1 "${rc}" "gog ${c} with no path"
  assert_contains "${out}" "requires at least one path" "it says what it wanted"
  assert_contains "${out}" "Usage:" "and shows the usage"
done
out="$(gogq repository add 2>&1)"; rc=$?
assert_rc 1 "${rc}" "repository add with no name"
assert_contains "${out}" "requires a repository name and an optional URL" "it says what it wanted"
out="$(gogq repository remove 2>&1)"; rc=$?
assert_rc 1 "${rc}" "repository remove with no name"
assert_contains "${out}" "requires a repository name" "it says what it wanted"
# Usage belongs to a wrong invocation, not to a command that ran and failed
out="$(gogq repository remove missing 2>&1)"; rc=$?
assert_rc 1 "${rc}" "repository remove with an unknown name"
assert_not_contains "${out}" "Usage:" "a failure while running does not print usage"

echo
echo "=== 14.4 failures name the path gog was given ==="
new_sandbox cli-paths
gogq repository add dots >/dev/null 2>&1
out="$(gogq add ~/.missing 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "adding a path that does not exist"
assert_contains "${out}" "path does not exist: ${HOME}/.missing" "the path is named as it was given"
assert_not_contains "${out}" "lstat" "no system call is quoted at the user"

echo
echo "=== 14.5 a failed clone names the step, not git's wait status ==="
new_sandbox cli-clone
out="$(gogq repository add broken /nonexistent/repo.git 2>&1)"; rc=$?
printf '%s\n' "${out}" | tail -1
assert_rc 1 "${rc}" "cloning a repository that is not there"
assert_contains "${out}" "failed to clone /nonexistent/repo.git" "gog names its own step"
assert_not_contains "${out}" "exit status" "git's wait status is not restated"
# git is what explains a wait status, so a git that never ran has to be reported
out="$(PATH="${SANDBOX}/etc" "${GOG_BIN}" repository add nogit 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "adding a repository without git on the path"
assert_contains "${out}" "executable file not found" "the cause is kept when git never ran"

echo
echo "=== 14.6 a repository named by the environment ==="
new_sandbox cli-default-env
gogq repository add dots >/dev/null 2>&1
out="$(GOG_DEFAULT_REPOSITORY_NAME=missing "${GOG_BIN}" apply 2>&1)"; rc=$?
printf '%s\n' "${out}"
assert_rc 1 "${rc}" "a default repository that does not exist"
assert_contains "${out}" "GOG_DEFAULT_REPOSITORY_NAME" "the failure names where the name came from"

echo
echo "=== 14.7 conflicts are reported in the same form as every other failure ==="
new_sandbox cli-conflict
gogq repository add dots >/dev/null 2>&1
printf 'mine\n' >"${HOME}/.bashrc"
gogq add ~/.bashrc >/dev/null 2>&1
rm "${HOME}/.bashrc"; printf 'theirs\n' >"${HOME}/.bashrc"
out="$(gogq apply 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "applying over a file of the user's"
assert_contains "${out}" "Error: ${HOME}/.bashrc already exists" "the conflict names the path in the way"
assert_not_contains "${out}" "ERROR " "one error form is used throughout"

echo
echo "=== 14.8 repository names complete ==="
new_sandbox cli-completion
gogq repository add dots >/dev/null 2>&1
gogq repository add work >/dev/null 2>&1
out="$(gogq __complete repository remove "" 2>&1)"
printf '%s\n' "${out}" | head -2
assert_contains "${out}" "dots" "repository remove completes a name"
assert_contains "${out}" "work" "every repository is offered"
out="$(gogq __complete apply --repository "" 2>&1)"
assert_contains "${out}" "dots" "--repository completes a name"

echo
echo "=== 14.9 -r belongs to the commands that select a repository ==="
new_sandbox cli-repoflag
gogq repository add dots >/dev/null 2>&1
out="$(gogq repository list -r dots 2>&1)"; rc=$?
printf '%s\n' "${out}" | head -1
assert_rc 1 "${rc}" "repository list does not take -r"
assert_contains "${out}" "unknown shorthand flag" "it is refused rather than ignored"
out="$(gogq -r dots repository list 2>&1)"; rc=$?
assert_rc 1 "${rc}" "and writing it before the command does not smuggle it in"
# Cobra hands a flag written before the command to the command it belongs to,
# so the pre-command form goes on working wherever -r means something
out="$(gogq -r dots list 2>&1)"; rc=$?
assert_rc 0 "${rc}" "a command that owns -r takes it in either position"

echo
echo "=== 14.10 gog list ==="
new_sandbox cli-list
gogq repository add dots >/dev/null 2>&1
REPO="${HOME}/.local/share/gog/dots"
mkdir -p "${HOME}/.config/app"
printf 'a\n' >"${HOME}/.bashrc"
printf 'b\n' >"${HOME}/.config/app/conf"
printf 'c\n' >"${HOME}/.vimrc"
gogq add ~/.bashrc ~/.config/app ~/.vimrc >/dev/null 2>&1
printf 'skip\n' >"${REPO}/README.md"
rm "${HOME}/.vimrc"
rm "${HOME}/.bashrc"; printf 'mine\n' >"${HOME}/.bashrc"
out="$(gogq list 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "list succeeds"
assert_contains "${out}" "${HOME}/.bashrc" "a path it holds is listed"
assert_contains "${out}" "${HOME}/.config/app/conf" "a nested path is listed"
assert_not_contains "${out}" "README.md" "a file the repository keeps for itself is not listed"
assert_not_contains "${out}" "Linked:" "the listing is paths alone"
out="$(gogq list --status 2>&1)"
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_contains "${out}" "linked   ${HOME}/.config/app/conf" "a linked path says so"
assert_contains "${out}" "missing  ${HOME}/.vimrc" "a path with nothing at it is missing"
assert_contains "${out}" "conflict ${HOME}/.bashrc" "a path of the user's is a conflict"
# What the listing says has to be what applying then does
ln -s /nowhere "${HOME}/.vimrc"
out="$(gogq list --status 2>&1)"
assert_contains "${out}" "replace  ${HOME}/.vimrc" "a broken link is one applying replaces"
out="$(gogq apply 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_contains "${out}" "Linked: ${HOME}/.vimrc" "and applying does replace it"
assert_contains "${out}" "Error: ${HOME}/.bashrc already exists" "while the conflict is left alone"
assert_rc 1 "${rc}" "the run reports the conflict it listed"
printf 'x\n' >"${HOME}/.x"
gogq add ~/.x >/dev/null 2>&1
rm "${HOME}/.x"; printf 'x\n' >"${HOME}/.x"
out="$(gogq list --status 2>&1)"
assert_contains "${out}" "replace  ${HOME}/.x" "a copy of what the repository holds is replaceable"
out="$(GOG_IGNORE_FILES_REGEX='vimrc' gogq list 2>&1)"
assert_not_contains "${out}" ".vimrc" "GOG_IGNORE_FILES_REGEX leaves a path out"
out="$(gogq list -s -r dots 2>&1)"; rc=$?
assert_rc 0 "${rc}" "list takes -r"

echo
echo "=== 14.11 result lines name what happened ==="
new_sandbox cli-results
gogq repository add dots >/dev/null 2>&1
printf 'x\n' >"${HOME}/.bashrc"
out="$(gogq add ~/.bashrc 2>&1)"
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_contains "${out}" "Linked: ${HOME}/.bashrc -> " "add says what it linked"
out="$(gogq remove ~/.bashrc 2>&1)"
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_contains "${out}" "Restored: ${HOME}/.bashrc" "remove says what it restored"
out="$(gogq remove ~/.never 2>&1)"
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_contains "${out}" "Skipped: ${HOME}/.never (not tracked by dots)" "and what it passed over"

summary
