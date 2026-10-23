import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { en } from "./i18n";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  location.hash = "";
});

test("availability can retry independently while cached detail and private rating remain usable", async () => {
  location.hash = "detail/movie/71";
  let offerRequests = 0;
  const writes: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      if (init?.method && init.method !== "GET") writes.push(path);
      let data: unknown = [];
      if (path === "/api/me")
        data = {
          ID: "fixture",
          Name: "Fixture",
          Admin: false,
          Settings: { language: "en", country: "TR" },
        };
      else if (path === "/api/library")
        data = [
          {
            id: "tmdb:movie:71",
            kind: "movie",
            title: "Cached movie",
            base: "planned",
            rating: 8,
            position: 0,
            metadata: {},
          },
        ];
      else if (path.startsWith("/api/catalog/movie/71/providers")) {
        offerRequests++;
        if (offerRequests === 1)
          return Promise.resolve(new Response("TMDB is unavailable; retry later", { status: 502 }));
        data = {
          offers: {
            flatrate: [{ provider_id: 1, provider_name: "Fixture service" }],
            link: "https://example.com/availability",
          },
          fetched_at: "2026-01-01T00:00:00Z",
          stale: true,
        };
      } else if (path.startsWith("/api/catalog/movie/71?"))
        data = {
          metadata: { id: 71, title: "Cached movie" },
          fetched_at: "2026-01-01T00:00:00Z",
          stale: false,
        };
      else if (path.endsWith("/alternate"))
        data = { structure: "native", incomplete: false, titles: [] };
      return Promise.resolve(new Response(JSON.stringify(data), { status: 200 }));
    }),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <App />
    </QueryClientProvider>,
  );
  const alert = await screen.findByRole("alert");
  expect(screen.getByRole("heading", { name: "Cached movie", level: 1 })).toBeTruthy();
  expect(screen.getByRole<HTMLSelectElement>("combobox", { name: "Your rating" }).value).toBe("8");
  await userEvent.click(within(alert).getByRole("button", { name: en.retry }));
  expect(await screen.findByRole("link", { name: "Fixture service ↗" })).toBeTruthy();
  expect(screen.getByText(/JustWatch · Showing cached data/)).toBeTruthy();
  expect(screen.queryByRole("alert")).toBeNull();
  expect(offerRequests).toBe(2);
  expect(writes).toEqual([]);
});
