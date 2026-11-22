import { bindings, defineConfig, defineContainer, defineWorker, exports as cfExports } from "cf/config";

const container = defineContainer({
  name: "nagar-archive",
  schedulingPolicy: "durable-object",
  images: { base: { dockerfile: "./Dockerfile", buildContext: "." } },
});

const worker = defineWorker({
  name: "nagar-archive",
  entrypoint: "./worker.js",
  compatibilityDate: "2026-11-22",
  workersDev: true,
  previewUrls: false,
  exports: { Archive: cfExports.durableObject({ storage: "sqlite", container }) },
  env: { ARCHIVE: bindings.durableObject({ worker: "nagar-archive", exportName: "Archive" }) },
});

export default defineConfig({ worker, containers: [container] });
