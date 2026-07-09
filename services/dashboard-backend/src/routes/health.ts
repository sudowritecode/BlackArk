import { Hono } from "hono";

const SERVICE_NAME = "blackark-hono-backend";
const UPSTREAM_TIMEOUT_MS = 75;

interface HealthRouteOptions {
	controlUrl: string;
	fetch?: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;
}

export function createHealthRoutes(options: HealthRouteOptions) {
	const routes = new Hono();
	const fetchUpstream = options.fetch ?? fetch;
	const healthUrl = `${options.controlUrl.replace(/\/$/, "")}/healthz`;

	routes.get("/health", async (c) => {
		try {
			const response = await fetchUpstream(healthUrl, {
				method: "GET",
				signal: AbortSignal.timeout(UPSTREAM_TIMEOUT_MS),
			});

			if (response.ok) {
				return c.json({ status: "ok", service: SERVICE_NAME, upstream: "ok" });
			}
		} catch {
			// An unreachable or slow control server makes this service unhealthy.
		}

		return c.json(
			{ status: "ok", service: SERVICE_NAME, upstream: "error" },
			503,
		);
	});

	return routes;
}
