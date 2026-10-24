import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import { en } from "./i18n";
import { episodeLink } from "./episode-links";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  location.hash = "";
});
function mount(episode = "702") {
  location.hash = `detail/tv/70/season/2/episode/${episode}`;
  const writes: Record<string, unknown>[] = [];
  const paths: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      paths.push(path);
      let data: unknown = [];
      if (path === "/api/me")
        data = {
          ID: "fixture",
          Name: "Fixture",
          Admin: false,
          Settings: { language: "en", timezone: "UTC", spoilers: false },
        };
      else if (path === "/api/library")
        data = [
          {
            id: "tmdb:tv:70",
            kind: "tv",
            title: "Fixture show",
            base: "watching",
            position: 0,
            metadata: {},
          },
        ];
      else if (path.startsWith("/api/catalog/tv/70/seasons/2"))
        data = {
          name: "Second season",
          overview: "Season synopsis",
          episodes: [
            { id: 701, name: "First episode", season_number: 2, episode_number: 1 },
            {
              id: 702,
              name: "Chosen episode",
              season_number: 2,
              episode_number: 2,
              overview: "Hidden plot",
              still_path: "/synthetic.jpg",
              runtime: 42,
              air_date: "2026-02-03",
            },
          ],
        };
      else if (path.startsWith("/api/catalog/tv/70?"))
        data = {
          metadata: {
            id: 70,
            name: "Fixture show",
            seasons: [
              { season_number: 1, name: "First season", episode_count: 1 },
              { season_number: 2, name: "Second season", episode_count: 2 },
            ],
          },
          fetched_at: "2026-01-01T00:00:00Z",
        };
      else if (path.endsWith("/alternate"))
        data = { structure: "show", incomplete: false, titles: [] };
      else if (path.endsWith("/track")) {
        writes.push(
          JSON.parse(typeof init?.body === "string" ? init.body : "{}") as Record<string, unknown>,
        );
        data = { earlier: [], offer_return_watching: false };
      }
      return Promise.resolve(new Response(JSON.stringify(data), { status: 200 }));
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
      <App />
    </QueryClientProvider>,
  );
  return { writes, paths };
}
test("episode deep link loads the actual source identity and reveals no synopsis or still before consent", async () => {
  const { writes, paths } = mount();
  expect(await screen.findByRole("heading", { name: "Chosen episode" })).toBeTruthy();
  expect(screen.queryByRole("heading", { name: "First episode" })).toBeNull();
  expect(screen.queryByText("Hidden plot")).toBeNull();
  expect(screen.queryByRole("img", { name: "Chosen episode" })).toBeNull();
  expect(screen.getByText("2026-02-03")).toBeTruthy();
  expect(screen.getByText(/42 minutes/)).toBeTruthy();
  expect(screen.getByText(/tmdb:episode:702/)).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: en.reveal }));
  expect(screen.getByText("Hidden plot")).toBeTruthy();
  const image = screen.getByRole("img", { name: "Chosen episode" });
  expect(image.getAttribute("src")).toBe("https://image.tmdb.org/t/p/w780/synthetic.jpg");
  fireEvent.error(image);
  expect(screen.queryByRole("img", { name: "Chosen episode" })).toBeNull();
  const watchButton = screen.getByRole("button", { name: `＋ ${en.watched}` });
  const description = watchButton.getAttribute("aria-describedby");
  expect(description && document.getElementById(description)?.textContent).toBe("Chosen episode");
  await userEvent.click(watchButton);
  await waitFor(() => {
    expect(writes.some((write) => !write.preview && write.episode_id === "tmdb:episode:702")).toBe(
      true,
    );
  });
  expect(writes.every((write) => write.episode_id === "tmdb:episode:702")).toBe(true);
  expect(paths.some((path) => path.includes("/seasons/1"))).toBe(false);
  expect(screen.getByRole("link", { name: en.allEpisodes }).getAttribute("href")).toBe(
    "#detail/tv/70/season/2",
  );
});
test("missing source episode is explicit, never replaced by another episode with the same number", async () => {
  const { writes } = mount("999");
  expect(await screen.findByText(en.missingEpisode)).toBeTruthy();
  expect(screen.queryByRole("heading", { name: "Chosen episode" })).toBeNull();
  expect(screen.queryByRole("button", { name: `＋ ${en.watched}` })).toBeNull();
  expect(writes).toEqual([]);
});
test("episode links preserve specials and native identity instead of treating numbering as identity", () => {
  expect(episodeLink("anidb:anime:42", 0, "anidb:episode:501")).toBe(
    "#detail/anime/42/season/0/episode/501",
  );
  expect(episodeLink("tmdb:tv:42", 3, "tmdb:episode:801")).toBe(
    "#detail/tv/42/season/3/episode/801",
  );
  expect(episodeLink("anidb:anime:42", 0, "tmdb:episode:501")).toBe("#detail/anime/42");
});

test("clearing a chosen watch date cannot silently record an unknown date", async () => {
  const { writes } = mount();
  await screen.findByRole("heading", { name: "Chosen episode" });
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.watchDate }), "chosen");
  const date = screen.getByLabelText(en.watchDate, { selector: "input" });
  fireEvent.change(date, { target: { value: "" } });
  await userEvent.click(screen.getByRole("button", { name: `＋ ${en.watched}` }));
  expect(await screen.findByText(en.chooseWatchDate)).toBeTruthy();
  expect(writes).toHaveLength(0);
});
