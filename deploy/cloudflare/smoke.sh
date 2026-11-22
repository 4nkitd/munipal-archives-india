#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
  echo "Usage: sh deploy/cloudflare/smoke.sh BASE_URL EXPECTED_INDEXED_AT" >&2
  exit 2
fi

base=$1
indexed=$2
curl -fsS --max-time 40 "$base/api/health" | jq -e 'select(.status=="ok")'
curl -fsS --max-time 15 "$base/api/stats" |
  jq -e --arg indexed "$indexed" 'select(.indexed_at==$indexed and .municipalities_indexed==289 and .embedding_dimensions==384)
    | {municipalities_indexed,total_documents,total_chunks,indexed_at}'
curl -fsS --max-time 15 "$base/?municipality=all" | grep -Fq 'Search all 289 municipal archives'
curl -fsS --max-time 15 -o /dev/null "$base/static/styles.css"

for mode in keyword vector hybrid; do
  curl -fsSG --max-time 20 "$base/api/search" --data-urlencode 'q=property tax assessment' \
    -d "municipality=chennai&mode=$mode&limit=5" |
    jq -e --arg mode "$mode" '.hits | map(select(.doc_id=="chennai-property-tax-assessment" and .municipality=="chennai"))
      | .[0] | select(.match_sources | index(if $mode=="keyword" then "keyword" else "vector" end)) | {doc_id, match_sources}'
done
curl -fsSG --max-time 15 "$base/api/search" --data-urlencode 'q=રામાપીર ચોકડી' \
  -d 'municipality=rajkot&mode=keyword&limit=5' |
  jq -e 'select(any(.hits[]; .doc_id=="rajkot-ward-offices")) | {municipality, total}'
curl -fsSG --max-time 20 "$base/api/ask" --data-urlencode 'q=Guwahati ward revenue zones' \
  -d 'municipality=guwahati&mode=hybrid&limit=4' |
  jq -e '.brief.citations[] | select(.doc_id=="guwahati-revenue-zones") | {doc_id, source_url}'
