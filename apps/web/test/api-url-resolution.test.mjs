import assert from "node:assert/strict";
import test from "node:test";

test("Next.js rewrite configuration with exact production environment variables", async () => {
  // Simulate Render production environment:
  // API_URL=https://ratels.onrender.com
  // NEXT_PUBLIC_API_URL=/api/v1
  process.env.API_URL = "https://ratels.onrender.com";
  process.env.NEXT_PUBLIC_API_URL = "/api/v1";
  process.env.NODE_ENV = "production";

  // Fresh import
  const nextConfigModule = await import(`../next.config.js?t=${Date.now()}`);
  const nextConfig = nextConfigModule.default || nextConfigModule;

  assert.ok(nextConfig.rewrites, "rewrites should be configured");
  const rewrites = await nextConfig.rewrites();
  assert.ok(Array.isArray(rewrites), "rewrites should return an array");

  const v1Rewrite = rewrites.find(r => r.source === "/api/v1/:path*");
  assert.ok(v1Rewrite, "should have /api/v1/:path* rewrite rule");

  // Exact rewrite destination must point to backend host with /api/v1/:path*
  assert.equal(v1Rewrite.destination, "https://ratels.onrender.com/api/v1/:path*");
  assert.doesNotMatch(v1Rewrite.destination, /\/api\/v1\/api\/v1/);
});

test("Next.js rewrite handles various API_URL format variations safely", async () => {
  const variations = [
    { input: "https://ratels.onrender.com/", expected: "https://ratels.onrender.com/api/v1/:path*" },
    { input: "https://ratels.onrender.com/api/v1", expected: "https://ratels.onrender.com/api/v1/:path*" },
    { input: "https://ratels.onrender.com/api/v1/", expected: "https://ratels.onrender.com/api/v1/:path*" },
    { input: "https://api.ratelsworldwide.org", expected: "https://api.ratelsworldwide.org/api/v1/:path*" },
  ];

  for (const v of variations) {
    process.env.API_URL = v.input;
    process.env.NEXT_PUBLIC_API_URL = "/api/v1";
    const mod = await import(`../next.config.js?t=${Date.now()}_${Math.random()}`);
    const cfg = mod.default || mod;
    const rewrites = await cfg.rewrites();
    const rule = rewrites.find(r => r.source === "/api/v1/:path*");
    assert.equal(rule.destination, v.expected, `Mismatch for input ${v.input}`);
    assert.doesNotMatch(rule.destination, /\/api\/v1\/api\/v1/);
  }
});

test("Next.js rewrite falls back to ratels.onrender.com when only relative NEXT_PUBLIC_API_URL=/api/v1 is supplied", async () => {
  delete process.env.API_URL;
  delete process.env.INTERNAL_API_URL;
  delete process.env.BACKEND_URL;
  process.env.NEXT_PUBLIC_API_URL = "/api/v1";
  process.env.NODE_ENV = "production";

  const mod = await import(`../next.config.js?t=${Date.now()}_fallback`);
  const cfg = mod.default || mod;
  const rewrites = await cfg.rewrites();
  const rule = rewrites.find(r => r.source === "/api/v1/:path*");
  assert.equal(rule.destination, "https://ratels.onrender.com/api/v1/:path*");
});

test("Frontend apiRequest in browser environment uses relative /api/v1 same-origin paths", async () => {
  // In the browser, window is defined
  const originalWindow = globalThis.window;
  globalThis.window = { location: { origin: "https://ratels-web.onrender.com" } };

  const apiModule = await import(`../src/lib/api.ts?t=${Date.now()}`);
  let capturedUrl = "";
  let capturedInit = null;

  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, init) => {
    capturedUrl = url.toString();
    capturedInit = init;
    return {
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ status: "ok" }),
    };
  };

  try {
    // 1. Standard relative endpoint
    await apiModule.apiRequest("/marketplace/products");
    assert.equal(capturedUrl, "/api/v1/marketplace/products");
    assert.equal(capturedInit?.credentials, "same-origin");

    // 2. Caller providing /api/v1/... (defensive deduplication)
    await apiModule.apiRequest("/api/v1/marketplace/products");
    assert.equal(capturedUrl, "/api/v1/marketplace/products");

    // 3. Caller providing api/v1/... without leading slash
    await apiModule.apiRequest("api/v1/auth/login");
    assert.equal(capturedUrl, "/api/v1/auth/login");

    // 4. Mutating request attaches CSRF header
    apiModule.setCsrfToken("test-csrf-token-12345");
    await apiModule.apiRequest("/auth/logout", { method: "POST" });
    assert.equal(capturedUrl, "/api/v1/auth/logout");
    assert.equal(capturedInit?.headers["X-CSRF-Token"], "test-csrf-token-12345");
  } finally {
    globalThis.fetch = originalFetch;
    if (originalWindow !== undefined) {
      globalThis.window = originalWindow;
    } else {
      delete globalThis.window;
    }
  }
});

test("Frontend apiRequest in SSR environment resolves absolute backend URL without duplicating /api/v1", async () => {
  // In SSR (Node.js), window is undefined
  delete globalThis.window;
  process.env.API_URL = "https://ratels.onrender.com";
  process.env.NEXT_PUBLIC_API_URL = "/api/v1";

  const apiModule = await import(`../src/lib/api.ts?t=${Date.now()}_ssr`);
  let capturedUrl = "";

  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url) => {
    capturedUrl = url.toString();
    return {
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "application/json" }),
      json: async () => ({ status: "ok" }),
    };
  };

  try {
    await apiModule.apiRequest("/marketplace/products");
    assert.equal(capturedUrl, "https://ratels.onrender.com/api/v1/marketplace/products");
    assert.doesNotMatch(capturedUrl, /\/api\/v1\/api\/v1/);

    await apiModule.apiRequest("/api/v1/marketplace/products");
    assert.equal(capturedUrl, "https://ratels.onrender.com/api/v1/marketplace/products");
    assert.doesNotMatch(capturedUrl, /\/api\/v1\/api\/v1/);
  } finally {
    globalThis.fetch = originalFetch;
  }
});
