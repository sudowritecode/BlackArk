import { Hono } from "hono";
import {
	type ActivityEvent,
	eventBuffer as defaultEventBuffer,
	type EventBuffer,
} from "../lib/event-buffer";

const HEARTBEAT_INTERVAL_MS = 15_000;

interface EventsRouteOptions {
	controlUrl: string;
	controlApiToken: string;
	pollIntervalMs: number;
	maxConnections: number;
	heartbeatIntervalMs?: number;
	eventBuffer?: EventBuffer;
	fetch?: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;
}

export function createEventsRoutes(options: EventsRouteOptions) {
	const routes = new Hono();
	const fetchDashboard = options.fetch ?? fetch;
	const eventBuffer = options.eventBuffer ?? defaultEventBuffer;
	const seenEventKeys = new Set(eventBuffer.getAll().map(eventKey));
	const heartbeatIntervalMs =
		options.heartbeatIntervalMs ?? HEARTBEAT_INTERVAL_MS;
	let activeConnections = 0;

	routes.get("/api/v1/events", (c) => {
		if (activeConnections >= options.maxConnections) {
			return c.json({ error: "SSE connection limit reached" }, 503, {
				"Retry-After": "5",
			});
		}

		activeConnections += 1;
		const encoder = new TextEncoder();
		let closed = false;
		let pollInProgress = false;
		let pollTimer: ReturnType<typeof setInterval>;
		let heartbeatTimer: ReturnType<typeof setInterval>;

		const body = new ReadableStream<Uint8Array>({
			start(controller) {
				const close = () => {
					if (closed) return;
					closed = true;
					clearInterval(pollTimer);
					clearInterval(heartbeatTimer);
					activeConnections -= 1;
					try {
						controller.close();
					} catch {
						// The consumer may already have cancelled the stream.
					}
				};

				const write = (value: string) => {
					if (closed) return;
					try {
						controller.enqueue(encoder.encode(value));
					} catch {
						close();
					}
				};

				const poll = async () => {
					if (closed || pollInProgress) return;
					pollInProgress = true;
					try {
						const response = await fetchDashboard(
							`${options.controlUrl.replace(/\/$/, "")}/api/v1/dashboard`,
							{
								headers: {
									Authorization: `Bearer ${options.controlApiToken}`,
								},
							},
						);
						if (response.ok) {
							const dashboard = await response.json();
							if (isRecord(dashboard)) {
								const incomingEvents = Array.isArray(dashboard.events)
									? dashboard.events.filter(isActivityEvent)
									: [];
								for (const event of incomingEvents) {
									const key = eventKey(event);
									if (seenEventKeys.has(key)) continue;
									seenEventKeys.add(key);
									eventBuffer.push(event);
								}
								dashboard.events = eventBuffer.getAll();
							}
							write(`data: ${JSON.stringify(dashboard)}\n\n`);
						}
					} catch {
						// A transient upstream failure should not terminate the SSE stream.
					} finally {
						pollInProgress = false;
					}
				};

				pollTimer = setInterval(poll, options.pollIntervalMs);
				heartbeatTimer = setInterval(
					() => write(`event: ping\ndata: ${Date.now()}\n\n`),
					heartbeatIntervalMs,
				);
				c.req.raw.signal.addEventListener("abort", close, { once: true });
			},
			cancel() {
				if (closed) return;
				closed = true;
				clearInterval(pollTimer);
				clearInterval(heartbeatTimer);
				activeConnections -= 1;
			},
		});

		return new Response(body, {
			headers: {
				"Content-Type": "text/event-stream",
				"Cache-Control": "no-cache",
				Connection: "keep-alive",
			},
		});
	});

	return routes;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isActivityEvent(value: unknown): value is ActivityEvent {
	return (
		isRecord(value) &&
		typeof value.timestamp === "string" &&
		typeof value.type === "string" &&
		typeof value.message === "string"
	);
}

function eventKey(event: ActivityEvent): string {
	return typeof event.id === "string" || typeof event.id === "number"
		? `id:${event.id}`
		: JSON.stringify([event.timestamp, event.type, event.message]);
}
