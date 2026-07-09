import { apiToken } from '../auth'

export interface ClusterInfo {
	url: string;
	version: string;
	uptime_seconds: number;
}

export interface NodeDetail {
	id: string;
	name: string;
	status: string;
	cpus: number;
	cpus_used: number;
	mem_bytes: number;
	mem_used_bytes: number;
	app_count: number;
	last_seen_at: string | null;
}

export interface NodeSection {
	healthy: number;
	unhealthy: number;
	pending: number;
	total: number;
	details: NodeDetail[];
}

export interface AppDetail {
	id: string;
	name: string;
	image: string;
	desired_replicas: number;
	ready_replicas: number;
	status: string;
}

export interface AppSection {
	running: number;
	stopped: number;
	failed: number;
	total: number;
	details: AppDetail[];
}

export interface DashboardData {
	cluster: ClusterInfo;
	nodes: NodeSection;
	apps: AppSection;
}

export interface HealthStatus {
	status: string;
	service: string;
	upstream: "ok" | "error";
}

export type ApiErrorKind = "network" | "auth" | "server" | "http" | "invalid_response";

export class ApiError extends Error {
	constructor(
		message: string,
		public readonly kind: ApiErrorKind,
		public readonly status?: number,
		public readonly cause?: unknown,
	) {
		super(message);
		this.name = "ApiError";
	}
}

export interface ApiConfig {
	baseUrl?: string;
	authToken?: string;
	authHeader?: string;
	authScheme?: string;
	fetch?: typeof globalThis.fetch;
}

export interface ApiClient {
	fetchDashboard(): Promise<DashboardData>;
	fetchHealth(): Promise<HealthStatus>;
}

type ErrorBody = { error?: string; message?: string };

function viteEnv(name: string): string | undefined {
	const env = (import.meta as ImportMeta & { env?: Record<string, unknown> }).env;
	const value = env?.[name];
	return typeof value === "string" && value.length > 0 ? value : undefined;
}

function normalizeBaseUrl(value: string): string {
	return value === "/" ? "" : value.replace(/\/$/, "");
}

async function errorMessage(response: Response): Promise<string> {
	try {
		const body = (await response.clone().json()) as ErrorBody;
		return body.error ?? body.message ?? response.statusText;
	} catch {
		return response.statusText;
	}
}

function errorKind(status: number): ApiErrorKind {
	if (status === 401 || status === 403) return "auth";
	if (status >= 500) return "server";
	return "http";
}

export function createApiClient(config: ApiConfig = {}): ApiClient {
	const baseUrl = normalizeBaseUrl(config.baseUrl ?? viteEnv("VITE_API_BASE_URL") ?? "");
	const configuredToken = config.authToken ?? viteEnv("VITE_API_TOKEN");
	const header = config.authHeader ?? "Authorization";
	const scheme = config.authScheme ?? "Bearer";
	const fetchImpl = config.fetch ?? globalThis.fetch;

	async function request<T>(path: string): Promise<T> {
		const headers = new Headers({ Accept: "application/json" });
		const token = configuredToken ?? apiToken.value;
		if (token) headers.set(header, scheme ? `${scheme} ${token}` : token);

		let response: Response;
		try {
			response = await fetchImpl(`${baseUrl}${path}`, { method: "GET", headers });
		} catch (cause) {
			throw new ApiError("Unable to reach the BlackArk API", "network", undefined, cause);
		}

		if (!response.ok) {
			const detail = await errorMessage(response);
			throw new ApiError(detail || `API request failed with status ${response.status}`, errorKind(response.status), response.status);
		}

		try {
			return (await response.json()) as T;
		} catch (cause) {
			throw new ApiError("API returned an invalid JSON response", "invalid_response", response.status, cause);
		}
	}

	return {
		fetchDashboard: () => request<DashboardData>("/api/v1/dashboard"),
		fetchHealth: () => request<HealthStatus>("/health"),
	};
}

let apiClient = createApiClient();

export function configureApi(config: ApiConfig): void {
	apiClient = createApiClient(config);
}

export function fetchDashboard(): Promise<DashboardData> {
	return apiClient.fetchDashboard();
}

export function fetchHealth(): Promise<HealthStatus> {
	return apiClient.fetchHealth();
}
