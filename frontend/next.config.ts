import type { NextConfig } from "next";
import { fileURLToPath } from "node:url";
import { loadEnvConfig } from "@next/env";

// mountabo keeps a single .env at the repository root, shared by the Go backend
// and this Next app. Next only loads .env files from the project directory, so
// we load the parent (repo root) here with Next's own loader to populate this
// config process's environment.
// forceReload (4th arg) is required: Next loads the frontend-dir env first and
// caches it, so without it this call to load the repo root would be a no-op.
const repoRoot = fileURLToPath(new URL("..", import.meta.url));
loadEnvConfig(repoRoot, process.env.NODE_ENV !== "production", undefined, true);

const nextConfig: NextConfig = {
  // Loading the root .env above only populates the config process — it does not
  // by itself reach the request runtime — so forward the values the server-side
  // OAuth route handlers need. Only the non-secret client id and backend URL are
  // exposed here; the GitHub client secret stays with the Go backend and is
  // never sent to the frontend.
  env: {
    GITHUB_CLIENT_ID: process.env.GITHUB_CLIENT_ID ?? "",
    MOUNTABO_BACKEND: process.env.MOUNTABO_BACKEND ?? "",
  },
};

export default nextConfig;
