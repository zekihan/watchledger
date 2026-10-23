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
