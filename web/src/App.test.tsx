import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  location.hash = "";
});
function mount() {
  return render(
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
}
test("unauthenticated UI exposes only OIDC login and language switch", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(() => Promise.resolve(new Response("login required", { status: 401 }))),
  );
  mount();
  const login = await screen.findByRole("link", { name: "Sign in with your account ↗" });
  expect(login.getAttribute("href")).toBe("/auth/login");
  await userEvent.click(screen.getByRole("button", { name: "Türkçe" }));
  expect(await screen.findByRole("link", { name: "Hesabınızla giriş yapın ↗" })).toBeTruthy();
  expect(screen.queryByRole("textbox", { name: /password/i })).toBeNull();
});
test("authenticated private library and Turkish navigation render from API", async () => {
  location.hash = "library";
  vi.stubGlobal(
    "fetch",
    vi.fn((input: string) =>
      Promise.resolve(
        new Response(
          JSON.stringify(
            input === "/api/me"
              ? {
                  ID: "synthetic",
                  Name: "Fixture User",
                  Admin: false,
                  Settings: { language: "tr" },
                }
              : input === "/api/library"
                ? [
                    {
                      id: "tmdb:movie:1",
                      title: "Synthetic Film",
                      kind: "movie",
                      base: "watching",
                      metadata: { en: { title: "Synthetic Film" } },
                      rating: 8,
                      status_id: null,
                      status_name: null,
                    },
                  ]
                : [],
          ),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      ),
    ),
  );
  mount();
  expect((await screen.findAllByText("Synthetic Film")).length).toBeGreaterThan(0);
  expect(screen.getByRole("heading", { name: "Kütüphanem" })).toBeTruthy();
  expect(screen.queryByRole("link", { name: /Yönetim/ })).toBeNull();
  await userEvent.click(screen.getByRole("button", { name: "Tablo" }));
  expect(screen.getByRole("table")).toBeTruthy();
});

test("earlier-episode confirmation defaults to only the selected episode", async () => {
  location.hash = "detail/tv/101";
  HTMLDialogElement.prototype.showModal = function () {
    this.open = true;
  };
  const writes: Record<string, unknown>[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((input: string, init?: RequestInit) => {
      let data: unknown = [];
      if (input === "/api/me")
        data = {
          ID: "fixture",
          Name: "Fixture",
          Admin: false,
          Settings: { language: "en", timezone: "UTC" },
        };
      else if (input === "/api/library")
        data = [
          {
            id: "tmdb:tv:101",
            title: "Synthetic Show",
            kind: "tv",
            base: "watching",
            metadata: {},
            rating: null,
            status_id: null,
            position: 0,
            anime_view: null,
          },
        ];
      else if (input.startsWith("/api/catalog/tv/101/seasons/"))
        data = {
          episodes: [
            {
              id: 1,
              name: "Synthetic earlier",
              season_number: 1,
              episode_number: 1,
              air_date: "2020-01-01",
            },
            {
              id: 2,
              name: "Synthetic selected",
              season_number: 1,
              episode_number: 2,
              air_date: "2020-01-02",
            },
          ],
        };
      else if (input.includes("/providers?"))
        data = { offers: null, stale: false, fetched_at: "2026-01-01" };
      else if (input.startsWith("/api/catalog/tv/101?"))
        data = {
          metadata: {
            id: 101,
            name: "Synthetic Show",
            seasons: [{ season_number: 1, name: "Season 1", episode_count: 2 }],
          },
          fetched_at: "2026-01-01",
          stale: false,
        };
      else if (input.endsWith("/track")) {
        if (typeof init?.body !== "string") throw new Error("Expected JSON request body");
        const body = JSON.parse(init.body) as Record<string, unknown>;
        writes.push(body);
        data = body.preview
          ? { earlier: [{ ID: "tmdb:episode:1" }], offer_return_watching: false }
          : { base: "watching" };
      }
      return Promise.resolve(
        new Response(JSON.stringify(data), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
    }),
  );
  mount();
  const buttons = await screen.findAllByRole("button", { name: "＋ Mark watched" });
  await userEvent.click(buttons[1]!);
  const only = await screen.findByRole("radio", { name: "Only this episode" });
  expect((only as HTMLInputElement).checked).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Save" }));
  expect(
    writes.some(
      (x) => x.preview !== true && x.episode_id === "tmdb:episode:2" && x.earlier === false,
    ),
  ).toBe(true);
});
