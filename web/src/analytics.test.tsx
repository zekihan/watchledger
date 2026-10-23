import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AnalyticsPage } from "./analytics";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
test("statistics distinguish canonical totals, partial units and uncertain links", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(() =>
      Promise.resolve(
        new Response(
          JSON.stringify({
            movies: 1,
            episodes: 4,
            estimated_minutes: 96,
            unknown_runtime: 0,
            partial_native_episodes: 2,
            partial_source_records: 3,
            unresolved_linked_records: 1,
            derived_date_units: 1,
            monthly: [{ month: "2026-02", count: 1 }],
            genres: [],
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
      <AnalyticsPage t={(k) => en[k]} />
    </QueryClientProvider>,
  );
  expect(await screen.findByText(en.canonicalAnimeStats)).toBeTruthy();
  expect(screen.getByText(`${en.partialNativeStats}: 2 · ${en.sourceRecords}: 3`)).toBeTruthy();
  expect(screen.getByText(`${en.unresolvedStats}: 1`)).toBeTruthy();
  expect(screen.getByText(`${en.derivedStatsDates}: 1`)).toBeTruthy();
  expect(screen.getByText("2026-02")).toBeTruthy();
});
