// Web app ESLint flat config.
// Extends the workspace root config and layers Next.js + a small set of
// app-local conventions on top.
import root from "../../eslint.config.js";
import { FlatCompat } from "@eslint/eslintrc";

const compat = new FlatCompat({ baseDirectory: import.meta.dirname });

const config = [
  ...root,
  ...compat.extends("next/core-web-vitals"),
  {
    ignores: ["**/.next/**", "**/dist/**", "**/coverage/**", "next-env.d.ts"],
  },
  {
    files: ["src/**/*.{ts,tsx}", "*.{ts,mts,js,mjs}"],
    rules: {
      // Next.js handles default exports for pages/layouts/route handlers.
      "import/no-default-export": "off",
      // The Next preset replaces the parser, so type-aware rules from
      // typescript-eslint can't run here. Disable them in this app and
      // rely on `tsc --noEmit` for type correctness.
      "@typescript-eslint/consistent-type-imports": "off",
    },
  },
];

export default config;
