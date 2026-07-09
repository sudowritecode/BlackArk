import { Hono } from "hono";

const UPSTREAM_TIMEOUT_MS = 5_000;

interface DashboardRouteOptions {
	controlUrl: string;
	controlToken: string;
	fetch?: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;
}

export function createDashboardRoutes(options: DashboardRouteOptions) {
	const routes = new Hono();
	const fetchUpstream = options.fetch ?? fetch;
	const dashboardUrl = `${options.controlUrl.replace(/\/$/, "")}/api/v1/dashboard`;

	routes.get("/api/v1/dashboard", async (c) => {
		c.header("Cache-Control", "no-cache");

		try {
			const response = await fetchUpstream(dashboardUrl, {
				method: "GET",
				headers: { Authorization: `Bearer ${options.controlToken}` },
				signal: AbortSignal.timeout(UPSTREAM_TIMEOUT_MS),
			});

			if (!response.ok) {
				return c.json(
					{
						error: "dashboard upstream request failed",
						upstreamStatus: response.status,
					},
					502,
				);
			}

			return c.json(await response.json());
		} catch (error) {
			const message =
				error instanceof SyntaxError
					? "dashboard upstream returned invalid JSON"
					: "dashboard upstream is unavailable";
			return c.json({ error: message }, 502);
		}
	});

	return routes;
}
