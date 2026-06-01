/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Allow importing from sibling workspace packages.
  transpilePackages: ["@doclens/api-client"],
  // Surface real type/lint errors during builds.
  typescript: { ignoreBuildErrors: false },
  eslint: { ignoreDuringBuilds: false },
  // Trim image optimization to known sources.
  images: {
    remotePatterns: [
      { protocol: "https", hostname: "images.clerk.dev" },
      { protocol: "https", hostname: "img.clerk.com" },
    ],
  },
  // Build-time env defaults so `next build` succeeds in CI without a
  // real Clerk app. Real keys MUST be supplied via .env.local in
  // development or via the deployment platform in production. These
  // strings only satisfy Clerk's key-format validator at build time;
  // authentication fails at runtime unless overridden.
  env: {
    NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY:
      process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY ??
      "pk_test_Y2xlcmstZGV2LXBsYWNlaG9sZGVyLWtleS5leGFtcGxlJA",
    CLERK_SECRET_KEY:
      process.env.CLERK_SECRET_KEY ?? "sk_test_clerk-dev-placeholder-key-not-real",
  },
};

export default nextConfig;
