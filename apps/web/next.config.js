/** @type {import('next').NextConfig} */
const rawApiUrl = process.env.INTERNAL_API_URL || process.env.API_URL || process.env.BACKEND_URL || process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8000";
const cleanApiUrl = rawApiUrl.trim().replace(/\/+$/, "");
const apiHost = cleanApiUrl.replace(/\/api\/v1\/?$/, "");

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
    return [
      {
        source: "/api/v1/:path*",
        destination: `${apiHost}/api/v1/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
