import { afterEach, expect, test, vi } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AnimeIndex } from "./anime-index";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
test("title upload sends the original file and displays failure without claiming replacement", async () => {
  const file = new File(["<animetitles/>"], "titles.xml", { type: "text/xml" });
  let body: BodyInit | null | undefined;
  vi.stubGlobal(
    "fetch",
    vi.fn((_path: string, init?: RequestInit) => {
      if (init?.method === "POST") {
        body = init.body;
        return Promise.resolve(new Response("Previous index preserved", { status: 400 }));
      }
      return Promise.resolve(
        new Response(JSON.stringify({ entries: 2, imported_at: "2026-01-01T00:00:00Z" }), {
          status: 200,
        }),
      );
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
      <AnimeIndex t={(k) => en[k]} />
    </QueryClientProvider>,
  );
  await userEvent.upload(screen.getByLabelText(en.animeIndexFile), file);
  expect(await screen.findByRole("alert")).toHaveProperty(
    "textContent",
    "Previous index preserved",
  );
  expect(body).toBe(file);
  expect(screen.queryByText(en.updated)).toBeNull();
});
