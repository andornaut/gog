#!/usr/bin/env bash
# Build the binary under test and run every scenario script
set -u

LAB_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! (cd "${LAB_ROOT}/.." && go build -o "${LAB_ROOT}/bin/gog" .); then
  echo "build failed" >&2
  exit 1
fi

failed=0
for script in "${LAB_ROOT}"/[0-9][0-9]-*.sh; do
  name="$(basename "${script}")"
  result="$(bash "${script}" 2>&1 | grep -E '^RESULT')"
  printf '%-24s %s\n' "${name}" "${result:-no result reported}"
  case "${result}" in
    *", 0 failed") ;;
    *) failed=1 ;;
  esac
done

# Run one script on its own to see which assertion failed and what it printed
exit "${failed}"
