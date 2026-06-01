// Tiny unit test that locks the visible label for each status.
// Catches accidental enum drift between the API and the UI.

import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";

import { StatusBadge } from "../StatusBadge";

describe("StatusBadge", () => {
  const cases = [
    ["queued", "Queued"],
    ["extracting", "Extracting"],
    ["ready", "Ready"],
    ["failed", "Failed"],
    ["deleted", "Deleted"],
  ] as const;

  for (const [status, label] of cases) {
    it(`renders the canonical label for ${status}`, () => {
      const { unmount } = render(<StatusBadge status={status} />);
      expect(screen.getByText(label)).toBeInTheDocument();
      unmount();
    });
  }
});
