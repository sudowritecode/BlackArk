export interface Config {
	readonly controlUrl: string;
	readonly controlToken: string;
	readonly controlApiToken: string;
	readonly host: string;
	readonly port: number;
	readonly corsOrigin: string;
	readonly dashboardPollIntervalSeconds: number;
	readonly maxSseConnections: number;
}

export function loadConfig(): Config {
	const controlUrl = process.env.CONTROL_URL ?? "http://localhost:8080";
	const controlToken = process.env.CONTROL_TOKEN?.trim();
	if (!controlToken) {
		throw new Error("CONTROL_TOKEN is required");
	}
	const host = process.env.HOST?.trim() || "0.0.0.0";
	const port = parsePort(process.env.PORT, 3001);
	const corsOrigin =
		process.env.CORS_ORIGIN?.trim() || "http://localhost:5173";
	const dashboardPollIntervalSeconds = parsePositiveInteger(
		process.env.DASHBOARD_POLL_INTERVAL,
		5,
	);
	const maxSseConnections = parsePositiveInteger(
		process.env.MAX_SSE_CONNECTIONS,
		10,
	);

	return {
		controlUrl,
		controlToken,
		// Compatibility for route factories that still use the earlier field name.
		controlApiToken: controlToken,
		host,
		port,
		corsOrigin,
		dashboardPollIntervalSeconds,
		maxSseConnections,
	};
}

function parsePositiveInteger(
	raw: string | undefined,
	fallback: number,
): number {
	if (raw === undefined) return fallback;
	const value = Number(raw);
	return Number.isInteger(value) && value > 0 ? value : fallback;
}

function parsePort(raw: string | undefined, fallback: number): number {
	if (raw === undefined) return fallback;
	const port = Number(raw);
	return Number.isInteger(port) && port > 0 && port <= 65535 ? port : fallback;
}
