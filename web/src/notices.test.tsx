import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Notices } from "./admin";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
test("notices require explicit acknowledgement and a failed write leaves the notice visible", async () => {
  const writes: string[] = [];
  let read = false;
  let fail = true;
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      if (init?.method === "POST") {
        writes.push(path);
        if (fail) return Promise.resolve(new Response("Temporary failure", { status: 503 }));
        read = true;
        return Promise.resolve(new Response(null, { status: 204 }));
      }
      return Promise.resolve(
        new Response(
          JSON.stringify(
            read
              ? []
              : [{ id: "notice-1", kind: "mapping_changed", data: { mapping_id: "map-1" } }],
          ),
        ),
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
      <Notices t={(key) => en[key]} />
    </QueryClientProvider>,
  );
  expect(await screen.findByText(en.mappingChanged, { exact: false })).toBeTruthy();
  expect(writes).toEqual([]);
  await userEvent.click(screen.getByRole("button", { name: en.markRead }));
  expect(await screen.findByRole("alert")).toBeTruthy();
  expect(screen.getByRole("button", { name: en.markRead })).toBeTruthy();
  fail = false;
  await userEvent.click(screen.getByRole("button", { name: en.markRead }));
  await waitFor(() => {
    expect(screen.queryByRole("button", { name: en.markRead })).toBeNull();
  });
  expect(writes).toEqual(["/api/notices/notice-1/read", "/api/notices/notice-1/read"]);
});
