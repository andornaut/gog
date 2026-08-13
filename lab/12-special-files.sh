#!/usr/bin/env bash
# pipes, sockets, devices and hard links
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# Every gog call here is bounded: the point of the capability is that some of
# these never return. 124 is timeout's exit status.
gogt() { timeout 10 "${GOG_BIN}" "$@"; }

# Bound from inside the directory: an AF_UNIX path is limited to 108 bytes and
# a sandbox path is longer than that
make_socket() {
  python3 -c 'import os,socket,sys; os.chdir(os.path.dirname(sys.argv[1])); s=socket.socket(socket.AF_UNIX); s.bind(os.path.basename(sys.argv[1]))' "$1"
}

setup() {
  new_sandbox "$1"
  gogq repository add dots >/dev/null 2>&1
  REPO="${HOME}/.local/share/gog/dots"
}

echo "=== 12.1 a named pipe inside a tree ==="
setup special-fifo-tree
mkdir -p "${HOME}/.gnupg"
printf 'conf\n' >"${HOME}/.gnupg/gpg.conf"
mkfifo "${HOME}/.gnupg/S.gpg-agent"
out="$(gogt add ~/.gnupg 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "add of a tree holding a named pipe returns"
assert_contains "${out}" "S.gpg-agent" "the pipe is named in the output"
assert_not_exists "${REPO}/\$HOME/.gnupg/S.gpg-agent"
assert_exists "${REPO}/\$HOME/.gnupg/gpg.conf"
assert_symlink_to "${HOME}/.gnupg/gpg.conf" "${REPO}/\$HOME/.gnupg/gpg.conf"
echo "--- the pipe itself is untouched:"
[ -p "${HOME}/.gnupg/S.gpg-agent" ] && _ok "the pipe is left as a pipe" || _bad "the pipe is left as a pipe"

echo
echo "=== 12.2 a named pipe given directly to add ==="
setup special-fifo-direct
mkfifo "${HOME}/.pipe"
out="$(gogt add ~/.pipe 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "add of a named pipe is refused"
assert_contains "${out}" "named pipe" "the refusal names the kind"
assert_not_exists "${REPO}/\$HOME/.pipe"
[ -p "${HOME}/.pipe" ] && _ok "the pipe is left as a pipe" || _bad "the pipe is left as a pipe"

echo
echo "=== 12.3 a unix socket inside a tree ==="
setup special-socket
mkdir -p "${HOME}/.tmux"
printf 'conf\n' >"${HOME}/.tmux/tmux.conf"
make_socket "${HOME}/.tmux/default"
out="$(gogt add ~/.tmux 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "add of a tree holding a socket returns"
assert_contains "${out}" "default" "the socket is named in the output"
assert_not_exists "${REPO}/\$HOME/.tmux/default"
assert_symlink_to "${HOME}/.tmux/tmux.conf" "${REPO}/\$HOME/.tmux/tmux.conf"
[ -S "${HOME}/.tmux/default" ] && _ok "the socket is left as a socket" || _bad "the socket is left as a socket"

echo
echo "=== 12.4 a unix socket given directly to add ==="
setup special-socket-direct
make_socket "${HOME}/.sock"
out="$(gogt add ~/.sock 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "add of a socket is refused"
assert_contains "${out}" "socket" "the refusal names the kind"
assert_not_exists "${REPO}/\$HOME/.sock"

echo
echo "=== 12.5 a character device given directly to add ==="
setup special-device
out="$(gogt add /dev/null 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "add of a device node is refused"
assert_contains "${out}" "character device" "the refusal names the kind"
assert_not_exists "${REPO}/dev/null"
echo "--- /dev/null is still a device:"
[ -c /dev/null ] && _ok "/dev/null is untouched" || _bad "/dev/null is untouched"

echo
echo "=== 12.6 a device that never reaches end of file ==="
setup special-device-endless
echo "--- bounded by ulimit, so that a refusal and a runaway copy are told apart:"
out="$( (ulimit -f 2048; gogt add /dev/zero 2>&1) )"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|" | head -3
assert_rc 1 "${rc}" "add of an endless device is refused"
assert_contains "${out}" "character device" "the refusal names the kind"
assert_not_contains "${out}" "file too large" "nothing was read from it"
assert_not_exists "${REPO}/dev/zero"

echo
echo "=== 12.7 hard links inside a tree ==="
setup special-hardlink
mkdir -p "${HOME}/.config/app"
printf 'shared\n' >"${HOME}/.config/app/one.conf"
ln "${HOME}/.config/app/one.conf" "${HOME}/.config/app/two.conf"
echo "--- before: $(stat -c %h "${HOME}/.config/app/one.conf") links to one inode"
out="$(gogt add ~/.config 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "add of a tree holding hard links"
assert_exists "${REPO}/\$HOME/.config/app/one.conf"
assert_exists "${REPO}/\$HOME/.config/app/two.conf"
echo "--- in the repository: $(stat -c %h "${REPO}/\$HOME/.config/app/one.conf") links to one inode"
if [ "$(stat -c %i "${REPO}/\$HOME/.config/app/one.conf")" = "$(stat -c %i "${REPO}/\$HOME/.config/app/two.conf")" ]; then
  echo "    the hard link survived"
else
  echo "    the hard link became two independent files"
fi
printf 'edited\n' >"${REPO}/\$HOME/.config/app/one.conf"
echo "--- after editing one of them, the other holds: $(cat "${REPO}/\$HOME/.config/app/two.conf")"

echo
echo "=== 12.8 a regular file where a directory must be created ==="
setup special-file-as-dir
mkdir -p "${HOME}/.config/app"
printf 'a\n' >"${HOME}/.config/app/a.conf"
gogq add ~/.config >/dev/null 2>&1
gogq git commit -q -m init >/dev/null 2>&1
rm -rf "${HOME}/.config"
printf 'not a directory\n' >"${HOME}/.config"
out="$(gogt apply 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "apply fails when a directory cannot replace a file"
assert_contains "${out}" "failed to create directory" "the failure says what could not be done"
assert_regular "${HOME}/.config"
assert_file_contents "${HOME}/.config" "not a directory"

echo
echo "=== 12.9 removing a path the user replaced with a pipe ==="
setup special-remove-fifo
printf 'x\n' >"${HOME}/.bashrc"
gogq add ~/.bashrc >/dev/null 2>&1
gogq git commit -q -m init >/dev/null 2>&1
rm "${HOME}/.bashrc"
mkfifo "${HOME}/.bashrc"
out="$(gogt remove ~/.bashrc 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 0 "${rc}" "remove returns when the path is now a pipe"
assert_not_exists "${REPO}/\$HOME/.bashrc"
[ -p "${HOME}/.bashrc" ] && _ok "the user's pipe is left alone" || _bad "the user's pipe is left alone"

echo
echo "=== 12.10 applying over a pipe the user left in the way ==="
setup special-apply-fifo
printf 'x\n' >"${HOME}/.bashrc"
gogq add ~/.bashrc >/dev/null 2>&1
gogq git commit -q -m init >/dev/null 2>&1
rm "${HOME}/.bashrc"
mkfifo "${HOME}/.bashrc"
out="$(gogt apply 2>&1)"; rc=$?
printf '%s\n' "${out}" | sed "s|${HOME}|~|"
assert_rc 1 "${rc}" "apply refuses to replace the pipe"
assert_contains "${out}" "already exists" "the refusal names the conflict"
[ -p "${HOME}/.bashrc" ] && _ok "the user's pipe is left alone" || _bad "the user's pipe is left alone"

summary
