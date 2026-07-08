export interface Config {
	readonly controlUrl: string;
	readonly controlApiToken: string;
	readonly port: number;
	readonly corsOrigins: string[];
	readonly dashboardPollIntervalSeconds: number;
	readonly maxSseConnections: number;
}

export function loadConfig(): Config {
	const controlUrl = process.env.CONTROL_URL ?? "http://localhost:8080";
	const controlApiToken = process.env.CONTROL_API_TOKEN ?? "";
	const port = parsePort(process.env.PORT, 3000);
	const corsOrigins = parseCorsOrigins(process.env.CORS_ORIGINS);
	const dashboardPollIntervalSeconds = parsePositiveInteger(
		process.env.DASHBOARD_POLL_INTERVAL,
		5,
	);
	const maxSseConnections = parsePositiveInteger(
		process.env.MAX_SSE_CONNECTIONS,
		10,
	);

	if (!controlApiToken) {
		console.warn(
			"CONTROL_API_TOKEN is not set — requests to the control plane will lack an auth header",
		);
	}

	return {
		controlUrl,
		controlApiToken,
		port,
		corsOrigins,
		dashboardPollIntervalSeconds,
		maxSseConnections,
	};
}

function parsePositiveInteger(raw: string | undefined, fallback: number): number {
	if (raw === undefined) return fallback;
	const value = Number(raw);
	return Number.isInteger(value) && value > 0 ? value : fallback;
}

function parseCorsOrigins(raw: string | undefined): string[] {
	if (!raw) return ["http://localhost:5173"];
	return raw
		.split(",")
		.map((s) => s.trim())
		.filter(Boolean);
}

function parsePort(raw: string | undefined, fallback: number): number {
	if (raw === undefined) return fallback;
	const port = Number(raw);
	return Number.isInteger(port) && port > 0 && port <= 65535 ? port : fallback;
}
