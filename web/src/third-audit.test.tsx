import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { en } from "./i18n";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  location.hash = "";
});
function mount(count = 0) {
  const entries = Array.from({ length: count }, (_, i) => ({
    id: `tmdb:movie:${i + 1}`,
    title: `Synthetic ${String(i).padStart(3, "0")}`,
    kind: "movie",
    base: "watching",
    rating: null,
    added_at: "2026-01-01",
    metadata: { selected: { title: "Synthetic" } },
  }));
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) =>
      Promise.resolve(
        new Response(
          JSON.stringify(
            path === "/api/me"
              ? { ID: "synthetic", Name: "User", Admin: false, Settings: { language: "en" } }
              : path === "/api/library"
                ? entries
                : [],
          ),
          { status: 200 },
        ),
      ),
    ),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <App />
    </QueryClientProvider>,
  );
}
test.each(["#search/%", "#views/%E0%A4%A"])(
  "invalid route %s keeps navigation usable",
  async (hash) => {
    location.hash = hash;
    mount();
    expect((await screen.findByRole("alert")).textContent).toContain(en.invalidLink);
    await userEvent.click(screen.getByRole("link", { name: en.library }));
    expect(await screen.findByRole("heading", { name: en.library })).toBeTruthy();
  },
);
test("large library renders bounded pages and filters across all entries", async () => {
  location.hash = "#library";
  mount(1000);
  expect((await screen.findAllByText("Synthetic 000")).length).toBeGreaterThan(0);
  expect(document.querySelectorAll(".poster-card")).toHaveLength(48);
  expect(screen.queryByText("Synthetic 048")).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: en.nextPage }));
  expect((await screen.findAllByText("Synthetic 048")).length).toBeGreaterThan(0);
  await userEvent.type(
    within(screen.getByRole("main")).getByRole("textbox", { name: en.search }),
    "Synthetic 999",
  );
  await waitFor(() => expect(document.querySelectorAll(".poster-card")).toHaveLength(1));
  expect(screen.getAllByText("Synthetic 999").length).toBeGreaterThan(0);
});
