import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import userEvent from "@testing-library/user-event";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  location.hash = "";
});
test("pending status save blocks stale rating writes", async () => {
  location.hash = "detail/movie/880020";
  const writes: Record<string, unknown>[] = [];
  const releases: (() => void)[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      let data: unknown = [];
      if (path === "/api/me")
        data = {
          ID: "audit",
          Name: "Audit",
          Admin: false,
          Settings: { language: "en", timezone: "UTC" },
        };
      else if (path === "/api/library")
        data = [
          {
            id: "tmdb:movie:880020",
            kind: "movie",
            title: "Audit",
            base: "watching",
            rating: 5,
            position: 0,
            metadata: {},
          },
        ];
      else if (path.startsWith("/api/catalog/movie/880020?"))
        data = { metadata: { id: 880020, title: "Audit" }, fetched_at: "2026-01-01T00:00:00Z" };
      else if (path.endsWith("/alternate"))
        data = { structure: "native", incomplete: false, titles: [] };
      else if (path === "/api/library/tmdb:movie:880020" && init?.method === "PUT") {
        writes.push(
          JSON.parse(typeof init.body === "string" ? init.body : "{}") as Record<string, unknown>,
        );
        return new Promise<Response>((resolve) =>
          releases.push(() => resolve(new Response(null, { status: 204 }))),
        );
      }
      return Promise.resolve(new Response(JSON.stringify(data), { status: 200 }));
    }),
  );
  render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
        })
      }
    >
      <App />
    </QueryClientProvider>,
  );
  const user = userEvent.setup();
  await user.selectOptions(await screen.findByRole("combobox", { name: en.status }), "hold");
  await waitFor(() => expect(writes).toHaveLength(1));
  const rating = screen.getByRole<HTMLSelectElement>("combobox", { name: en.rating });
  expect(rating.disabled).toBe(true);
  expect(screen.getByRole<HTMLSelectElement>("combobox", { name: en.status }).disabled).toBe(true);
  await user.selectOptions(rating, "9");
  expect(writes).toHaveLength(1);
  expect(writes[0]).toMatchObject({ base: "hold", rating: 5 });
  releases.forEach((release) => release());
});
