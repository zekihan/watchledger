import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  location.hash = "";
});
function mount() {
  location.hash = "detail/tv/42";
  HTMLDialogElement.prototype.showModal = function () {
    this.open = true;
  };
  const writes: Record<string, unknown>[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      let data: unknown = [];
      if (path === "/api/me")
        data = {
          ID: "synthetic",
          Name: "Fixture",
          Admin: false,
          Settings: { language: "en", timezone: "UTC" },
        };
      else if (path === "/api/library")
        data = [
          {
            id: "tmdb:tv:42",
            title: "Synthetic",
            kind: "tv",
            base: "watching",
            metadata: {},
            position: 0,
          },
        ];
      else if (path.startsWith("/api/catalog/tv/42/seasons"))
        data = {
          episodes: [{ id: 501, name: "Linked episode", season_number: 1, episode_number: 1 }],
        };
      else if (path.startsWith("/api/catalog/tv/42?"))
        data = {
          metadata: {
            id: 42,
            name: "Synthetic",
            seasons: [{ season_number: 1, name: "Season 1", episode_count: 1 }],
          },
          fetched_at: "2026-01-01T00:00:00Z",
        };
      else if (path.endsWith("/effective"))
        data = {
          "tmdb:episode:501": { direct: false, derived: true, sources: ["anidb:episode:1"] },
        };
      else if (path.endsWith("/alternate"))
        data = { structure: "show", titles: [], incomplete: false };
      else if (path.endsWith("/track")) {
        const body = JSON.parse(typeof init?.body === "string" ? init.body : "{}") as Record<
          string,
          unknown
        >;
        writes.push(body);
        data = {
          earlier: [],
          offer_return_watching: false,
          linked_plan: "synthetic-plan",
          linked_changes: [
            { episode_id: "anidb:episode:1", catalog_id: "anidb:anime:1", title: "Native source" },
          ],
        };
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
  return writes;
}
test("linked unwatch cancels without writes and submits the reviewed plan only after explicit confirmation", async () => {
  const writes = mount();
  await userEvent.click(await screen.findByRole("button", { name: new RegExp(en.unwatched) }));
  const dialog = await screen.findByRole("dialog");
  expect(within(dialog).getByText(/Native source/)).toBeTruthy();
  await userEvent.click(within(dialog).getByRole("button", { name: en.cancel }));
  expect(writes.filter((w) => !w.preview)).toHaveLength(0);
  await userEvent.click(screen.getByRole("button", { name: new RegExp(en.unwatched) }));
  const second = await screen.findByRole("dialog");
  await userEvent.click(within(second).getByRole("radio", { name: en.unwatchLinked }));
  await userEvent.click(within(second).getByRole("button", { name: en.save }));
  await waitFor(() => expect(writes.filter((w) => !w.preview)).toHaveLength(1));
  expect(writes.find((w) => !w.preview)).toMatchObject({
    watched: false,
    include_linked: true,
    linked_plan: "synthetic-plan",
  });
});
