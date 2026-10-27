/** @type {import('next').NextConfig} */
function getApiHost() {
  const rawApiTarget = process.env.INTERNAL_API_URL || process.env.API_URL || process.env.BACKEND_URL;
  let host = "";
  if (rawApiTarget && rawApiTarget.trim()) {
    const clean = rawApiTarget.trim().replace(/\/+$/, "");
    host = clean.replace(/\/api\/v1\/?$/, "");
  } else if (process.env.NEXT_PUBLIC_API_URL && (process.env.NEXT_PUBLIC_API_URL.startsWith("http://") || process.env.NEXT_PUBLIC_API_URL.startsWith("https://"))) {
    const clean = process.env.NEXT_PUBLIC_API_URL.trim().replace(/\/+$/, "");
    host = clean.replace(/\/api\/v1\/?$/, "");
  }

  // Fallback for native Render deployment when NEXT_PUBLIC_API_URL is relative (/api/v1)
  if (!host || !host.startsWith("http")) {
    host = process.env.NODE_ENV === "production" ? "https://ratels.onrender.com" : "http://127.0.0.1:8000";
  }
  return host;
}

const nextConfig = {
  reactStrictMode: true,
  images: {
    remotePatterns: [
      { protocol: "https", hostname: "pub-82990f82f02446b5a11817fdce96912d.r2.dev" },
      { protocol: "https", hostname: "*.r2.dev" },
      { protocol: "https", hostname: "**.r2.dev" },
      { protocol: "https", hostname: "**.r2.cloudflarestorage.com" },
      { protocol: "https", hostname: "**.ratels.org.ng" },
      { protocol: "https", hostname: "ratels.org.ng" },
      { protocol: "https", hostname: "images.unsplash.com" },
      { protocol: "http", hostname: "localhost" },
      { protocol: "http", hostname: "127.0.0.1" },
    ],
  },
  async rewrites() {
    const host = getApiHost();
    return [
      {
        source: "/api/v1/:path*",
        destination: `${host}/api/v1/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
