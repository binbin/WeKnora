#!/usr/bin/env bash
# Fail when migration version prefixes collide inside a directory.
#
# Sequential (-seq) numbers are allocated from the *local* max+1, so two
# long-lived branches can both create 000072_*. CI must catch the merge
# that would land duplicate versions — golang-migrate only errors at
# runtime once both files exist in the tree.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FAILED=0

check_dir() {
  local dir="$1"
  local label="$2"
  if [[ ! -d "${ROOT}/${dir}" ]]; then
    echo "skip ${label}: ${dir} missing"
    return 0
  fi

  local ups
  ups="$(find "${ROOT}/${dir}" -maxdepth 1 -type f -name '*.up.sql' | sort)"
  if [[ -z "${ups}" ]]; then
    echo "ok   ${label}: no .up.sql files"
    return 0
  fi

  local versions=()
  local file base version
  while IFS= read -r file; do
    [[ -z "${file}" ]] && continue
    base="$(basename "${file}")"
    if [[ ! "${base}" =~ ^([0-9]{6})_.+\.up\.sql$ ]]; then
      echo "FAIL ${label}: invalid name '${base}' (want NNNNNN_name.up.sql)"
      FAILED=1
      continue
    fi
    version="${BASH_REMATCH[1]}"
    versions+=("${version}")

    local down="${file%.up.sql}.down.sql"
    if [[ ! -f "${down}" ]]; then
      echo "FAIL ${label}: missing pair for ${base} (expected $(basename "${down}"))"
      FAILED=1
    fi
  done <<< "${ups}"

  # Duplicate version prefixes (the parallel-branch collision case).
  local dupes
  dupes="$(printf '%s\n' "${versions[@]}" | sort | uniq -d)"
  if [[ -n "${dupes}" ]]; then
    echo "FAIL ${label}: duplicate migration version(s):"
    while IFS= read -r version; do
      [[ -z "${version}" ]] && continue
      echo "  ${version}:"
      find "${ROOT}/${dir}" -maxdepth 1 -type f -name "${version}_*.sql" -printf '    %f\n' \
        2>/dev/null || find "${ROOT}/${dir}" -maxdepth 1 -type f -name "${version}_*.sql" \
        | while IFS= read -r hit; do echo "    $(basename "${hit}")"; done
    done <<< "${dupes}"
    FAILED=1
  else
    local count="${#versions[@]}"
    local max
    max="$(printf '%s\n' "${versions[@]}" | sort | tail -1)"
    echo "ok   ${label}: ${count} up migration(s), latest=${max}"
  fi
}

check_dir "migrations/versioned" "postgres (migrations/versioned)"
check_dir "migrations/sqlite" "sqlite (migrations/sqlite)"

if [[ "${FAILED}" -ne 0 ]]; then
  cat <<'EOF'

Migration version check failed.

Sequential numbers collide when two branches both run `make migrate-create`
against an older max. Fix by:

  1. git fetch origin && git rebase origin/main
  2. Renaming your new files to max(main)+1 (and updating NOTICE comments)
  3. Re-running: make check-migrations

Do NOT force-merge duplicate NNNNNN_ prefixes — golang-migrate will refuse
to load the source directory.
EOF
  exit 1
fi

echo "All migration version prefixes are unique."
