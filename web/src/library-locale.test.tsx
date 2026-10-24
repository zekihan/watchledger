import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { tr } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  location.hash = "";
});
function mount() {
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <App />
    </QueryClientProvider>,
  );
}
test("library custom and base filters coexist and Turkish search respects dotted/dotless I", async () => {
  location.hash = "library";
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) =>
      Promise.resolve(
        new Response(
          JSON.stringify(
            path === "/api/me"
              ? {
                  ID: "fixture",
                  Name: "Fixture",
                  Admin: false,
                  Settings: { language: "tr", metadata_language: "en" },
                }
              : path === "/api/library"
                ? [
                    {
                      id: "tmdb:movie:1",
                      title: "Işık",
                      kind: "movie",
                      metadata: {},
                      base: "watching",
                      status_id: "weekend",
                      status_name: "Hafta sonu",
                      rating: 8,
                      added_at: "2026-01-01",
                    },
                    {
                      id: "tmdb:movie:2",
                      title: "İz",
                      kind: "movie",
                      metadata: {},
                      base: "watching",
                      status_id: null,
                      status_name: null,
                      rating: 4,
                      added_at: "2026-01-02",
                    },
                    {
                      id: "anidb:anime:3",
                      title: "Anime",
                      kind: "anime",
                      metadata: {},
                      base: "planned",
                      status_id: null,
                      status_name: null,
                      rating: null,
                      added_at: "2026-01-03",
                    },
                  ]
                : [],
          ),
          { status: 200 },
        ),
      ),
    ),
  );
  mount();
  await screen.findByRole("link", { name: /Işık/ });
  await userEvent.selectOptions(screen.getByRole("combobox", { name: tr.status }), "watching");
  expect(screen.getByRole("link", { name: /Işık/ })).toBeTruthy();
  expect(screen.getByRole("link", { name: /İz/ })).toBeTruthy();
  expect(screen.queryByRole("link", { name: /Anime/ })).toBeNull();
  await userEvent.selectOptions(
    screen.getByRole("combobox", { name: tr.status }),
    "custom:weekend",
  );
  expect(screen.queryByRole("link", { name: /İz/ })).toBeNull();
  const search = screen
    .getAllByRole("textbox", { name: tr.search })
    .find((input) => input.getAttribute("placeholder") === tr.title);
  if (!search) throw new Error("Missing library search");
  await userEvent.type(search, " ışık ");
  expect(screen.getByRole("link", { name: /Işık/ })).toBeTruthy();
  await userEvent.selectOptions(screen.getByRole("combobox", { name: tr.kind }), "anime");
  expect(screen.queryByRole("link", { name: /Işık/ })).toBeNull();
});
test.each([
  ["tr-TR", "tr"],
  ["fr-FR", "en"],
])("browser %s initializes supported language with fallback", async (browser, expected) => {
  vi.spyOn(window.navigator, "language", "get").mockReturnValue(browser);
  vi.stubGlobal(
    "fetch",
    vi.fn(() => Promise.resolve(new Response("login required", { status: 401 }))),
  );
  mount();
  await screen.findByRole("link", {
    name: expected === "tr" ? "Hesabınızla giriş yapın ↗" : "Sign in with your account ↗",
  });
  expect(document.documentElement.lang).toBe(expected);
});
