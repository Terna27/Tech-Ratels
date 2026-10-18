// Next.js API Client with HttpOnly Cookie-Based Authentication & Double-Submit CSRF Protection
// Browser session tokens are strictly stored in HttpOnly cookies (never in sessionStorage or localStorage).
// In the browser, client-side requests target the same-origin Next.js reverse proxy (/api/v1).

class ApiError extends Error {
  code: string;
  details: Record<string, any>;
  status: number;

  constructor(status: number, code: string, message: string, details: Record<string, any> = {}) {
    super(message);
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

// In the browser, use same-origin relative proxy /api/v1. On the server (SSR), use backend URL.
const getApiBase = (): string => {
  if (typeof window !== "undefined") {
    return "/api/v1";
  }
  const raw = process.env.INTERNAL_API_URL || process.env.API_URL || process.env.BACKEND_URL || process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8000";
  return raw.endsWith("/api/v1") ? raw : `${raw.replace(/\/+$/, "")}/api/v1`;
};

export const API_BASE = getApiBase();

let memoryCsrfToken: string | null = null;

export function setCsrfToken(token: string | null) {
  memoryCsrfToken = token;
}

export function getCsrfToken(): string | null {
  if (memoryCsrfToken) return memoryCsrfToken;
  if (typeof document !== "undefined") {
    const match = document.cookie.match(/(?:^|;\s*)ratels_csrf=([^;]+)/);
    if (match) {
      memoryCsrfToken = decodeURIComponent(match[1]);
      return memoryCsrfToken;
    }
  }
  return null;
}

export function clearCsrfToken() {
  memoryCsrfToken = null;
}

export async function apiRequest<T = any>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const method = (options.method || "GET").toUpperCase();
  const isMutating = ["POST", "PUT", "DELETE", "PATCH"].includes(method);

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };

  // Double-submit CSRF token attachment for mutating cookie-authenticated requests
  if (isMutating && !headers["X-CSRF-Token"]) {
    const csrf = getCsrfToken();
    if (csrf) {
      headers["X-CSRF-Token"] = csrf;
    }
  }

  // Same-origin relative URL resolution in browser (/api/v1/...)
  let url: string;
  if (endpoint.startsWith("http")) {
    url = endpoint;
  } else {
    const base = typeof window !== "undefined" ? "/api/v1" : API_BASE;
    const cleanEndpoint = endpoint.startsWith("/") ? endpoint : `/${endpoint}`;
    url = `${base}${cleanEndpoint}`;
  }

  const res = await fetch(url, {
    ...options,
    credentials: options.credentials || "same-origin",
    headers,
  });

  if (!res.ok) {
    let errCode = "UNKNOWN_ERROR";
    let errMsg = `Request failed with status ${res.status}`;
    let errDetails = {};

    try {
      const errJson = await res.json();
      if (errJson.error) {
        errCode = errJson.error.code || errCode;
        errMsg = errJson.error.message || errMsg;
        errDetails = errJson.error.details || {};
      } else if (errJson.detail) {
        errMsg = typeof errJson.detail === "string" ? errJson.detail : JSON.stringify(errJson.detail);
      }
    } catch {
      // Non-JSON response
    }

    throw new ApiError(res.status, errCode, errMsg, errDetails);
  }

  // Handle 204 No Content
  if (res.status === 204) {
    return {} as T;
  }

  const data = await res.json();

  // If response contains csrf_token (e.g. login), register it
  if (data && typeof data === "object" && "csrf_token" in data && typeof data.csrf_token === "string") {
    setCsrfToken(data.csrf_token);
  }

  return data as T;
}
