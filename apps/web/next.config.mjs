/** @type {import('next').NextConfig} */
const nextConfig = {
  output: process.env.NEXT_OUTPUT_MODE === 'default' ? undefined : 'standalone',
  reactStrictMode: true,
};

export default nextConfig;
