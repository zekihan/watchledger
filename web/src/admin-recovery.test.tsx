import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MappingPage } from "./admin";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
function mount(admin: boolean) {
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <MappingPage admin={admin} t={(key) => en[key]} />
    </QueryClientProvider>,
  );
}
test("admin mappings, account oversight and background failures can recover independently", async () => {
  const attempts = new Map<string, number>();
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      const count = (attempts.get(path) ?? 0) + 1;
      attempts.set(path, count);
      if (["/api/mappings", "/api/admin/accounts", "/api/admin/jobs"].includes(path) && count === 1)
        return Promise.resolve(new Response(`Unavailable ${path}`, { status: 503 }));
      const data =
        path === "/api/admin/accounts"
          ? [{ id: "synthetic", name: "Recovered account", admin: false }]
          : path === "/api/admin/jobs"
            ? [
                {
                  id: "synthetic-job",
                  attempts: 2,
                  last_error: "Synthetic provider failure",
                  run_at: "2026-01-01T12:00:00Z",
                },
              ]
            : path === "/api/admin/anidb/titles"
              ? { entries: 0 }
              : [];
      return Promise.resolve(new Response(JSON.stringify(data), { status: 200 }));
    }),
  );
  mount(true);
  await waitFor(() => expect(screen.getAllByRole("alert")).toHaveLength(3));
  for (const path of ["/api/mappings", "/api/admin/accounts", "/api/admin/jobs"]) {
    const alert = screen.getAllByRole("alert").find((node) => node.textContent?.includes(path));
    if (!alert) throw new Error(`Missing error ${path}`);
    await userEvent.click(within(alert).getByRole("button", { name: en.retry }));
  }
  expect(await screen.findByText("Recovered account")).toBeTruthy();
  expect(await screen.findByText("Synthetic provider failure")).toBeTruthy();
  await waitFor(() => expect(screen.queryAllByRole("alert")).toHaveLength(0));
  expect(attempts.get("/api/admin/accounts")).toBe(2);
  expect(attempts.get("/api/admin/jobs")).toBe(2);
});
test("non-admin mapping browsing never requests account or background-job oversight", async () => {
  const paths: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      paths.push(path);
      return Promise.resolve(new Response("[]", { status: 200 }));
    }),
  );
  mount(false);
  await waitFor(() => expect(paths).toContain("/api/mappings"));
  expect(paths.some((path) => path.startsWith("/api/admin/"))).toBe(false);
  expect(screen.queryByRole("heading", { name: en.accounts })).toBeNull();
});
