import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { loadConfig } from "./config";

describe("loadConfig", () => {
	const keys = [
		"CONTROL_URL",
		"CONTROL_TOKEN",
		"HOST",
		"PORT",
		"CORS_ORIGIN",
		"DASHBOARD_POLL_INTERVAL",
		"MAX_SSE_CONNECTIONS",
	] as const;
	const original = new Map<string, string | undefined>();

	beforeEach(() => {
		for (const key of keys) {
			original.set(key, process.env[key]);
			delete process.env[key];
		}
		process.env.CONTROL_TOKEN = "test-token";
	});

	afterEach(() => {
		for (const key of keys) {
			const value = original.get(key);
			if (value === undefined) delete process.env[key];
			else process.env[key] = value;
		}
		original.clear();
	});

	test("returns documented defaults", () => {
		const config = loadConfig();
		expect(config.controlUrl).toBe("http://localhost:8080");
		expect(config.host).toBe("0.0.0.0");
		expect(config.port).toBe(3001);
		expect(config.corsOrigin).toBe("http://localhost:5173");
		expect(config.dashboardPollIntervalSeconds).toBe(5);
		expect(config.maxSseConnections).toBe(10);
	});

	test("reads environment overrides", () => {
		process.env.CONTROL_URL = "https://control.example.com";
		process.env.CONTROL_TOKEN = "secret-token";
		process.env.HOST = "127.0.0.1";
		process.env.PORT = "9090";
		process.env.CORS_ORIGIN = " https://app.example.com ";
		process.env.DASHBOARD_POLL_INTERVAL = "2";
		process.env.MAX_SSE_CONNECTIONS = "25";

		const config = loadConfig();
		expect(config.controlUrl).toBe("https://control.example.com");
		expect(config.controlToken).toBe("secret-token");
		expect(config.host).toBe("127.0.0.1");
		expect(config.port).toBe(9090);
		expect(config.corsOrigin).toBe("https://app.example.com");
		expect(config.dashboardPollIntervalSeconds).toBe(2);
		expect(config.maxSseConnections).toBe(25);
	});

	test("throws a clear error when CONTROL_TOKEN is missing", () => {
		delete process.env.CONTROL_TOKEN;
		expect(() => loadConfig()).toThrow("CONTROL_TOKEN is required");
	});

	test("falls back for invalid numeric settings", () => {
		process.env.PORT = "9090-http";
		process.env.DASHBOARD_POLL_INTERVAL = "0";
		process.env.MAX_SSE_CONNECTIONS = "invalid";
		const config = loadConfig();
		expect(config.port).toBe(3001);
		expect(config.dashboardPollIntervalSeconds).toBe(5);
		expect(config.maxSseConnections).toBe(10);
	});
});
