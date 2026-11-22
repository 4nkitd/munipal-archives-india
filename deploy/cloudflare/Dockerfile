FROM docker.io/library/debian:trixie-slim@sha256:a72b4433de1cf7d6bae0c4cee74d0547f58601a643b3375f070c51600d63b6b2
RUN groupadd --gid 10001 archive && useradd --uid 10001 --gid 10001 --no-create-home archive
WORKDIR /app
COPY --chown=10001:10001 archive /app/archive
COPY --chown=10001:10001 data/ /app/data/
COPY --chown=10001:10001 models/ /app/models/
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/app/archive", "serve", "--addr=0.0.0.0:8080", "--data=/app/data", "--models=/app/models", "--auto-index=false"]
