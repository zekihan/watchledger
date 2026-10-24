import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AnalyticsPage, CalendarPage } from "./analytics";
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

test("calendar preserves date-only releases and places timestamp releases in the selected timezone", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) =>
      Promise.resolve(
        new Response(
          JSON.stringify(
            path.includes("month=2026-02")
              ? [
                  {
                    id: "tmdb:episode:1",
                    catalog_id: "tmdb:tv:1",
                    title: "Boundary release",
                    season: 1,
                    number: 1,
                    air_date: null,
                    air_time: "2026-01-31T23:30:00Z",
                  },
                  {
                    id: "tmdb:episode:2",
                    catalog_id: "tmdb:tv:1",
                    title: "Date-only release",
                    season: 1,
                    number: 2,
                    air_date: "2026-02-02",
                    air_time: null,
                  },
                ]
              : [],
          ),
        ),
      ),
    ),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <CalendarPage
        t={(key) => en[key]}
        user={{
          ID: "fixture",
          Name: "Fixture",
          Admin: false,
          Settings: { language: "en", timezone: "Europe/Istanbul" },
        }}
      />
    </QueryClientProvider>,
  );
  fireEvent.change(screen.getByLabelText(en.month), { target: { value: "2026-02" } });
  const boundary = await screen.findByRole("link", { name: /Boundary release/ });
  expect(boundary.closest('[role="listitem"]')?.getAttribute("aria-label")).toBe("2026-02-01");
  expect(boundary.getAttribute("href")).toBe("#detail/tv/1/season/1/episode/1");
  expect(
    screen
      .getByRole("link", { name: /Date-only release/ })
      .closest('[role="listitem"]')
      ?.getAttribute("aria-label"),
  ).toBe("2026-02-02");
  fireEvent.change(screen.getByRole("combobox", { name: en.calendar }), {
    target: { value: "list" },
  });
  expect(screen.getByText("2026-02-02")).toBeTruthy();
  expect(
    screen.getByText(
      new Date("2026-01-31T23:30:00Z").toLocaleString("en", { timeZone: "Europe/Istanbul" }),
    ),
  ).toBeTruthy();
});

test("phone calendars start with readable agenda links and retain an explicit month view", async () => {
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => ({ matches: true })),
  );
  vi.stubGlobal(
    "fetch",
    vi.fn(() =>
      Promise.resolve(
        new Response(
          JSON.stringify([
            {
              id: "tmdb:episode:3",
              catalog_id: "tmdb:tv:1",
              title: "A readable phone release title",
              season: 1,
              number: 3,
              air_date: "2026-10-01",
              air_time: null,
            },
          ]),
          { status: 200 },
        ),
      ),
    ),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <CalendarPage
        t={(key) => en[key]}
        user={{ ID: "fixture", Name: "Fixture", Admin: false, Settings: {} }}
      />
    </QueryClientProvider>,
  );
  expect(await screen.findByRole("link", { name: /A readable phone release title/ })).toBeTruthy();
  expect(screen.getByRole<HTMLSelectElement>("combobox", { name: en.calendar }).value).toBe("list");
  fireEvent.change(screen.getByRole("combobox", { name: en.calendar }), {
    target: { value: "month" },
  });
  expect(screen.getByRole("region", { name: new RegExp(en.calendar) }).tabIndex).toBe(0);
  fireEvent.change(screen.getByRole("combobox", { name: en.calendar }), {
    target: { value: "list" },
  });
  expect(screen.getByRole("link", { name: /A readable phone release title/ })).toBeTruthy();
});
