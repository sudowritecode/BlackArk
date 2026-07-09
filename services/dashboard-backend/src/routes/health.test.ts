import { describe, expect, test } from "bun:test";
import { createHealthRoutes } from "./health";

describe("GET /health", () => {
	test("reports a healthy control server", async () => {
		let requestedUrl = "";
		const app = createHealthRoutes({
			controlUrl: "http://control:8080/",
			fetch: async (input) => {
				requestedUrl = input.toString();
				return new Response("ok");
			},
		});

		const response = await app.request("/health");

		expect(response.status).toBe(200);
		expect(requestedUrl).toBe("http://control:8080/healthz");
		expect(await response.json()).toEqual({
			status: "ok",
			service: "blackark-hono-backend",
			upstream: "ok",
		});
	});

	test("returns 503 when the control server is unhealthy", async () => {
		const app = createHealthRoutes({
			controlUrl: "http://control:8080",
			fetch: async () => new Response("unhealthy", { status: 503 }),
		});

		const response = await app.request("/health");

		expect(response.status).toBe(503);
		expect(await response.json()).toEqual({
			status: "ok",
			service: "blackark-hono-backend",
			upstream: "error",
		});
	});

	test("returns 503 within 100ms when the control server is unreachable", async () => {
		const app = createHealthRoutes({
			controlUrl: "http://control:8080",
			fetch: async (_input, init) =>
				await new Promise<Response>((_resolve, reject) => {
					init?.signal?.addEventListener("abort", () =>
						reject(init.signal?.reason),
					);
				}),
		});
		const startedAt = performance.now();

		const response = await app.request("/health");

		expect(response.status).toBe(503);
		expect(performance.now() - startedAt).toBeLessThan(100);
	});
});
