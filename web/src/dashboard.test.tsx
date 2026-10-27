import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { DashboardPage } from "./dashboard";
import { displayDate } from "./dates";
import { en } from "./i18n";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
const settings = { language: "en", timezone: "Europe/Istanbul" };
const dashboard = {
  up_next: [
    {
      catalog_id: "anidb:anime:1",
      episode_id: "n1",
      title: "Native",
      season: 1,
      number: 1,
      view_anchor: "anidb:anime:1",
      incomplete: true,
    },
    { catalog_id: "tmdb:tv:2", episode_id: "t2", title: "Second", season: 2, number: 3 },
  ],
  activity: [
    {
      catalog_id: "tmdb:movie:1",
      title: "Timestamp",
      watched_date: null,
      watched_time: "2026-02-01T22:30:00Z",
    },
    {
      catalog_id: "tmdb:movie:2",
      title: "Date only",
      watched_date: "2026-02-01",
      watched_time: null,
    },
    { catalog_id: "tmdb:movie:3", title: "Unknown date", watched_date: null, watched_time: null },
  ],
  upcoming: [
    {
      catalog_id: "tmdb:tv:2",
      id: "future",
      title: "Upcoming",
      season: 2,
      number: 4,
      air_date: null,
      air_time: "2026-02-01T22:30:00Z",
    },
  ],
  statistics: { movies: 0, episodes: 0, monthly: [] },
};
function mount() {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
        })
      }
    >
      <DashboardPage
        user={{ ID: "synthetic", Name: "Fixture", Admin: false, Settings: settings }}
        t={(key) => en[key]}
      />
    </QueryClientProvider>,
  );
}
test("dashboard timestamps use selected timezone, while date-only and unknown values retain precision", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(() => Promise.resolve(new Response(JSON.stringify(dashboard), { status: 200 }))),
  );
  mount();
  expect((await screen.findByRole("link", { name: /Timestamp/ })).textContent).toContain(
    "Feb 2, 2026",
  );
  expect(screen.getByRole("link", { name: /Date only/ }).textContent).toContain("2026-02-01");
  expect(screen.getByRole("link", { name: /Unknown date/ }).textContent).toContain(en.unknown);
  expect(screen.getByRole("link", { name: /Upcoming/ }).textContent).toContain("1:30 AM");
  expect(screen.getByRole("link", { name: /Native/ }).getAttribute("href")).toBe(
    "#views/anidb%3Aanime%3A1",
  );
  expect(displayDate(null, "invalid", settings, "unknown")).toBe("unknown");
  expect(
    displayDate(
      null,
      "2026-02-01T22:30:00Z",
      { language: "tr", timezone: "Europe/Istanbul" },
      "Bilinmiyor",
    ),
  ).toContain("2 Şub 2026");
});
test("up-next visible move controls save stable displayed IDs and block concurrent ordering", async () => {
  const writes: unknown[] = [];
  let finish!: (response: Response) => void;
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      if (path === "/api/up-next/order") {
        if (typeof init?.body !== "string") throw new Error("Missing ordering body");
        writes.push(JSON.parse(init.body));
        return new Promise<Response>((resolve) => {
          finish = resolve;
        });
      }
      return Promise.resolve(new Response(JSON.stringify(dashboard), { status: 200 }));
    }),
  );
  mount();
  await userEvent.click(await screen.findByRole("button", { name: `${en.moveDown}: Native` }));
  expect(writes).toEqual([{ ids: ["tmdb:tv:2", "anidb:anime:1"] }]);
  expect(
    screen.getByRole<HTMLButtonElement>("button", { name: `${en.moveUp}: Second` }).disabled,
  ).toBe(true);
  expect(screen.getByRole<HTMLAnchorElement>("link", { name: /Native/ }).draggable).toBe(false);
  finish(new Response(null, { status: 204 }));
  await waitFor(() =>
    expect(
      screen.getByRole<HTMLButtonElement>("button", { name: `${en.moveUp}: Second` }).disabled,
    ).toBe(false),
  );
});
test("cancelled up-next drag cannot trigger a later unrelated drop", async () => {
  const writes: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      if (path === "/api/up-next/order") writes.push(path);
      return Promise.resolve(new Response(JSON.stringify(dashboard), { status: 200 }));
    }),
  );
  mount();
  const first = await screen.findByRole("link", { name: /Native/ });
  const second = screen.getByRole("link", { name: /Second/ });
  const dataTransfer = { setData: vi.fn(), effectAllowed: "" };
  fireEvent.dragStart(second, { dataTransfer });
  fireEvent.dragEnd(second, { dataTransfer });
  fireEvent.drop(first, { dataTransfer });
  expect(writes).toEqual([]);
});

test("dashboard quick watch previews then records only the selected source episode with unknown date", async () => {
  const writes: { path: string; body: unknown }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      if (path.endsWith("/track")) {
        if (typeof init?.body !== "string") throw new Error("Missing tracking body");
        const body = JSON.parse(init.body) as { preview?: boolean };
        writes.push({ path, body });
        return Promise.resolve(
          body.preview
            ? new Response(JSON.stringify({ earlier: [], offer_return_watching: false }), {
                status: 200,
              })
            : new Response(null, { status: 204 }),
        );
      }
      return Promise.resolve(new Response(JSON.stringify(dashboard), { status: 200 }));
    }),
  );
  mount();
  await screen.findByRole("button", { name: `${en.watched}: Native` });
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.watchDate }), "unknown");
  await userEvent.click(screen.getByRole("button", { name: `${en.watched}: Native` }));
  await waitFor(() => expect(writes).toHaveLength(2));
  expect(writes[0]).toEqual({
    path: "/api/library/anidb%3Aanime%3A1/track",
    body: {
      episode_id: "n1",
      watched: true,
      view_anchor: "anidb:anime:1",
      date: {},
      preview: true,
    },
  });
  expect(writes[1]?.body).toMatchObject({
    episode_id: "n1",
    date: {},
    earlier: false,
    include_linked: false,
  });
});

test("large up-next lists keep the rest of the dashboard reachable and expand on request", async () => {
  const entries = Array.from({ length: 12 }, (_, i) => ({
    catalog_id: `tmdb:tv:${i}`,
    episode_id: `episode:${i}`,
    title: `Show ${i}`,
    season: 1,
    number: 1,
  }));
  vi.stubGlobal(
    "fetch",
    vi.fn(() =>
      Promise.resolve(
        new Response(JSON.stringify({ ...dashboard, up_next: entries }), { status: 200 }),
      ),
    ),
  );
  mount();
  expect(await screen.findByRole("link", { name: /Show 9/ })).toBeTruthy();
  expect(screen.queryByRole("link", { name: /Show 10/ })).toBeNull();
  expect(screen.getByRole("heading", { name: en.recent })).toBeTruthy();
  const toggle = screen.getByRole("button", { name: /Show more/ });
  expect(toggle.getAttribute("aria-expanded")).toBe("false");
  await userEvent.click(toggle);
  expect(screen.getByRole("link", { name: /Show 10/ })).toBeTruthy();
  expect(toggle.getAttribute("aria-expanded")).toBe("true");
  await userEvent.click(toggle);
  expect(screen.queryByRole("link", { name: /Show 10/ })).toBeNull();
});
