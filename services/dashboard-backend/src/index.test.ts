import { afterAll, beforeAll, describe, expect, test } from "bun:test";

const corsOrigin = "https://dashboard.example.com";
let app: typeof import("./index")["app"];

beforeAll(async () => {
	process.env.CONTROL_API_TOKEN = "test-token";
	process.env.CORS_ORIGINS = corsOrigin;
	app = (await import("./index")).app;
});

afterAll(() => {
	delete process.env.CONTROL_API_TOKEN;
	delete process.env.CORS_ORIGINS;
});

describe("CORS", () => {
	test("handles API preflight requests with the configured policy", async () => {
		const response = await app.request("/api/v1/dashboard", {
			method: "OPTIONS",
			headers: {
				Origin: corsOrigin,
				"Access-Control-Request-Method": "POST",
				"Access-Control-Request-Headers": "Content-Type, Authorization",
			},
		});

		expect(response.status).toBe(204);
		expect(response.headers.get("Access-Control-Allow-Origin")).toBe(
			corsOrigin,
		);
		expect(response.headers.get("Access-Control-Allow-Methods")).toBe(
			"GET,POST,OPTIONS",
		);
		expect(response.headers.get("Access-Control-Allow-Headers")).toBe(
			"Content-Type,Authorization",
		);
		expect(response.headers.get("Access-Control-Allow-Credentials")).toBe(
			"true",
		);
	});

	test("exposes the SSE content and cache headers", async () => {
		const controller = new AbortController();
		const response = await app.request("/api/v1/events", {
			headers: { Origin: corsOrigin },
			signal: controller.signal,
		});
		controller.abort();

		expect(response.headers.get("Access-Control-Allow-Origin")).toBe(
			corsOrigin,
		);
		expect(response.headers.get("Access-Control-Expose-Headers")).toBe(
			"Content-Type,Cache-Control",
		);
		expect(response.headers.get("Access-Control-Allow-Credentials")).toBe(
			"true",
		);
	});
});
