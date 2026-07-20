// Thin fetch wrapper around the gateway API.
//
// Responsibilities:
//  - prefix requests with NEXT_PUBLIC_API_BASE_URL
//  - attach `Authorization: Bearer <access_token>` from localStorage
//  - unwrap the standard { success, data, error, request_id } envelope
//  - on a 401, attempt exactly one silent refresh via POST /auth/refresh,
//    retry the original request once, and if that also fails, clear
//    tokens and hard-redirect to /login.
//
// Known simplification (Phase 0): tokens live in localStorage and are
// read/written directly by this module. See frontend/README.md for the
// plan to move this to httpOnly cookies set by Next.js route handlers.

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080";

const ACCESS_TOKEN_KEY = "finora_access_token";
const REFRESH_TOKEN_KEY = "finora_refresh_token";

export interface ApiEnvelope<T> {
  success: boolean;
  data: T | null;
  error: {
    code: string;
    message: string;
    details?: Record<string, unknown>;
  } | null;
  request_id: string;
}

export class ApiError extends Error {
  code: string;
  status: number;
  details?: Record<string, unknown>;

  constructor(
    message: string,
    code: string,
    status: number,
    details?: Record<string, unknown>
  ) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
    this.details = details;
  }
}

export function getAccessToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(ACCESS_TOKEN_KEY);
}

export function getRefreshToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(REFRESH_TOKEN_KEY);
}

export function setTokens(accessToken: string, refreshToken: string): void {
  if (typeof window === "undefined") return;
  localStorage.setItem(ACCESS_TOKEN_KEY, accessToken);
  localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
}

export function clearTokens(): void {
  if (typeof window === "undefined") return;
  localStorage.removeItem(ACCESS_TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
}

function redirectToLogin(): void {
  if (typeof window !== "undefined") {
    window.location.href = "/login";
  }
}

async function parseEnvelope<T>(res: Response): Promise<ApiEnvelope<T>> {
  if (res.status === 204) {
    return { success: true, data: null, error: null, request_id: "" };
  }
  const text = await res.text();
  if (!text) {
    return { success: res.ok, data: null, error: null, request_id: "" };
  }
  try {
    return JSON.parse(text) as ApiEnvelope<T>;
  } catch {
    return {
      success: false,
      data: null,
      error: { code: "PARSE_ERROR", message: "Invalid response from server" },
      request_id: "",
    };
  }
}

export interface ApiFetchOptions extends RequestInit {
  /** Skip attaching the Authorization header (public endpoints: login/register/refresh). */
  skipAuth?: boolean;
}

async function rawFetch(
  path: string,
  options: ApiFetchOptions = {}
): Promise<Response> {
  const { skipAuth, headers, ...rest } = options;
  // A FormData body (file uploads, e.g. CSV import) must NOT get a manual
  // Content-Type — the browser sets its own `multipart/form-data;
  // boundary=...` when it serializes a FormData body, and a fetch() call
  // that already has a Content-Type header set is left alone rather than
  // having its boundary appended, so the request would be sent without a
  // boundary the server can parse. Every other call in this app sends a
  // JSON string body, so that stays the default.
  const finalHeaders: Record<string, string> = rest.body instanceof FormData
    ? { ...(headers as Record<string, string>) }
    : { "Content-Type": "application/json", ...(headers as Record<string, string>) };
  if (!skipAuth) {
    const token = getAccessToken();
    if (token) {
      finalHeaders["Authorization"] = `Bearer ${token}`;
    }
  }
  return fetch(`${API_BASE_URL}${path}`, { ...rest, headers: finalHeaders });
}

// Dedupe concurrent refresh attempts so parallel 401s trigger only one
// call to /auth/refresh.
let refreshPromise: Promise<boolean> | null = null;

async function tryRefresh(): Promise<boolean> {
  if (refreshPromise) return refreshPromise;

  refreshPromise = (async () => {
    const refreshToken = getRefreshToken();
    if (!refreshToken) return false;
    try {
      const res = await rawFetch("/api/v1/auth/refresh", {
        method: "POST",
        body: JSON.stringify({ refresh_token: refreshToken }),
        skipAuth: true,
      });
      if (!res.ok) return false;
      const envelope = await parseEnvelope<{
        access_token: string;
        refresh_token: string;
      }>(res);
      if (!envelope.success || !envelope.data) return false;
      setTokens(envelope.data.access_token, envelope.data.refresh_token);
      return true;
    } catch {
      return false;
    }
  })();

  try {
    return await refreshPromise;
  } finally {
    refreshPromise = null;
  }
}

/**
 * Call a gateway endpoint and return the unwrapped `data` field.
 * Throws ApiError on any non-success response (after the one retry-after-
 * refresh attempt for 401s).
 */
export async function apiFetch<T>(
  path: string,
  options: ApiFetchOptions = {}
): Promise<T> {
  let res = await rawFetch(path, options);

  if (res.status === 401 && !options.skipAuth) {
    const refreshed = await tryRefresh();
    if (refreshed) {
      res = await rawFetch(path, options);
    } else {
      clearTokens();
      redirectToLogin();
      throw new ApiError(
        "Session expired. Please log in again.",
        "UNAUTHORIZED",
        401
      );
    }
  }

  const envelope = await parseEnvelope<T>(res);

  if (!res.ok || !envelope.success) {
    if (res.status === 401) {
      clearTokens();
      redirectToLogin();
    }
    throw new ApiError(
      envelope.error?.message || `Request failed with status ${res.status}`,
      envelope.error?.code || "UNKNOWN_ERROR",
      res.status,
      envelope.error?.details
    );
  }

  return envelope.data as T;
}
