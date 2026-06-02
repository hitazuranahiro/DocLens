// Playwright configuration for the v0.1 smoke test.
//
// The smoke test boots the API and worker via docker compose and
// the web app via `next start`, then walks one user through the
// full flow: sign in → upload → wait for ready → read → search →
// delete. We don't run a separate dev server here because the test
// expects a built bundle (matching production); pass-through env
// vars route the web client to the API the compose stack exposes.

import { defineConfig, devices } from "@playwright/test";

const PORT = process.env.PORT ?? "3000";
const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? `http://localhost:${PORT}`;

export default defineConfig({
  testDir: "./e2e",
  // Smoke is a single sequence; 60s is plenty for the slow path
  // (extraction worker takes ~10–30s on a fresh container).
  timeout: 90_000,
  expect: { timeout: 10_000 },
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
