import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MappingPage } from "./admin";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});
test("mapping review names connected episodes and confirms only the reviewed revision", async () => {
  const writes: unknown[] = [];
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      if (path.endsWith("/confirm")) {
        if (typeof init?.body !== "string") throw new Error("Missing body");
        writes.push(JSON.parse(init.body) as unknown);
        return Promise.resolve(new Response(null, { status: 204 }));
      }
      const data =
        path === "/api/mappings"
          ? [
              {
                id: "m1",
                revision: "reviewed-revision",
                reviewed: true,
                confirmed: false,
                requires_consent: true,
                provenance: { source: "Synthetic", version: "1" },
                relationships: [{ from: "n1", to: "s1" }],
                episodes: {
                  n1: { title: "Native cour", season: 1, number: 2, name: "Native episode" },
                  s1: { title: "Combined show", season: 2, number: 5, name: "Show episode" },
                },
              },
            ]
          : [];
      return Promise.resolve(new Response(JSON.stringify(data), { status: 200 }));
    }),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MappingPage admin={false} t={(k) => en[k]} />
    </QueryClientProvider>,
  );
  expect(await screen.findByText(/Native cour · 1:2 · Native episode/)).toBeTruthy();
  expect(screen.getByText(/Combined show · 2:5 · Show episode/)).toBeTruthy();
  expect(writes).toHaveLength(0);
  await userEvent.click(screen.getByRole("button", { name: en.confirmMapping }));
  expect(writes).toHaveLength(0);
  confirm.mockReturnValue(true);
  await userEvent.click(screen.getByRole("button", { name: en.confirmMapping }));
  await waitFor(() => expect(writes).toEqual([{ revision: "reviewed-revision" }]));
});
