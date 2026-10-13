#!/usr/bin/env bash
# Checks commit subjects against Conventional Commits.
#
#   check-commit-messages.sh --file PATH      one message, as the commit-msg hook passes it
#   check-commit-messages.sh --range A..B     every commit in a range, as make check does
set -euo pipefail

types='build|chore|ci|docs|feat|fix|perf|refactor|revert|test'
pattern="^(${types})(\([a-z0-9][a-z0-9/-]*\))?!?: [^ ].*[^. ]$"
max_length=72

failures=0

check_subject() {
  local subject="$1" origin="$2"

  # Git writes these itself.
  case "$subject" in
    "Merge "* | "Revert \""* | "fixup! "* | "squash! "*) return 0 ;;
  esac

  if ! [[ "$subject" =~ $pattern ]]; then
    printf '%s: not a Conventional Commit: %s\n' "$origin" "$subject" >&2
    failures=$((failures + 1))
  elif [ "${#subject}" -gt "$max_length" ]; then
    printf '%s: subject is %d characters, the limit is %d: %s\n' "$origin" "${#subject}" "$max_length" "$subject" >&2
    failures=$((failures + 1))
  fi
}

case "${1:-}" in
  --file)
    check_subject "$(head -n 1 "$2")" "commit message"
    ;;
  --range)
    commits=$(git log --format='%h%x09%s' "$2")
    while IFS=$'\t' read -r hash subject; do
      [ -n "$hash" ] && check_subject "$subject" "$hash"
    done <<<"$commits"
    ;;
  *)
    sed -n '2,5p' "$0" | sed 's/^# \{0,1\}//' >&2
    exit 2
    ;;
esac

if [ "$failures" -gt 0 ]; then
  printf '\nExpected: <type>(<scope>): <summary>, where type is one of: %s\n' "${types//|/, }" >&2
  printf 'See docs/working-on-the-code.md.\n' >&2
  exit 1
fi
