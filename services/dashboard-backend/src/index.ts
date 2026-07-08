import { Hono } from "hono";
import { cors } from "hono/cors";
import { loadConfig } from "./config";
import { createHealthRoutes } from "./routes/health";

const config = loadConfig();

const app = new Hono();

app.use(
	"/api/*",
	cors({
		origin: config.corsOrigin,
		allowMethods: ["GET", "POST", "OPTIONS"],
		allowHeaders: ["Content-Type", "Authorization"],
		exposeHeaders: ["Content-Type", "Cache-Control"],
		credentials: true,
	}),
);

app.route("/", createHealthRoutes({ controlUrl: config.controlUrl }));

app.get("/api/v1/dashboard", async (c) => {
	const res = await fetch(`${config.controlUrl}/api/v1/dashboard`, {
		headers: { Authorization: `Bearer ${config.controlApiToken}` },
	});
	if (!res.ok) {
		return c.json({ error: "upstream error", status: res.status }, 502);
	}
	const body = await res.json();
	return c.json(body);
});

app.get("/api/v1/events", (c) => {
	const { readable, writable } = new TransformStream();
	const writer = writable.getWriter();
	const encoder = new TextEncoder();

	c.header("Content-Type", "text/event-stream");
	c.header("Cache-Control", "no-cache");
	c.header("Connection", "keep-alive");

	const interval = setInterval(async () => {
		try {
			const res = await fetch(`${config.controlUrl}/api/v1/dashboard`, {
				headers: { Authorization: `Bearer ${config.controlApiToken}` },
			});
			if (res.ok) {
				const data = await res.json();
				await writer.write(encoder.encode(`data: ${JSON.stringify(data)}\n\n`));
			}
		} catch {
			// connection lost — keep-alive will retry
		}
	}, 5000);

	c.req.raw.signal.addEventListener("abort", () => {
		clearInterval(interval);
		writer.close();
	});

	return c.newResponse(readable);
});

export default app;

if (import.meta.main) {
	Bun.serve({ fetch: app.fetch, hostname: config.host, port: config.port });
	console.log(`dashboard backend listening on ${config.host}:${config.port}`);
}
