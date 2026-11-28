// The one place that talks to the Go API.
// It only imports types, so it also runs under plain Node for tests.

import type {
  AuthResponse,
  CreateIntakeResponse,
  Intake,
  IntakeDetail,
  IntakeStatus,
  NewPatient,
  PublicIntake,
  SendMessageResponse,
  Staff,
  Triage,
} from "./types";

export const API_URL = (
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"
).replace(/\/+$/, "");

/** An error from the API. status is 0 when the server could not be reached. */
export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

// When a logged-in request gets 401, the token is no good any more (expired,
// or the account was removed). The app registers a handler that logs out.
let onUnauthorized: (() => void) | null = null;

export function setUnauthorizedHandler(fn: (() => void) | null) {
  onUnauthorized = fn;
}

interface RequestOptions {
  method?: string;
  body?: unknown;
  token?: string | null;
  signal?: AbortSignal;
}

async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  if (opts.token) headers.Authorization = `Bearer ${opts.token}`;

  let res: Response;
  try {
    res = await fetch(`${API_URL}${path}`, {
      method: opts.method ?? "GET",
      headers,
      body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
      signal: opts.signal,
    });
  } catch (err) {
    if (err instanceof Error && err.name === "AbortError") throw err;
    throw new ApiError(0, "Cannot reach the server. Check your connection and try again.");
  }

  const text = await res.text();
  let data: unknown = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = null;
    }
  }

  if (!res.ok) {
    if (res.status === 401 && opts.token) onUnauthorized?.();
    throw new ApiError(res.status, errorMessage(data, res.status));
  }
  return data as T;
}

function errorMessage(data: unknown, status: number): string {
  if (data && typeof data === "object" && "error" in data) {
    const msg = (data as { error: unknown }).error;
    if (typeof msg === "string" && msg) return msg;
  }
  return `Request failed (${status})`;
}

// ---- auth ----

export interface RegisterInput {
  email: string;
  password: string;
  full_name: string;
}

export const api = {
  register: (input: RegisterInput) =>
    request<AuthResponse>("/auth/register", { method: "POST", body: input }),

  login: (email: string, password: string) =>
    request<AuthResponse>("/auth/login", { method: "POST", body: { email, password } }),

  me: (token: string, signal?: AbortSignal) =>
    request<{ staff: Staff }>("/auth/me", { token, signal }),

  // ---- intakes (staff) ----

  createIntake: (token: string, patient?: NewPatient) =>
    request<CreateIntakeResponse>("/intakes", {
      method: "POST",
      token,
      body: patient ? { patient } : undefined,
    }),

  listIntakes: (
    token: string,
    opts: { status?: Exclude<IntakeStatus, "expired">; limit?: number; signal?: AbortSignal } = {},
  ) => {
    const query = new URLSearchParams();
    if (opts.status) query.set("status", opts.status);
    if (opts.limit) query.set("limit", String(opts.limit));
    const qs = query.size > 0 ? `?${query}` : "";
    return request<{ intakes: Intake[] }>(`/intakes${qs}`, { token, signal: opts.signal });
  },

  getIntake: (token: string, id: string, signal?: AbortSignal) =>
    request<IntakeDetail>(`/intakes/${encodeURIComponent(id)}`, { token, signal }),

  /** The saved triage result, or null when triage has not run yet. */
  getTriage: async (token: string, id: string, signal?: AbortSignal): Promise<Triage | null> => {
    try {
      const res = await request<{ triage: Triage }>(`/intakes/${encodeURIComponent(id)}/triage`, {
        token,
        signal,
      });
      return res.triage;
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) return null;
      throw err;
    }
  },

  runTriage: (token: string, id: string) =>
    request<{ triage: Triage }>(`/intakes/${encodeURIComponent(id)}/triage/run`, {
      method: "POST",
      token,
    }).then((r) => r.triage),

  // ---- patient chat (no login. The link token is the key) ----

  getPublicIntake: (linkToken: string, signal?: AbortSignal) =>
    request<PublicIntake>(`/public/intakes/${encodeURIComponent(linkToken)}`, { signal }),

  sendMessage: (linkToken: string, content: string) =>
    request<SendMessageResponse>(`/public/intakes/${encodeURIComponent(linkToken)}/messages`, {
      method: "POST",
      body: { content },
    }),

  completeIntake: (linkToken: string) =>
    request<{ status: IntakeStatus }>(`/public/intakes/${encodeURIComponent(linkToken)}/complete`, {
      method: "POST",
    }),
};
