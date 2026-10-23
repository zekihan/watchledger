import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AlternateView } from "./alternate";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
test("alternate view explains derived history and never manufactures an aggregate rating", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(() =>
      Promise.resolve(
        new Response(
          JSON.stringify({
            structure: "native",
            incomplete: true,
            titles: [
              {
                id: "native",
                title: "Synthetic native",
                kind: "anime",
                base: "watching",
                rating: 8,
                watched: 1,
                total: 2,
                incomplete: true,
                episodes: [
                  {
                    id: "e1",
                    season: 1,
                    number: 1,
                    name: "Recorded",
                    retired: false,
                    state: { direct: true, derived: false, blocked: false, sources: ["t1"] },
                  },
                  {
                    id: "e2",
                    season: 1,
                    number: 2,
                    name: "Pending",
                    retired: false,
                    state: { direct: false, derived: false, blocked: true, sources: ["t2"] },
                  },
                ],
              },
              {
                id: "show",
                title: "Synthetic show",
                kind: "tv",
                base: null,
                rating: null,
                watched: 1,
                total: 1,
                incomplete: false,
                episodes: [
                  {
                    id: "t1",
                    season: 2,
                    number: 1,
                    name: "Derived",
                    retired: false,
                    state: { direct: false, derived: true, blocked: false, sources: ["e1"] },
                  },
                ],
              },
            ],
          }),
          { status: 200 },
        ),
      ),
    ),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <AlternateView id="native" t={(k) => en[k]} />
    </QueryClientProvider>,
  );
  expect(await screen.findByText(en.derivedWatch)).toBeTruthy();
  expect(screen.getByText(en.mappingNeedsReview, { exact: false })).toBeTruthy();
  expect(screen.getByText(en.alternateHistoryNotice)).toBeTruthy();
  expect(screen.getAllByRole("heading", { level: 3 })[0]?.textContent).toContain(
    "Synthetic native",
  );
  expect(screen.queryByRole("button", { name: en.watched })).toBeNull();
  expect(screen.getByText(/Synthetic show · 2:1 · Derived/)).toBeTruthy();
});
