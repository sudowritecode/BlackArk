import { Hono } from "hono";
import { cors } from "hono/cors";
import { loadConfig } from "./config";
import { createDashboardRoutes } from "./routes/dashboard";
import { createEventsRoutes } from "./routes/events";
import { createHealthRoutes } from "./routes/health";

const config = loadConfig();

export const app = new Hono();

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
app.route(
	"/",
	createEventsRoutes({
		controlUrl: config.controlUrl,
		controlApiToken: config.controlApiToken,
		pollIntervalMs: config.dashboardPollIntervalSeconds * 1000,
		maxConnections: config.maxSseConnections,
	}),
);
app.route(
	"/",
	createDashboardRoutes({
		controlUrl: config.controlUrl,
		controlToken: config.controlApiToken,
	}),
);

if (import.meta.main) {
	Bun.serve({ fetch: app.fetch, hostname: config.host, port: config.port });
	console.log(`dashboard backend listening on ${config.host}:${config.port}`);
}
