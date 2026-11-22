# Nagar Archive

Nagar Archive is a search-first, plain-text civic archive built in Go. The original collection covers Haryana's 11 municipal corporations and 10 neighboring councils and committees. See the [northwest coverage plan](docs/NORTHWEST_COVERAGE.md) for the regional expansion boundary and evidence criteria.

The [municipal archive directories](archives/) hold the source corpus; `/api/stats` reports what the running index contains. Records preserve source URLs, dates, jurisdictions, and citations. They are archival references, not a substitute for current official notices.

No Node.js, Python, Docker, or external API keys are required. Embeddings run locally. The first indexing run downloads the public embedding model; later runs reuse the model cache.

## Architecture

```text
archives/<municipality>/*.md
  (including category subdirectories)
                  |
                  v
        Pure-Go Indexer
        +-----------------------------------------------+
        | YAML metadata + section-aware Markdown chunks  |
        | hugot: BAAI/bge-small-en-v1.5, 384 dimensions    |
        |                  |                            |
        |                  v                            |
        | chromem-go Vector DB + modernc.org/sqlite      |
        |                        FTS5 BM25              |
        | Paired stores in atomic generation dirs       |
        +-----------------------------------------------+
                  |
                  v
        Hybrid RRF Search + Grounded Synthesizer
                  |
                  v
        Embedded Go html/template + JSON REST API
                  |
                  v
              Single binary
```

Hybrid search combines vector and keyword rankings with reciprocal rank fusion. The synthesizer builds answers from retrieved records with citations, without a hosted language model. Indexing publishes a complete SQLite and vector-store generation through an atomic `data/CURRENT` pointer.

## Build and run

Use Go 1.27.3 or later. Run commands from the repository root.

```sh
go build -o ./bin/archive ./cmd/archive
./bin/archive index
./bin/archive serve
```

Open <http://localhost:8080>. The server embeds its HTML templates and static assets. It reads the index from `data/` and caches the model under `.cache/hugot-models/`. `serve` automatically indexes the archive if no complete index exists.

Rebuild an older index with `./bin/archive index` to pick up the northwest collections and the updated Hindi and Gujarati keyword tokenizer. Keyword search uses the archived spelling. The local embedding model is English-oriented.

Query from another terminal:

```sh
./bin/archive query "Who handles property tax in Manesar?"
./bin/archive query -municipality all -mode hybrid -limit 10 "birth certificate"
./bin/archive query -municipality gurugram -mode keyword "property tax"
go test ./...
```

Flags must precede the question. Query modes are `hybrid`, `vector`, and `keyword`. CLI queries default to `manesar`, `hybrid`, and 5 results, with a maximum of 50.

All commands accept `-data` and `-models`. `index` and `serve` accept `-archives`. To change the listen address or require an existing index:

```sh
./bin/archive serve -addr 127.0.0.1:8080 -auto-index=false
```

## Original Haryana and NCR collection

Jurisdiction identifies the local body and its district, not the entire district.

| Municipality | Slug | Jurisdiction |
| --- | --- | --- |
| Municipal Corporation Manesar | `manesar` | Manesar municipal area, Gurugram district, Haryana |
| Municipal Corporation of Gurugram | `gurugram` | Gurugram municipal area, Gurugram district, Haryana |
| Municipal Corporation Faridabad | `faridabad` | Faridabad municipal area, Faridabad district, Haryana |
| Municipal Corporation Ambala | `ambala` | Ambala City, Ambala district, Haryana; excludes Ambala Sadar and the cantonment |
| Municipal Corporation Hisar | `hisar` | Hisar municipal area, Hisar district, Haryana |
| Municipal Corporation Karnal | `karnal` | Karnal municipal area, Karnal district, Haryana |
| Municipal Corporation Panchkula | `panchkula` | Panchkula municipal area, Panchkula district, Haryana |
| Municipal Corporation Panipat | `panipat` | Panipat municipal area, Panipat district, Haryana |
| Municipal Corporation Rohtak | `rohtak` | Rohtak municipal area, Rohtak district, Haryana |
| Municipal Corporation Sonipat | `sonipat` | Sonipat municipal area, Sonipat district, Haryana |
| Municipal Corporation Yamunanagar-Jagadhri | `yamunanagar` | Yamunanagar and Jagadhri municipal area, Yamunanagar district, Haryana |
| Municipal Council Bahadurgarh | `bahadurgarh` | Bahadurgarh municipal area, Jhajjar district, Haryana |
| Municipal Council Bhiwadi | `bhiwadi` | Bhiwadi municipal area, Khairthal-Tijara district, Rajasthan |
| Municipal Committee Dharuhera | `dharuhera` | Dharuhera municipal area, Rewari district, Haryana |
| Municipal Committee Farrukhnagar | `farrukhnagar` | Farrukhnagar municipal area, Gurugram district, Haryana |
| Municipal Council Jhajjar | `jhajjar` | Jhajjar municipal area, Jhajjar district, Haryana |
| Municipal Council Nuh | `nuh` | Nuh municipal area, Nuh district, Haryana |
| Municipal Council Palwal | `palwal` | Palwal municipal area, Palwal district, Haryana |
| Municipal Council Pataudi-Jatoli Mandi | `pataudi-jatoli-mandi` | Merged Pataudi, Haily Mandi, Jatoli and included villages, Gurugram district, Haryana |
| Municipal Council Rewari | `rewari` | Rewari municipal area, Rewari district, Haryana |
| Municipal Council Sohna | `sohna` | Sohna municipal area, Gurugram district, Haryana |

