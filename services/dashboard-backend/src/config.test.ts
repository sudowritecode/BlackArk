import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { loadConfig } from "./config";

describe("loadConfig", () => {
	const configKeys = [
		"CONTROL_URL",
		"CONTROL_API_TOKEN",
		"PORT",
		"CORS_ORIGINS",
		"DASHBOARD_POLL_INTERVAL",
		"MAX_SSE_CONNECTIONS",
	] as const;
	const originalValues = new Map<string, string | undefined>();

	beforeEach(() => {
		for (const key of configKeys) {
			originalValues.set(key, process.env[key]);
			delete process.env[key];
		}
	});

	afterEach(() => {
		for (const key of configKeys) {
			const value = originalValues.get(key);
			if (value === undefined) delete process.env[key];
			else process.env[key] = value;
		}
		originalValues.clear();
	});

	test("returns defaults when no env vars are set", () => {
		const config = loadConfig();
		expect(config.controlUrl).toBe("http://localhost:8080");
		expect(config.controlApiToken).toBe("");
		expect(config.port).toBe(3000);
		expect(config.corsOrigins).toEqual(["http://localhost:5173"]);
		expect(config.dashboardPollIntervalSeconds).toBe(5);
		expect(config.maxSseConnections).toBe(10);
	});

	test("reads SSE settings and rejects non-positive values", () => {
		process.env.DASHBOARD_POLL_INTERVAL = "2";
		process.env.MAX_SSE_CONNECTIONS = "25";
		let config = loadConfig();
		expect(config.dashboardPollIntervalSeconds).toBe(2);
		expect(config.maxSseConnections).toBe(25);

		process.env.DASHBOARD_POLL_INTERVAL = "0";
		process.env.MAX_SSE_CONNECTIONS = "invalid";
		config = loadConfig();
		expect(config.dashboardPollIntervalSeconds).toBe(5);
		expect(config.maxSseConnections).toBe(10);
	});

	test("reads CONTROL_URL from the environment", () => {
		process.env.CONTROL_URL = "https://control.example.com";
		const config = loadConfig();
		expect(config.controlUrl).toBe("https://control.example.com");
	});

	test("reads CONTROL_API_TOKEN from the environment", () => {
		process.env.CONTROL_API_TOKEN = "secret-token";
		const config = loadConfig();
		expect(config.controlApiToken).toBe("secret-token");
	});

	test("reads PORT from the environment", () => {
		process.env.PORT = "9090";
		const config = loadConfig();
		expect(config.port).toBe(9090);
	});

	test("falls back to default port when PORT is not a valid number", () => {
		process.env.PORT = "not-a-number";
		const config = loadConfig();
		expect(config.port).toBe(3000);
	});

	test("falls back to default port when PORT has trailing characters", () => {
		process.env.PORT = "9090-http";
		const config = loadConfig();
		expect(config.port).toBe(3000);
	});

	test("falls back to default port when PORT is out of range", () => {
		process.env.PORT = "70000";
		const config = loadConfig();
		expect(config.port).toBe(3000);
	});

	test("parses CORS_ORIGINS as a comma-separated list", () => {
		process.env.CORS_ORIGINS = "https://app.example.com,https://admin.example.com";
		const config = loadConfig();
		expect(config.corsOrigins).toEqual([
			"https://app.example.com",
			"https://admin.example.com",
		]);
	});

	test("trims whitespace from CORS_ORIGINS entries", () => {
		process.env.CORS_ORIGINS = " https://a.com , https://b.com ";
		const config = loadConfig();
		expect(config.corsOrigins).toEqual(["https://a.com", "https://b.com"]);
	});

	test("handles a single CORS_ORIGINS entry", () => {
		process.env.CORS_ORIGINS = "https://single.example.com";
		const config = loadConfig();
		expect(config.corsOrigins).toEqual(["https://single.example.com"]);
	});
});
