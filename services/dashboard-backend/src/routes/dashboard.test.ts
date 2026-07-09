import { describe, expect, test } from "bun:test";
import { createDashboardRoutes } from "./dashboard";

describe("GET /api/v1/dashboard", () => {
	test("proxies dashboard JSON with bearer authentication", async () => {
		let requestedUrl = "";
		let authorization = "";
		const dashboard = { cluster: { nodes: 3 }, events: [{ id: "event-1" }] };
		const app = createDashboardRoutes({
			controlUrl: "http://control:8080/",
			controlToken: "secret-token",
			fetch: async (input, init) => {
				requestedUrl = input.toString();
				authorization = new Headers(init?.headers).get("Authorization") ?? "";
				return Response.json(dashboard);
			},
		});

		const response = await app.request("/api/v1/dashboard");

		expect(response.status).toBe(200);
		expect(requestedUrl).toBe("http://control:8080/api/v1/dashboard");
		expect(authorization).toBe("Bearer secret-token");
		expect(response.headers.get("Cache-Control")).toBe("no-cache");
		expect(await response.json()).toEqual(dashboard);
	});

	test("returns a descriptive 502 for an upstream HTTP error", async () => {
		const app = createDashboardRoutes({
			controlUrl: "http://control:8080",
			controlToken: "token",
			fetch: async () => new Response("unavailable", { status: 503 }),
		});

		const response = await app.request("/api/v1/dashboard");

		expect(response.status).toBe(502);
		expect(response.headers.get("Cache-Control")).toBe("no-cache");
		expect(await response.json()).toEqual({
			error: "dashboard upstream request failed",
			upstreamStatus: 503,
		});
	});

	test("returns a descriptive 502 when the upstream is unreachable", async () => {
		const app = createDashboardRoutes({
			controlUrl: "http://control:8080",
			controlToken: "token",
			fetch: async () => {
				throw new TypeError("connection refused");
			},
		});

		const response = await app.request("/api/v1/dashboard");

		expect(response.status).toBe(502);
		expect(await response.json()).toEqual({
			error: "dashboard upstream is unavailable",
		});
	});

	test("returns a descriptive 502 for malformed upstream JSON", async () => {
		const app = createDashboardRoutes({
			controlUrl: "http://control:8080",
			controlToken: "token",
			fetch: async () =>
				new Response("not-json", {
					headers: { "Content-Type": "application/json" },
				}),
		});

		const response = await app.request("/api/v1/dashboard");

		expect(response.status).toBe(502);
		expect(await response.json()).toEqual({
			error: "dashboard upstream returned invalid JSON",
		});
	});
});
