# Cloudflare deployment

Deploy only after [nationwide coverage](INDIA_COVERAGE.md) passes its checks. The Cloudflare Worker forwards to one named container. The application in the container is the same `CGO_ENABLED=0` Go binary, with its templates and assets embedded. The image also carries one SQLite and chromem generation plus the exact Hugot model used to build it. It does not download the model or rebuild the index at startup.

## Build the candidate

Run this from the repository root. Set `RELEASE` to a new empty directory outside the checkout and build `bin/archive` from the release source. The public export removes direct contact fields and screens prose while keeping citations, municipal records and directory roles. The screen is not a guarantee: review the export for contact details and material text loss before indexing it. Do not use the private, currently served index. The staging script uses SQLite's backup command to include committed data still in the source WAL without copying that WAL into the image.

Short public service codes can remain in civic instructions. The public ward directory preserves councillor names only where there is a matching mapped ward. For details omitted from the public version, follow the record's citation.

```sh
go run ./cmd/public-corpus archives "$RELEASE/public"
```

```sh
./bin/archive index --archives "$RELEASE/public" --data "$RELEASE/data" --models .cache/hugot-models &&
sh deploy/cloudflare/verify.sh "$RELEASE/public" "$RELEASE/data" &&
sh deploy/cloudflare/stage.sh "$RELEASE/public" "$RELEASE/data" .cache/hugot-models "$RELEASE/worker" &&
sh deploy/cloudflare/verify.sh "$RELEASE/public" "$RELEASE/worker/data"
```

On a Linux builder, build the staged `linux/amd64` image and run it with outbound access disabled. The image runs as a non-root user. The generation directory must be writable for SQLite's WAL. Benchmark cold startup and concurrent hybrid requests at the container's configured CPU and memory before deciding on the instance size. Confirm that `/api/stats`, `/api/municipalities`, and queries return the expected generation and records. Check the public release for contact details that are not designated office or service channels.

Before publication, send a cold `/api/health` request through a local Worker with a container runtime, then a second request for an actual search through the same binding. The direct container check and deployment dry run do not exercise the Worker proxy.

## Publish and verify

Cloudflare Containers require Workers Paid and a working Docker builder. Use the installed `cf` CLI. The staged `package.json` declares Cloudflare's Worker build tools only. Neither those tools nor a JavaScript web server run in the image. Check the deployment account and whether `nagar-archive` already exists before creating or replacing anything. Do not deploy the sandbox Worker or upload the repository.

```sh
cd "$RELEASE/worker" &&
bun install &&
cf deploy --dry-run &&
cf deploy
```

Use the assigned HTTPS hostname for a test from outside the Cloudflare CLI. From the repository root, set `PUBLIC_URL` to that hostname and `INDEXED_AT` to the audited candidate's `/api/stats` timestamp:

```sh
sh deploy/cloudflare/smoke.sh "$PUBLIC_URL" "$INDEXED_AT"
sh deploy/cloudflare/verify.sh "$RELEASE/public" "$PUBLIC_URL" "$RELEASE/worker/records.tsv"
```

Repeat a search after the idle timeout to test a cold start. The timestamp check catches a container still running a previous generation. Keep the prior Worker version and image digest for rollback.
