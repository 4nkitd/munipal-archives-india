import { DurableObject } from "cloudflare:workers";

const port = 8080;
const idleMs = 5 * 60 * 1000;

export default {
  fetch(request, env) {
    return env.ARCHIVE.getByName("archive").fetch(request);
  },
};

export class Archive extends DurableObject {
  starting;
  ready = false;
  monitoring;

  async fetch(request) {
    if (request.method !== "GET" && request.method !== "HEAD") {
      return new Response("Method not allowed", { status: 405, headers: { Allow: "GET, HEAD" } });
    }
    if (!this.ready || !this.ctx.container.running) {
      this.starting ??= this.start().finally(() => {
        this.starting = undefined;
      });
      try {
        await this.starting;
      } catch (error) {
        console.error("Archive startup failed", error);
        return new Response("Archive starting. Try again shortly.", { status: 503, headers: { "Retry-After": "5" } });
      }
    }

    const url = new URL(request.url);
    const forwarded = new Request(`http://container${url.pathname}${url.search}`, request);
    forwarded.headers.delete("host");
    try {
      return await this.ctx.container.getTcpPort(port).fetch(forwarded);
    } catch (error) {
      console.error("Archive forwarding failed", error);
      this.ready = false;
      return new Response("Archive temporarily unavailable", { status: 503, headers: { "Retry-After": "5" } });
    }
  }

  async start() {
    const container = this.ctx.container;
    if (!container.running) {
      container.start({ image: container.images.base, instance: "standard-2", enableInternet: false });
    }
    this.observeExit(container);
    await container.setInactivityTimeout(idleMs);
    const service = container.getTcpPort(port);
    const deadline = Date.now() + 30_000;
    while (Date.now() < deadline) {
      try {
        const response = await service.fetch("http://container/api/health", { signal: AbortSignal.timeout(1000) });
        await response.body?.cancel();
        if (response.ok) {
          this.ready = true;
          return;
        }
      } catch {
        if (!container.running) break;
      }
      await scheduler.wait(250);
    }
    throw new Error("Go server did not report healthy on port 8080");
  }

  observeExit(container) {
    if (this.monitoring) return;
    this.monitoring = container.monitor()
      .catch((error) => console.error("Archive container exited", error))
      .finally(() => {
        this.ready = false;
        this.monitoring = undefined;
      });
    this.ctx.waitUntil(this.monitoring);
  }
}
