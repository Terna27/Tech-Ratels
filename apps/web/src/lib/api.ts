// Next.js API Client with HttpOnly Cookie-Based Authentication & Double-Submit CSRF Protection
// Browser session tokens are strictly stored in HttpOnly cookies (never in sessionStorage or localStorage).
// In the browser, client-side requests target the same-origin Next.js reverse proxy (/api/v1).

export class ApiError extends Error {
  code: string;
  details: Record<string, any>;
  status: number;

  constructor(status: number, code: string, message: string, details: Record<string, any> = {}) {
    super(message);
    this.status = status;
    this.code = code;
    this.details = details;
    this.name = "ApiError";
  }
}

// In the browser, use same-origin relative proxy /api/v1. On the server (SSR), use backend URL.
const getApiBase = (): string => {
  if (typeof window !== "undefined") {
    return "/api/v1";
  }
  const serverTarget = process.env.INTERNAL_API_URL || process.env.API_URL || process.env.BACKEND_URL;
  let raw = serverTarget;
  if (!raw && process.env.NEXT_PUBLIC_API_URL && (process.env.NEXT_PUBLIC_API_URL.startsWith("http://") || process.env.NEXT_PUBLIC_API_URL.startsWith("https://"))) {
    raw = process.env.NEXT_PUBLIC_API_URL;
  }
  if (!raw) {
    raw = process.env.NODE_ENV === "production" ? "https://ratels.onrender.com" : "http://127.0.0.1:8000";
  }
  const trimmed = raw.trim().replace(/\/+$/, "");
  return trimmed.endsWith("/api/v1") ? trimmed : `${trimmed}/api/v1`;
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

  // If request body is FormData, delete Content-Type so browser sets boundary automatically
  if (typeof FormData !== "undefined" && options.body instanceof FormData) {
    delete headers["Content-Type"];
  }

  // Double-submit CSRF token attachment for mutating cookie-authenticated requests
  if (isMutating && !headers["X-CSRF-Token"]) {
    const csrf = getCsrfToken();
    if (csrf) {
      headers["X-CSRF-Token"] = csrf;
    }
  }

  // Same-origin relative URL resolution in browser (/api/v1/...)
  let url: string;
  if (endpoint.startsWith("http://") || endpoint.startsWith("https://")) {
    url = endpoint;
  } else {
    const base = typeof window !== "undefined" ? "/api/v1" : API_BASE;
    let cleanEndpoint = endpoint.startsWith("/") ? endpoint : `/${endpoint}`;
    // Guard against duplicated /api/v1 paths (e.g. /api/v1/api/v1/... or /api/v1/auth/...)
    if (cleanEndpoint.startsWith("/api/v1/")) {
      cleanEndpoint = cleanEndpoint.substring("/api/v1".length);
    } else if (cleanEndpoint === "/api/v1") {
      cleanEndpoint = "";
    }
    url = `${base}${cleanEndpoint}`;
  }

  let res: Response;
  try {
    res = await fetch(url, {
      ...options,
      credentials: options.credentials || "same-origin",
      headers,
    });
  } catch (netErr: any) {
    throw new ApiError(
      503,
      "NETWORK_ERROR",
      "Unable to connect to server. Please check your internet connection or try again in a few moments.",
      { originalError: netErr?.message }
    );
  }

  if (!res.ok) {
    let errCode = "UNKNOWN_ERROR";
    let errMsg = `Request failed with status ${res.status}`;
    let errDetails: Record<string, any> = {};

    try {
      const contentType = res.headers.get("content-type") || "";
      if (contentType.includes("application/json")) {
        const errJson = await res.json();
        if (errJson && typeof errJson === "object") {
          if (errJson.error) {
            errCode = errJson.error.code || errCode;
            errMsg = errJson.error.message || errMsg;
            errDetails = errJson.error.details || {};
          } else if (errJson.detail) {
            errMsg = typeof errJson.detail === "string" ? errJson.detail : JSON.stringify(errJson.detail);
          } else if (errJson.message) {
            errMsg = errJson.message;
          }
        }
      } else {
        // Non-JSON response (e.g. 502 Bad Gateway / 503 / 504 HTML error page)
        if (res.status === 502) {
          errCode = "BAD_GATEWAY";
          errMsg = "Service temporarily unavailable (502 Bad Gateway). The backend server may be restarting or deploying. Please wait a moment and try again.";
        } else if (res.status === 503) {
          errCode = "SERVICE_UNAVAILABLE";
          errMsg = "Service temporarily unavailable (503 Service Unavailable). Please wait a moment and try again.";
        } else if (res.status === 504) {
          errCode = "GATEWAY_TIMEOUT";
          errMsg = "Gateway timeout (504). The server took too long to respond. Please try again shortly.";
        } else if (res.status >= 500) {
          errCode = "SERVER_ERROR";
          errMsg = `Server error (${res.status}). The service is temporarily unable to process your request. Please try again shortly.`;
        }
      }
    } catch {
      // JSON parse error or response stream closed
      if (res.status === 502) {
        errCode = "BAD_GATEWAY";
        errMsg = "Service temporarily unavailable (502 Bad Gateway). The backend server may be restarting or deploying. Please wait a moment and try again.";
      } else if (res.status === 503) {
        errCode = "SERVICE_UNAVAILABLE";
        errMsg = "Service temporarily unavailable (503 Service Unavailable). Please wait a moment and try again.";
      } else if (res.status === 504) {
        errCode = "GATEWAY_TIMEOUT";
        errMsg = "Gateway timeout (504). The server took too long to respond. Please try again shortly.";
      } else if (res.status >= 500) {
        errCode = "SERVER_ERROR";
        errMsg = `Server error (${res.status}). The service is temporarily unable to process your request. Please try again shortly.`;
      }
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

export const api = {
  get: <T = any>(endpoint: string, options?: RequestInit) =>
    apiRequest<T>(endpoint, { ...options, method: "GET" }),
  post: <T = any>(endpoint: string, data?: any, options?: RequestInit) =>
    apiRequest<T>(endpoint, {
      ...options,
      method: "POST",
      body:
        data !== undefined
          ? typeof data === "string" || (typeof FormData !== "undefined" && data instanceof FormData)
            ? data
            : JSON.stringify(data)
          : undefined,
    }),
  put: <T = any>(endpoint: string, data?: any, options?: RequestInit) =>
    apiRequest<T>(endpoint, {
      ...options,
      method: "PUT",
      body:
        data !== undefined
          ? typeof data === "string" || (typeof FormData !== "undefined" && data instanceof FormData)
            ? data
            : JSON.stringify(data)
          : undefined,
    }),
  delete: <T = any>(endpoint: string, options?: RequestInit) =>
    apiRequest<T>(endpoint, { ...options, method: "DELETE" }),
  patch: <T = any>(endpoint: string, data?: any, options?: RequestInit) =>
    apiRequest<T>(endpoint, {
      ...options,
      method: "PATCH",
      body:
        data !== undefined
          ? typeof data === "string" || (typeof FormData !== "undefined" && data instanceof FormData)
            ? data
            : JSON.stringify(data)
          : undefined,
    }),
};

