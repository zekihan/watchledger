import { afterEach, expect, test, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
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

function mountTrackingFixture(delayedPreview?: Promise<Response>) {
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
        if (body.preview && delayedPreview) return delayedPreview;
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
  return writes;
}
test("earlier-episode confirmation defaults to only the selected episode", async () => {
  const writes = mountTrackingFixture();
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

test.each(["button", "escape", "navigation"])(
  "tracking cancellation via %s makes no private write",
  async (mode) => {
    const writes = mountTrackingFixture();
    const buttons = await screen.findAllByRole<HTMLButtonElement>("button", {
      name: "＋ Mark watched",
    });
    const selected = buttons[1];
    if (!selected) throw new Error("missing selected episode");
    await userEvent.click(selected);
    const dialog = await screen.findByRole("dialog");
    if (mode === "button") await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    else if (mode === "escape")
      fireEvent(dialog, new Event("cancel", { bubbles: true, cancelable: true }));
    else await userEvent.click(screen.getByRole("link", { name: "My library" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    if (mode !== "navigation") {
      await waitFor(() => expect(selected.disabled).toBe(false));
      await waitFor(() => expect(document.activeElement).toBe(selected));
    }
    await act(async () => {
      await Promise.resolve();
    });
    expect(writes.filter((body) => body.preview !== true)).toHaveLength(0);
  },
);

test("a tracking preview arriving after navigation cannot open a prompt or apply history", async () => {
  let finish: ((response: Response) => void) | undefined;
  const pending = new Promise<Response>((resolve) => {
    finish = resolve;
  });
  const writes = mountTrackingFixture(pending);
  const buttons = await screen.findAllByRole("button", { name: "＋ Mark watched" });
  const selected = buttons[1];
  if (!selected) throw new Error("missing selected episode");
  await userEvent.click(selected);
  await waitFor(() => expect(writes).toHaveLength(1));
  await userEvent.click(screen.getByRole("link", { name: "My library" }));
  await act(async () => {
    finish?.(
      new Response(
        JSON.stringify({ earlier: [{ ID: "tmdb:episode:1" }], offer_return_watching: false }),
        { status: 200 },
      ),
    );
    await Promise.resolve();
  });
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(writes.filter((body) => body.preview !== true)).toHaveLength(0);
});

test("global shortcuts do not steal focus from a tracking confirmation dialog", async () => {
  const writes = mountTrackingFixture();
  const buttons = await screen.findAllByRole("button", { name: "＋ Mark watched" });
  const selected = buttons[1];
  if (!selected) throw new Error("missing selected episode");
  await userEvent.click(selected);
  await screen.findByRole("dialog");
  const save = screen.getByRole("button", { name: "Save" });
  save.focus();
  await userEvent.keyboard("/");
  expect(document.activeElement).toBe(save);
  await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(writes.filter((body) => body.preview !== true)).toHaveLength(0);
});

test("Up next links aggregate views to their native anchor and reorders displayed cards", async () => {
  const writes: unknown[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((input: string, init?: RequestInit) => {
      if (input === "/api/up-next/order") {
        if (typeof init?.body !== "string") throw new Error("Expected request body");
        writes.push(JSON.parse(init.body) as unknown);
        return Promise.resolve(new Response(null, { status: 204 }));
      }
      const data =
        input === "/api/me"
          ? { ID: "fixture", Name: "Fixture", Admin: false, Settings: { language: "en" } }
          : input === "/api/dashboard"
            ? {
                up_next: [
                  {
                    catalog_id: "tmdb:tv:1",
                    episode_id: "tmdb:episode:11",
                    title: "Aggregate show",
                    season: 2,
                    number: 7,
                    view_anchor: "anidb:anime:1",
                  },
                  {
                    catalog_id: "anidb:anime:2",
                    episode_id: "anidb:episode:22",
                    title: "Unmapped native",
                    season: 1,
                    number: 1,
                    incomplete: true,
                  },
                ],
                activity: [],
                upcoming: [],
                statistics: { movies: 0, episodes: 0, monthly: [] },
              }
            : [];
      return Promise.resolve(
        new Response(JSON.stringify(data), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
    }),
  );
  mount();
  const card = await screen.findByRole("link", { name: /Aggregate show/ });
  expect(card.getAttribute("href")).toBe("#views/anidb%3Aanime%3A1");
  expect(screen.getByRole("link", { name: /Unmapped native/ }).getAttribute("href")).toBe(
    "#detail/anime/2/season/1/episode/22",
  );
  await userEvent.click(screen.getByRole("button", { name: "Move up: Unmapped native" }));
  await waitFor(() => expect(writes).toEqual([{ ids: ["anidb:anime:2", "tmdb:tv:1"] }]));
});

test("skip navigation preserves the route and arrow keys do not steal focus from controls", async () => {
  location.hash = "library";
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) =>
      Promise.resolve(
        new Response(
          JSON.stringify(
            path === "/api/me"
              ? { ID: "fixture", Name: "Fixture", Admin: false, Settings: { language: "en" } }
              : path === "/api/library"
                ? [
                    {
                      id: "tmdb:movie:1",
                      title: "Focus fixture",
                      kind: "movie",
                      base: "watching",
                      metadata: { en: { title: "Focus fixture" } },
                    },
                  ]
                : [],
          ),
        ),
      ),
    ),
  );
  mount();
  await screen.findAllByText("Focus fixture");
  const skip = screen.getByRole("link", { name: "Skip to content" });
  fireEvent.click(skip);
  expect(document.activeElement).toBe(screen.getByRole("main"));
  expect(location.hash).toBe("#library");
  const table = screen.getByRole("button", { name: "Table" });
  table.focus();
  fireEvent.keyDown(table, { key: "ArrowRight" });
  expect(document.activeElement).toBe(table);
  expect(
    screen
      .getByRole("navigation", { name: "Main navigation" })
      .querySelector('[aria-current="page"]')
      ?.getAttribute("href"),
  ).toBe("#library");
});

test("failed logout stays visible rather than silently reporting success", async () => {
  location.hash = "settings";
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) =>
      Promise.resolve(
        path === "/api/logout"
          ? new Response("Logout unavailable", { status: 503 })
          : new Response(
              JSON.stringify(
                path === "/api/me"
                  ? { ID: "fixture", Name: "Fixture", Admin: false, Settings: { language: "en" } }
                  : path.startsWith("/api/providers/")
                    ? { results: [] }
                    : [],
              ),
            ),
      ),
    ),
  );
  mount();
  await userEvent.click(await screen.findByRole("button", { name: "Sign out" }));
  expect((await screen.findByRole("alert")).textContent).toContain("Logout unavailable");
  expect(screen.getByRole("heading", { name: "Settings" })).toBeTruthy();
});
