FROM scratch
WORKDIR /app
COPY --chown=10001:10001 archive /app/archive
COPY --chown=10001:10001 data/ /app/data/
COPY --chown=10001:10001 models/ /app/models/
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/app/archive", "serve", "--addr=0.0.0.0:8080", "--data=/app/data", "--models=/app/models", "--auto-index=false"]
