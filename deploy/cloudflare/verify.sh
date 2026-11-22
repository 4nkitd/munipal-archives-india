#!/bin/sh
set -eu

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
  echo "Usage: sh deploy/cloudflare/verify.sh ARCHIVES DATA_DIR_OR_HTTP_URL [RECORDS_TSV]" >&2
  exit 2
fi

root=$1
target=$2
export LC_ALL=C
scratch=$(mktemp -d "${TMPDIR:-/tmp}/nagar-verify.XXXXXX")
trap 'rm -f "$scratch/expected" "$scratch/actual" "$scratch/source" "$scratch/indexed" "$scratch/http-records" "$scratch/api-profiles" "$scratch/source-contacts" "$scratch/live-contacts"; rmdir "$scratch"' EXIT

{
  awk -F '\t' 'NR > 1 && NF == 3 {n = split($3, slugs, " "); for (i = 1; i <= n; i++) print slugs[i] "\t" $1}' docs/india-roster.tsv
  awk -F '\t' 'NR > 1 && NF == 2 {n = split($2, slugs, " "); for (i = 1; i <= n; i++) print slugs[i] "\t" $1}' docs/retained-councils.tsv
} | sort > "$scratch/expected"
test "$(wc -l < docs/india-roster.tsv | tr -d ' ')" -eq 37
test "$(wc -l < "$scratch/expected" | tr -d ' ')" -eq 289
test "$(cut -f1 "$scratch/expected" | uniq | wc -l | tr -d ' ')" -eq 289

for profile in "$root"/*/municipality.json; do
  jq -er '[.slug, .state] | @tsv' "$profile"
done | sort > "$scratch/source"
diff -u "$scratch/expected" "$scratch/source"

case "$target" in
  http://*|https://*)
    manifest=${3:?The HTTP check requires the staged records.tsv}
    test -s "$manifest"
    curl -fsS --max-time 40 -o "$scratch/api-profiles" "$target/api/municipalities"
    jq -er '.municipalities[] | [.slug, .state] | @tsv' "$scratch/api-profiles" | sort > "$scratch/actual"
    diff -u "$scratch/expected" "$scratch/actual"
    legacy=$(jq -r '.municipalities[] | . as $profile |
      ("helpdeskPhone", "tollFree", "whatsapp", "email") as $key |
      select(($profile[$key] // "") != "") | [$profile.slug, $key] | @tsv' "$scratch/api-profiles")
    if [ -n "$legacy" ]; then
      printf 'Unexpected legacy contact fields (municipality and field):\n%s\n' "$legacy" >&2
      exit 1
    fi
    while IFS="$(printf '\t')" read -r slug state; do
      contacts="$root/$slug/public-contacts.json"
      if [ -f "$contacts" ]; then
        jq -ceS --arg slug "$slug" '[$slug, .]' "$contacts"
      else
        jq -cn --arg slug "$slug" '[$slug, []]'
      fi
    done < "$scratch/expected" > "$scratch/source-contacts"
    jq -ceS '.municipalities | sort_by(.slug)[] | [.slug, (.public_contacts // [])]' \
      "$scratch/api-profiles" > "$scratch/live-contacts"
    if ! cmp -s "$scratch/source-contacts" "$scratch/live-contacts"; then
      echo "Published contacts differ from the approved files" >&2
      exit 1
    fi
    jq -e '.municipalities | {
      municipalities_with_phones: map(select(any(.public_contacts[]?; .number))) | length,
      municipalities_with_emails: map(select(any(.public_contacts[]?; .email))) | length,
      public_contact_rows: map((.public_contacts // []) | length) | add
    }' "$scratch/api-profiles"
    while IFS="$(printf '\t')" read -r slug state; do
      curl -fsS --max-time 15 "$target/api/documents?municipality=$slug" |
        jq -er --arg slug "$slug" '.documents[] | select(.municipality == $slug and .chunk_count > 0) | [.municipality, .id] | @tsv'
    done < "$scratch/expected" > "$scratch/http-records"
    sort "$manifest" > "$scratch/source"
    sort "$scratch/http-records" > "$scratch/indexed"
    diff -u "$scratch/source" "$scratch/indexed"
    records=$(wc -l < "$scratch/source" | tr -d ' ')
    curl -fsS --max-time 15 "$target/api/stats" |
      jq -e --argjson records "$records" 'select(.municipalities_indexed == 289 and .total_documents == $records and .embedding_dimensions == 384)'
    ;;
  *)
    generation=$(cat "$target/CURRENT")
    case "$generation" in
      generations/gen-[0-9]*) ;;
      *) echo "Invalid index generation" >&2; exit 1 ;;
    esac
    case "${generation#generations/gen-}" in *[!0-9]*) exit 1 ;; esac
    dir="$target/$generation"
    test -f "$dir/COMPLETE"
    test -s "$dir/archive.db"
    test -d "$dir/vectors"
    db="$dir/archive.db"
    test "$(sqlite3 "$db" 'PRAGMA integrity_check')" = ok
    sqlite3 -separator "$(printf '\t')" "$db" \
      "SELECT slug, json_extract(profile_json, '\$.state') FROM municipalities ORDER BY slug;" > "$scratch/actual"
    diff -u "$scratch/expected" "$scratch/actual"
    test "$(sqlite3 "$db" 'SELECT count(DISTINCT municipality) FROM documents WHERE chunk_count > 0;')" -eq 289
    (cd "$root" && find . -type f -iname '*.md' | sed 's|^\./||' | sort) > "$scratch/source"
    sqlite3 "$db" 'SELECT file_path FROM documents WHERE chunk_count > 0;' | sort > "$scratch/indexed"
    diff -u "$scratch/source" "$scratch/indexed"
    sqlite3 -header "$db" 'SELECT (SELECT count(*) FROM municipalities) AS bodies,
      (SELECT count(*) FROM documents) AS records, (SELECT count(*) FROM chunks) AS passages;'
    ;;
esac
printf '%s\n' 'Verified all 279 corporations and 10 retained councils/committees.'
