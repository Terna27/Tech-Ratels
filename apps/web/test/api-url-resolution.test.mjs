import assert from "node:assert/strict";
import test from "node:test";

test("Next.js rewrite configuration derives clean apiHost", async () => {
  const nextConfigModule = await import("../next.config.js");
  const nextConfig = nextConfigModule.default || nextConfigModule;

  assert.ok(nextConfig.rewrites, "rewrites should be configured");
  const rewrites = await nextConfig.rewrites();
  assert.ok(Array.isArray(rewrites), "rewrites should return an array");
  
  const v1Rewrite = rewrites.find(r => r.source === "/api/v1/:path*");
  assert.ok(v1Rewrite, "should have /api/v1/:path* rewrite rule");
  
  // Verify destination does not have duplicated /api/v1/api/v1
  assert.match(v1Rewrite.destination, /\/api\/v1\/:path\*$/);
  assert.doesNotMatch(v1Rewrite.destination, /\/api\/v1\/api\/v1/);
  assert.doesNotMatch(v1Rewrite.destination, /([^:])\/{2,}/); // No double slashes outside http://
});

test("Frontend apiRequest resolves correct URL and prevents duplicated /api/v1", async () => {
  const apiModule = await import("../src/lib/api.ts");
  assert.ok(apiModule.apiRequest, "apiRequest should be exported");

  // Mock global fetch to capture the resolved URL
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
    // 1. Standard endpoint without /api/v1
    await apiModule.apiRequest("/marketplace/products");
    assert.doesNotMatch(capturedUrl, /\/api\/v1\/api\/v1/);
    assert.match(capturedUrl, /\/api\/v1\/marketplace\/products$/);

    // 2. Endpoint with leading /api/v1/ (defensive deduplication)
    await apiModule.apiRequest("/api/v1/marketplace/products");
    assert.doesNotMatch(capturedUrl, /\/api\/v1\/api\/v1/);
    assert.match(capturedUrl, /\/api\/v1\/marketplace\/products$/);

    // 3. Endpoint without leading slash but with api/v1/
    await apiModule.apiRequest("api/v1/auth/login");
    assert.doesNotMatch(capturedUrl, /\/api\/v1\/api\/v1/);
    assert.match(capturedUrl, /\/api\/v1\/auth\/login$/);

    // 4. Absolute URL pass-through
    await apiModule.apiRequest("https://custom.example.com/api/v1/ping");
    assert.equal(capturedUrl, "https://custom.example.com/api/v1/ping");
  } finally {
    globalThis.fetch = originalFetch;
  }
});
