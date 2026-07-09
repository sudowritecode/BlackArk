import { describe, expect, test } from "bun:test";
import { EventBuffer } from "../lib/event-buffer";
import { createEventsRoutes } from "./events";

function createTestApp(
	maxConnections = 10,
	dashboard?: Record<string, unknown>,
) {
	let requests = 0;
	const app = createEventsRoutes({
		controlUrl: "http://control:8080/",
		controlApiToken: "test-token",
		pollIntervalMs: 5,
		heartbeatIntervalMs: 8,
		maxConnections,
		eventBuffer: new EventBuffer(),
		fetch: async (input, init) => {
			requests += 1;
			expect(input.toString()).toBe("http://control:8080/api/v1/dashboard");
			expect(new Headers(init?.headers).get("Authorization")).toBe(
				"Bearer test-token",
			);
			return Response.json(dashboard ?? { sequence: requests });
		},
	});
	return { app, requestCount: () => requests };
}

describe("GET /api/v1/events", () => {
	test("streams dashboard snapshots and heartbeat events", async () => {
		const { app } = createTestApp();
		const response = await app.request("/api/v1/events");
		const reader = response.body?.getReader();
		expect(response.status).toBe(200);
		expect(response.headers.get("Content-Type")).toBe("text/event-stream");
		expect(response.headers.get("Cache-Control")).toBe("no-cache");

		let output = "";
		const deadline = Date.now() + 250;
		while (
			(!output.includes('data: {"sequence":1}') ||
				!output.includes("event: ping")) &&
			Date.now() < deadline
		) {
			const chunk = await reader?.read();
			if (chunk?.value) output += new TextDecoder().decode(chunk.value);
		}
		await reader?.cancel();

		expect(output).toContain('data: {"sequence":1,"events":[]}\n\n');
		expect(output).toContain("event: ping\n");
	});

	test("rejects connections above the configured limit and frees disconnected slots", async () => {
		const { app } = createTestApp(1);
		const first = await app.request("/api/v1/events");
		const rejected = await app.request("/api/v1/events");

		expect(rejected.status).toBe(503);
		expect(rejected.headers.get("Retry-After")).toBe("5");
		expect(await rejected.json()).toEqual({
			error: "SSE connection limit reached",
		});

		await first.body?.cancel();
		const replacement = await app.request("/api/v1/events");
		expect(replacement.status).toBe(200);
		await replacement.body?.cancel();
	});

	test("includes buffered activity events in each dashboard update", async () => {
		const activity = {
			timestamp: "2026-07-08T12:00:00.000Z",
			type: "node_health",
			message: "Node edge-1 became healthy",
		};
		const { app } = createTestApp(10, { cluster: {}, events: [activity] });
		const response = await app.request("/api/v1/events");
		const reader = response.body?.getReader();
		let output = "";
		const deadline = Date.now() + 250;
		while (!output.includes(activity.message) && Date.now() < deadline) {
			const chunk = await reader?.read();
			if (chunk?.value) output += new TextDecoder().decode(chunk.value);
		}
		await reader?.cancel();

		expect(output).toContain(`"events":[${JSON.stringify(activity)}]`);
	});
});
