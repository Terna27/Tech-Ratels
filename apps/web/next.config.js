/** @type {import('next').NextConfig} */
const rawApiUrl = process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8000";
const apiHost = rawApiUrl.replace(/\/api\/v1\/?$/, "");

const nextConfig = {
  reactStrictMode: true,
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
