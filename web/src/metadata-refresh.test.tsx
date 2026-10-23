import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { en } from "./i18n";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  location.hash = "";
});

test("manual refresh retains the provider warning and original timestamp until a successful retry", async () => {
  location.hash = "detail/movie/72";
  const cached = {
    metadata: { id: 72, title: "Cached movie" },
    fetched_at: "2026-01-01T00:00:00Z",
    stale: false,
  };
  let refreshes = 0;
  let reads = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      let data: unknown = [];
      if (path === "/api/me")
        data = { ID: "fixture", Name: "Fixture", Admin: false, Settings: { language: "en" } };
      else if (path.startsWith("/api/catalog/movie/72/refresh")) {
        refreshes++;
        data =
          refreshes === 1
            ? { ...cached, stale: true, warning: "Provider unavailable; cached metadata retained" }
            : {
                ...cached,
                metadata: { id: 72, title: "Updated movie" },
                fetched_at: "2026-01-02T00:00:00Z",
              };
      } else if (path.startsWith("/api/catalog/movie/72?")) {
        reads++;
        data = cached;
      }
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
  expect(await screen.findByRole("heading", { name: "Cached movie", level: 1 })).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: en.refresh }));
  expect(await screen.findByText("Provider unavailable; cached metadata retained")).toBeTruthy();
  expect(screen.getByText(new RegExp(en.stale))).toBeTruthy();
  expect(reads).toBe(1);
  await userEvent.click(screen.getByRole("button", { name: en.refresh }));
  expect(await screen.findByRole("heading", { name: "Updated movie", level: 1 })).toBeTruthy();
  expect(screen.queryByText("Provider unavailable; cached metadata retained")).toBeNull();
  expect(screen.queryByText(new RegExp(en.stale))).toBeNull();
  expect(reads).toBe(1);
});
