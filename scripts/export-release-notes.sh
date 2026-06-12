#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 <release-tag> [output-file] [changelog-file]" >&2
}

if [ "$#" -lt 1 ] || [ "$#" -gt 3 ]; then
  usage
  exit 2
fi

release_tag="$1"
output_file="${2:-}"
changelog_file="${3:-CHANGELOG.md}"

if ! printf '%s\n' "${release_tag}" | grep -Eq '^v[0-9]{4}-[0-9]{2}-[0-9]{2}\.[0-9]{2}$'; then
  echo "Release tag is not valid: ${release_tag}" >&2
  exit 1
fi

if [ ! -f "${changelog_file}" ]; then
  echo "Changelog file does not exist: ${changelog_file}" >&2
  exit 1
fi

changelog_heading="## [${release_tag#v}]"

notes="$(
  awk -v heading="${changelog_heading}" '
    index($0, heading) == 1 {
      in_release = 1
      print
      next
    }

    in_release && /^## \[/ {
      exit
    }

    in_release {
      print
    }
  ' "${changelog_file}"
)"

if [ -z "${notes}" ]; then
  echo "No release notes found for ${release_tag} in ${changelog_file}" >&2
  exit 1
fi

if [ -n "${output_file}" ]; then
  printf '%s\n' "${notes}" > "${output_file}"
else
  printf '%s\n' "${notes}"
fi
