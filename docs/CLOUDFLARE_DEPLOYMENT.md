# Cloudflare deployment

Deploy only after [nationwide coverage](INDIA_COVERAGE.md) passes its checks. The Cloudflare Worker forwards to one named container. The application in the container is the same `CGO_ENABLED=0` Go binary, with its templates and assets embedded. The image also carries one SQLite and chromem generation plus the exact Hugot model used to build it. It does not download the model or rebuild the index at startup.

## Build the candidate

Run this from the repository root. Use the national candidate's data directory, not the currently served regional generation. `STAGE` must be a new directory outside the checkout. The staging script uses SQLite's backup command so it includes committed data still in the source WAL without copying that WAL into the image.

```sh
./bin/archive index --archives archives --data /path/to/national-data --models .cache/hugot-models
sh deploy/cloudflare/stage.sh /path/to/national-data .cache/hugot-models "$STAGE"
```

On a Linux builder, build the staged `linux/amd64` image and run it with outbound access disabled. The image runs as a non-root user. The generation directory must be writable for SQLite's WAL. Benchmark cold startup and concurrent hybrid requests at the container's configured CPU and memory before deciding on the instance size. Confirm that `/api/stats`, `/api/municipalities`, and queries return the expected generation and records. Check the public release for contact details that are not designated office or service channels.

## Publish and verify

Cloudflare Containers require Workers Paid and a working Docker builder. Use the installed `cf` CLI. Check the deployment account and whether `nagar-archive` already exists before creating or replacing anything. Do not deploy the sandbox Worker or upload the repository.

```sh
cd "$STAGE"
cf deploy --dry-run
cf deploy
```

Use the assigned HTTPS hostname for a test from outside the Cloudflare CLI. Check the home page, a static asset, exact municipality set, statistics, keyword and vector search, and citations. Repeat a search after the idle timeout to test a cold start. After an update, make sure the running container loaded the new generation rather than a previous image. Keep the prior Worker version and image digest for rollback.