## REST API

All endpoints use `GET` and return JSON. The default base URL is `http://localhost:8080`.

| Endpoint | Parameters | Response |
| --- | --- | --- |
| `/api/health` | None | `status` and `engine` identifiers |
| `/api/stats` | None | Archive counts and engine telemetry |
| `/api/municipalities` | None | `municipalities` array of profiles |
| `/api/search` | Required `q`; search parameters below | Ranked `hits`, facets, detected entities, query metadata and timing |
| `/api/ask` | Required `q`; search parameters below | Grounded brief with citations and retrieved hits |
| `/api/suggest` | Required `q`; optional `municipality`, `limit` | `suggestions` array |
| `/api/documents` | Optional `municipality`, `category`, `doc_type`, `ward` | `documents` array; no query text required |
| `/api/documents/{municipality}/{id}` | Municipality slug and record ID in path | Full record, rendered `html_content`, headings and related records |
| `/api/wards/{municipality}` | Municipality slug in path | `municipality` and `wards` array |
| `/api/officers/{municipality}` | Municipality slug in path | `municipality` and `officers` array |

Search parameters for `/api/search` and `/api/ask`:

| Parameter | Meaning | Default |
| --- | --- | --- |
| `q` | Nonempty query, at most 2,048 characters | Required |
| `municipality` | Indexed slug or `all` for cross-municipality search | `manesar` |
| `mode` | `hybrid`, `vector`, or `keyword` | `hybrid` |
| `limit` | Integer from 1 to 50 | `10` |
| `category` | Archive category slug | No category filter |
| `doc_type` | Document type from the archive schema | No document-type filter |
| `ward` | Positive ward number | No ward filter |

`/api/suggest` uses the same municipality and limit defaults. `/api/documents` defaults to `manesar` and returns all matching documents, without limit-based pagination. Invalid parameters return HTTP 400, missing records return HTTP 404, and internal failures return HTTP 500. Error bodies have the form `{"error":"message"}`.

```sh
curl http://localhost:8080/api/health
curl http://localhost:8080/api/stats
curl 'http://localhost:8080/api/search?q=property%20tax&municipality=all&mode=hybrid&limit=5'
curl 'http://localhost:8080/api/ask?q=birth%20certificate&municipality=manesar'
curl 'http://localhost:8080/api/suggest?q=ward&municipality=manesar&limit=5'
curl 'http://localhost:8080/api/documents?municipality=manesar&category=citizen-services'
curl http://localhost:8080/api/wards/manesar
curl http://localhost:8080/api/officers/manesar
```

## Add a municipality

1. Create `archives/<slug>/municipality.json`. Use an existing municipal profile as the field reference, set a unique `slug`, and supply the official name, state, district, website, and verified jurisdiction details.
2. Add `.md` records in that directory or category subdirectories. Follow [the archive schema](docs/ARCHIVE_SCHEMA.md) for YAML frontmatter, source URLs, verification dates, headings, and record categories. Set each record's `municipality` to the new slug and use globally unique record IDs.
3. Add `wards.json` and `officers.json` when verified structured directories are available. Follow the existing files' formats.
4. Rebuild the index:

```sh
./bin/archive index
```

Restart a running server to load the newly published generation. No municipality-specific Go registration is required.

Store parsed text, not source PDFs. Link the original document through `source_url` and identify scanned sources or missing information rather than inventing facts.

The [interface redesign](docs/UI_REDESIGN_SPEC.md) describes the search, document reader, ward lookup, and archive index views. The [original design](docs/DESIGN_SPEC.md) is retained for reference.
