#!/bin/sh
set -eu

if [ "$#" -ne 4 ]; then
  echo "Usage: sh deploy/cloudflare/stage.sh PUBLIC_ARCHIVES DATA_DIR MODEL_DIR NEW_STAGE_DIR" >&2
  exit 2
fi

public=$(cd "$1" && pwd -P)
data=$(cd "$2" && pwd -P)
models=$(cd "$3" && pwd -P)
stage=$4
sh deploy/cloudflare/verify.sh "$public" "$data"
generation=$(cat "$data/CURRENT")
case "$generation" in
  generations/gen-[0-9]*) ;;
  *) echo "Invalid index generation: $generation" >&2; exit 1 ;;
esac
case "${generation#generations/gen-}" in
  *[!0-9]*) echo "Invalid index generation: $generation" >&2; exit 1 ;;
esac

source="$data/$generation"
model=BAAI_bge-small-en-v1.5
test -f "$source/COMPLETE"
test -s "$source/archive.db"
test -d "$source/vectors"
for asset in model.onnx tokenizer.json tokenizer_config.json special_tokens_map.json config.json vocab.txt; do
  test -s "$models/$model/$asset"
done
test "$(sqlite3 "$source/archive.db" "SELECT value FROM index_meta WHERE key='fts_tokenizer'")" = porter-unicode61-nfc-marks
test "$(sqlite3 "$source/archive.db" "SELECT value FROM index_meta WHERE key='embedding_model'")" = BAAI/bge-small-en-v1.5
test "$(sqlite3 "$source/archive.db" "SELECT value FROM index_meta WHERE key='embedding_dimensions'")" = 384

mkdir "$stage"
cp deploy/cloudflare/cloudflare.config.ts deploy/cloudflare/worker.js deploy/cloudflare/package.json \
  deploy/cloudflare/Dockerfile deploy/cloudflare/.dockerignore "$stage/"
mkdir -p "$stage/data/$generation" "$stage/models"
cp "$source/COMPLETE" "$stage/data/$generation/"
cp -R "$source/vectors" "$stage/data/$generation/"
mkdir "$stage/models/$model"
for asset in model.onnx tokenizer.json tokenizer_config.json special_tokens_map.json config.json vocab.txt; do
  cp -L "$models/$model/$asset" "$stage/models/$model/$asset"
  test -s "$stage/models/$model/$asset"
done
printf '%s\n' "$generation" > "$stage/data/CURRENT"
(
  cd "$stage/data/$generation"
  sqlite3 "$source/archive.db" '.backup archive.db'
  test "$(sqlite3 archive.db 'PRAGMA journal_mode=DELETE')" = delete
  test "$(sqlite3 -readonly archive.db 'PRAGMA integrity_check')" = ok
  chunks=$(sqlite3 -readonly archive.db 'SELECT count(*) FROM chunks')
  test "$chunks" -gt 0
  test "$chunks" -eq "$(sqlite3 -readonly archive.db 'SELECT count(*) FROM chunks_fts')"
  sqlite3 -readonly -header archive.db 'SELECT (SELECT count(*) FROM municipalities) AS municipalities,
     (SELECT count(*) FROM documents) AS documents, (SELECT count(*) FROM chunks) AS passages,
     (SELECT count(*) FROM chunks_fts) AS fts_rows'
  vectors=$(find vectors -type f -name '*.gob' | wc -l | tr -d ' ')
  test "$vectors" -eq "$((chunks + 1))"
)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1 go build -trimpath -o "$stage/archive" ./cmd/archive
CGO_ENABLED=0 go run ./cmd/public-corpus audit "$public" "$stage/data" > "$stage/records.tsv"
(
  cd "$stage"
  find archive data models cloudflare.config.ts worker.js Dockerfile .dockerignore package.json records.tsv \
    -type f -exec shasum -a 256 {} + | LC_ALL=C sort > release.sha256
)
printf 'Staged %s in %s\n' "$generation" "$stage"
