import { afterEach, expect, test, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  location.hash = "";
});
test("account identity change clears previous private cache before rendering", async () => {
  location.hash = "library";
  let account = "Alice";
  let delayOld = false;
  let releaseOld: (() => void) | undefined;
  vi.stubGlobal(
    "fetch",
    vi.fn((input: string) => {
      if (input === "/api/library" && account === "Alice" && delayOld) {
        return new Promise<Response>((resolve) => {
          releaseOld = () =>
            resolve(
              new Response(
                JSON.stringify([
                  {
                    id: "tmdb:movie:1",
                    title: "Late Alice private film",
                    kind: "movie",
                    base: "watching",
                    metadata: {},
                  },
                ]),
                { status: 200 },
              ),
            );
        });
      }
      return Promise.resolve(
        new Response(
          JSON.stringify(
            input === "/api/me"
              ? { ID: account, Name: account, Admin: false, Settings: { language: "en" } }
              : input === "/api/library"
                ? [
                    {
                      id: "tmdb:movie:1",
                      title: account + " private film",
                      kind: "movie",
                      base: "watching",
                      metadata: { en: { title: "Film" } },
                      rating: 8,
                      status_id: null,
                      status_name: null,
                      added_at: "2020-01-01",
                    },
                  ]
                : [],
          ),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      );
    }),
  );
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
  expect((await screen.findAllByText("Alice private film")).length).toBeGreaterThan(0);
  // A different tab replaces the shared session cookie; the next /me returns Bob.
  delayOld = true;
  void client.refetchQueries({ queryKey: ["library"] });
  expect(releaseOld).toBeDefined();
  account = "Bob";
  await act(async () => {
    await client.invalidateQueries({ queryKey: ["me"] });
  });
  expect(await screen.findByText("Bob")).toBeTruthy();
  expect(screen.queryByText("Alice private film")).toBeNull();
  expect((await screen.findAllByText("Bob private film")).length).toBeGreaterThan(0);
  expect(client.getQueryData(["library"])).toEqual(
    expect.arrayContaining([expect.objectContaining({ title: "Bob private film" })]),
  );
  await act(async () => {
    releaseOld?.();
    await Promise.resolve();
  });
  expect(screen.queryByText("Late Alice private film")).toBeNull();
  expect(client.getQueryData(["library"])).toEqual(
    expect.arrayContaining([expect.objectContaining({ title: "Bob private film" })]),
  );
  client.clear();
});
