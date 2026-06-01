// Tailwind theme — every token here points to a CSS variable defined
// in src/app/globals.css. Two consequences:
//   1. Theme switching is `[data-theme="light"]` on <html>, no
//      Tailwind class plumbing required.
//   2. The values in DESIGN.md, globals.css, and tailwind.config.ts
//      are kept in lockstep by file proximity, not duplicated logic.

import type { Config } from "tailwindcss";

const config: Config = {
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        // Surfaces resolved per theme via --bg / --surface / --border.
        bg: "var(--bg)",
        surface: "var(--surface)",
        border: "var(--border)",
        text: "var(--text)",
        "text-strong": "var(--text-strong)",
        muted: "var(--muted)",

        // Brand and raw palette stay theme-stable.
        brand: {
          DEFAULT: "var(--color-brand)",
          soft: "var(--color-brand-soft)",
        },
        gray: {
          100: "var(--color-gray-100)",
          400: "var(--color-gray-400)",
          600: "var(--color-gray-600)",
          800: "var(--color-gray-800)",
          950: "var(--color-gray-950)",
        },
        success: {
          DEFAULT: "var(--color-success)",
          surface: "var(--color-success-surface)",
        },
        error: {
          DEFAULT: "var(--color-error)",
          surface: "var(--color-error-surface)",
        },
        warning: {
          DEFAULT: "var(--color-warning)",
          surface: "var(--color-warning-surface)",
        },
        info: {
          DEFAULT: "var(--color-info)",
          surface: "var(--color-info-surface)",
        },
      },
      fontFamily: {
        sans: ["var(--font-sans)"],
        mono: ["var(--font-mono)"],
      },
      fontSize: {
        // Roles from DESIGN.md (size + line-height pairs).
        display: ["48px", { lineHeight: "1.1", fontWeight: "700" }],
        heading: ["32px", { lineHeight: "1.2", fontWeight: "600" }],
        title: ["20px", { lineHeight: "1.3", fontWeight: "600" }],
        label: ["13px", { lineHeight: "1.4", fontWeight: "500" }],
        caption: ["12px", { lineHeight: "1.4", fontWeight: "400" }],
      },
      spacing: {
        1: "var(--space-1)",
        2: "var(--space-2)",
        3: "var(--space-3)",
        4: "var(--space-4)",
        6: "var(--space-6)",
        8: "var(--space-8)",
        12: "var(--space-12)",
      },
      borderRadius: {
        sm: "var(--radius-sm)",
        md: "var(--radius-md)",
        lg: "var(--radius-lg)",
        full: "var(--radius-full)",
      },
      boxShadow: {
        sm: "var(--shadow-sm)",
        md: "var(--shadow-md)",
        lg: "var(--shadow-lg)",
      },
      transitionTimingFunction: {
        DEFAULT: "var(--ease)",
      },
      transitionDuration: {
        fast: "var(--motion-fast)",
        base: "var(--motion-base)",
        slow: "var(--motion-slow)",
      },
    },
  },
  plugins: [],
};

export default config;
