// Smoke test for the auth-gated app layout.
//
// We mock `@clerk/nextjs` so the layout renders without network. The
// purpose is to lock in that the layout exposes the expected nav
// affordances (Library, Upload, Search, UserButton) and renders its
// children. Real Clerk integration is exercised via end-to-end tests
// in M9.

import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";

vi.mock("@clerk/nextjs", () => ({
  // Render a recognizable stand-in for the Clerk UserButton.
  UserButton: () => <button aria-label="user-menu">user</button>,
}));

import AppLayout from "../layout";

describe("AppLayout", () => {
  it("renders nav links and the user menu around its children", () => {
    render(
      <AppLayout>
        <p data-testid="child">hello</p>
      </AppLayout>,
    );

    // Nav links scoped to the protected app.
    expect(screen.getByRole("link", { name: "Library" })).toHaveAttribute("href", "/library");
    expect(screen.getByRole("link", { name: "Upload" })).toHaveAttribute("href", "/upload");
    expect(screen.getByRole("link", { name: "Search" })).toHaveAttribute("href", "/search");

    // User menu is hydrated from the Clerk mock.
    expect(screen.getByRole("button", { name: "user-menu" })).toBeInTheDocument();

    // Children pass through to the main content area.
    expect(screen.getByTestId("child")).toHaveTextContent("hello");
  });
});
