// Vitest config for the web app.
//
// We use the React plugin so JSX/TSX is parsed, jsdom for the DOM, and
// the `@/...` path alias mirrors tsconfig so test files import
// production code with the same paths.

import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./vitest.setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
  },
});
