import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import type { BrowserContext, Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { en, tr } from "../web/src/i18n";

const origin = "http://127.0.0.1:8080";
async function login(context: BrowserContext, file: "cookies" | "cookies-second" = "cookies") {
  if (
    process.env.GITHUB_ACTIONS !== "true" ||
    process.env.WATCHLEDGER_SYNTHETIC_OIDC !== "1" ||
    !process.env.SMOKE_DIR
  )
    throw new Error("Browser fixtures require the disposable synthetic CI environment");
  // This is the ephemeral session from the CA-verified synthetic OIDC login.
  // Do not print it, save storageState, record traces, or load operator cookies.
  const raw = await readFile(join(process.env.SMOKE_DIR, file), "utf8");
  const row = raw
    .split("\n")
    .map((line) => line.replace(/^#HttpOnly_/, "").split("\t"))
    .find((fields) => fields[0] === "127.0.0.1" && fields[5] === "wl_session");
  if (!row?.[6]) throw new Error("Missing synthetic session");
  await context.addCookies([
    { name: "wl_session", value: row[6], url: origin, httpOnly: true, sameSite: "Lax" },
  ]);
}
async function noOverflow(page: Page) {
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= document.documentElement.clientWidth + 1,
    ),
  ).toBe(true);
}

async function accessible(page: Page) {
  const result = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21aa"])
    .analyze();
  expect
    .soft(
      result.violations.map((violation) => ({
        id: violation.id,
        impact: violation.impact,
        nodes: violation.nodes.map((node) => ({
          target: node.target,
          reason: node.failureSummary,
        })),
      })),
    )
    .toEqual([]);
}

const cases = [
  { language: "en", width: 1440, height: 1000 },
  { language: "tr", width: 1440, height: 1000 },
  { language: "en", width: 390, height: 844 },
  { language: "tr", width: 360, height: 800 },
] as const;
for (const [index, scenario] of cases.entries()) {
  test(`${scenario.language} ${scenario.width}px private restore, tracking, lists and navigation`, async ({
    page,
    context,
  }) => {
    const t = scenario.language === "tr" ? tr : en;
    await page.setViewportSize({ width: scenario.width, height: scenario.height });
    await page.goto("/");
    await expect(page.getByRole("link", { name: /Sign in with your account/ })).toHaveAttribute(
      "href",
      "/auth/login",
    );
    await login(context);
    // A cookie handoff must discard the already-rendered anonymous query cache.
    await page.goto("about:blank");
    const settings = {
      language: scenario.language,
      metadata_language: "en",
      country: "TR",
      timezone: "Europe/Istanbul",
      anime_view: "native",
      anime_title: "romaji",
      spoilers: false,
      services: [],
    };
    expect(
      (
        await context.request.put(`${origin}/api/settings`, {
          headers: { Origin: origin },
          data: settings,
        })
      ).ok(),
    ).toBe(true);
    const exported = await context.request.get(`${origin}/api/export`);
    expect(exported.ok()).toBe(true);
    const snapshot: {
      version: number;
      created_at: string;
      catalog: unknown[];
      episodes: unknown[];
      library: unknown[];
      history: unknown[];
      settings: unknown;
      statuses: unknown[];
      lists: unknown[];
      tombstones: unknown[];
    } = {
      version: 1,
      created_at: new Date().toISOString(),
      catalog: [],
      episodes: [],
      library: [],
      history: [],
      settings,
      statuses: [],
      lists: [],
      tombstones: [],
    };
    const base = 910000 + index * 10;
    const catalogID = `tmdb:tv:${base}`;
    const title = `Synthetic browser series ${index}`;
    const stamp = "2026-01-01T00:00:00Z";
    snapshot.catalog = [
      {
        id: catalogID,
        source: "tmdb",
        source_id: String(base),
        kind: "tv",
        title,
        metadata: {
          en: {
            id: base,
            name: title,
            status: "Ended",
            number_of_episodes: 2,
            first_air_date: "2020-01-01",
            seasons: [{ season_number: 1, episode_count: 2 }],
          },
        },
        fetched_at: new Date().toISOString(),
      },
    ];
    snapshot.episodes = [1, 2].map((number) => ({
      id: `tmdb:episode:${base + number}`,
      catalog_id: catalogID,
      source: "tmdb",
      source_id: String(base + number),
      season: 1,
      number,
      kind: "regular",
      retired: false,
      air_date: "2020-01-01",
      air_time: null,
      metadata: {
        id: base + number,
        name: `Synthetic episode ${number}`,
        season_number: 1,
        episode_number: number,
        air_date: "2020-01-01",
        overview: `Hidden synthetic spoiler ${number}`,
        runtime: 24,
      },
    }));
    const future = new Date(Date.now() + 86400000).toISOString();
    snapshot.episodes.push(
      ...[3, 4].map((number) => ({
        id: `tmdb:episode:${base + number}`,
        catalog_id: catalogID,
        source: "tmdb",
        source_id: String(base + number),
        season: 0,
        number: number - 2,
        kind: "special",
        retired: false,
        air_date: number === 4 ? `${new Date().toISOString().slice(0, 7)}-01` : null,
        air_time: number === 3 ? future : null,
        metadata: {
          id: base + number,
          name: `Synthetic special ${number}`,
          season_number: 0,
          episode_number: number - 2,
          overview: "Synthetic special summary",
          runtime: 10,
        },
      })),
    );
    snapshot.library = [
      {
        catalog_id: catalogID,
        base: "watching",
        rating: 8,
        status_id: null,
        position: 0,
        anime_view: null,
        added_at: stamp,
        updated_at: stamp,
      },
    ];
    const nativeID = `anidb:anime:${base + 5}`;
    const nativeTitle = `Synthetic native entry ${index}`;
    snapshot.catalog.push({
      id: nativeID,
      source: "anidb",
      source_id: String(base + 5),
      kind: "anime",
      title: nativeTitle,
      metadata: {
        en: {
          id: base + 5,
          name: nativeTitle,
          status: "Ended",
          number_of_episodes: 1,
          seasons: [{ season_number: 1, name: "Regular", episode_count: 1 }],
          titles: [{ language: "x-jat", type: "main", text: nativeTitle }],
        },
      },
      fetched_at: new Date().toISOString(),
    });
    snapshot.episodes.push({
      id: `anidb:episode:${base + 6}`,
      catalog_id: nativeID,
      source: "anidb",
      source_id: String(base + 6),
      season: 1,
      number: 1,
      kind: "regular",
      retired: false,
      air_date: "2020-01-01",
      air_time: null,
      metadata: {
        id: base + 6,
        name: "Native restored episode",
        season_number: 1,
        episode_number: 1,
        native_number: "1",
        air_date: "2020-01-01",
        runtime: 24,
      },
    });
    snapshot.library.push({
      catalog_id: nativeID,
      base: "watching",
      rating: 9,
      status_id: null,
      position: 1,
      anime_view: null,
      added_at: stamp,
      updated_at: stamp,
    });
    snapshot.history = [];
    snapshot.settings = settings;

    await page.goto("/#settings");
    await expect(page.getByRole("heading", { name: t.settings, exact: true })).toBeVisible();
    await page.getByRole("checkbox", { name: t.backupSaved }).check();
    await page.getByLabel(t.restore, { exact: true }).setInputFiles({
      name: "synthetic-restore.json",
      mimeType: "application/json",
      buffer: Buffer.from(JSON.stringify(snapshot)),
    });
    await expect(page.getByText(t.previewReady, { exact: true })).toBeVisible();
    const before = await context.request.get(`${origin}/api/library`);
    expect(((await before.json()) as { id: string }[]).some((item) => item.id === catalogID)).toBe(
      false,
    );
    await page.getByRole("button", { name: t.apply, exact: true }).click();
    await expect(page.getByText(t.restoreDone, { exact: true })).toBeVisible();

    await page.goto(`/#detail/anime/${base + 5}`);
    await expect(page.getByRole("heading", { name: nativeTitle, exact: true })).toBeVisible();
    await page.getByRole("combobox", { name: t.watchDate }).selectOption("unknown");
    const nativeEpisode = page
      .locator("article.episode")
      .filter({ hasText: "Native restored episode" });
    await nativeEpisode.getByRole("button", { name: `＋ ${t.watched}`, exact: true }).click();
    await expect(
      nativeEpisode.getByRole("button", { name: `✓ ${t.unwatched}`, exact: true }),
    ).toBeVisible();
    const nativeHistory = (await (
      await context.request.get(`${origin}/api/library/${nativeID}/history`)
    ).json()) as { episode_id: string; watched_date: null; watched_time: null }[];
    expect(nativeHistory).toHaveLength(1);
    expect(nativeHistory[0]).toMatchObject({
      episode_id: `anidb:episode:${base + 6}`,
      watched_date: null,
      watched_time: null,
    });

    await page.goto(`/#detail/tv/${base}`);
    await expect(page.getByRole("heading", { name: title, exact: true })).toBeVisible();
    await expect(page.getByText("Hidden synthetic spoiler 2", { exact: true })).not.toBeVisible();
    await page.getByRole("combobox", { name: t.watchDate }).selectOption("unknown");
    const second = page.locator("article.episode").filter({ hasText: "Synthetic episode 2" });
    const watch = second.getByRole("button", { name: `＋ ${t.watched}`, exact: true });
    await expect(watch).toBeEnabled();
    await watch.focus();
    await expect(watch).toBeFocused();
    await page.keyboard.press("w");
    const dialog = page.getByRole("dialog", { name: t.earlierPrompt });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("radio", { name: t.onlyThis, exact: true })).toBeChecked();
    await page.keyboard.press("Escape");
    await expect(dialog).not.toBeVisible();
    await expect(watch).toBeFocused();
    await watch.click();
    await dialog.getByRole("button", { name: t.save, exact: true }).click();
    await expect(
      second.getByRole("button", { name: `✓ ${t.unwatched}`, exact: true }),
    ).toBeVisible();
    const history = (await (
      await context.request.get(`${origin}/api/library/${catalogID}/history`)
    ).json()) as { episode_id: string; watched_date: null; watched_time: null }[];
    expect(history).toHaveLength(1);
    expect(history[0]).toMatchObject({
      episode_id: `tmdb:episode:${base + 2}`,
      watched_date: null,
      watched_time: null,
    });
    await noOverflow(page);

    await page.goto("/#lists");
    await page.getByRole("button", { name: `＋ ${t.newList}`, exact: true }).click();
    const listName = `Browser list ${index}`;
    await page.getByRole("textbox", { name: t.name, exact: true }).fill(listName);
    await page.getByRole("button", { name: `＋ ${title}`, exact: true }).click();
    await page.getByRole("button", { name: t.save, exact: true }).click();
    const card = page.locator("article.list-card").filter({ hasText: listName });
    await expect(card).toBeVisible();
    await card.getByRole("button", { name: t.viewList, exact: true }).click();
    await expect(page.getByRole("link", { name: title, exact: true })).toBeVisible();

    await page.goto("/#lists");
    await page.getByRole("button", { name: `＋ ${t.newList}`, exact: true }).click();
    await page.getByRole("textbox", { name: t.name, exact: true }).fill(`Browser smart ${index}`);
    await page.getByRole("combobox", { name: t.kind, exact: true }).selectOption("smart");
    await expect(page.getByRole("link", { name: title, exact: true })).toBeVisible();
    await page.getByRole("button", { name: t.save, exact: true }).click();
    await expect(
      page.getByRole("heading", { name: `Browser smart ${index}`, exact: true }),
    ).toBeVisible();

    const headings = {
      home: t.home,
      library: t.library,
      lists: t.lists,
      calendar: t.calendar,
      statistics: t.statistics,
      settings: t.settings,
      credits: t.sourceCredits,
    };
    for (const route of [
      "home",
      "library",
      "lists",
      "calendar",
      "statistics",
      "settings",
      "credits",
    ] as const) {
      await page.goto(`/#${route}`);
      await expect(page.getByRole("heading", { name: headings[route], exact: true })).toBeVisible();
      if (route === "calendar") {
        const view = page.getByRole("combobox", { name: t.calendar, exact: true });
        await expect(view).toHaveValue(scenario.width < 680 ? "list" : "month");
        await view.selectOption("month");
        await expect(page.getByRole("region", { name: new RegExp(t.calendar) })).toBeVisible();
        await noOverflow(page);
        await view.selectOption(scenario.width < 680 ? "list" : "month");
      }
      await noOverflow(page);
      await page.screenshot({
        path: `test-results/${scenario.language}-${scenario.width}-${route}.png`,
        fullPage: true,
      });
      await accessible(page);
    }
    await page.goto("/#library");
    await page.keyboard.press("/");
    await expect(page.locator("header input")).toBeFocused();
    await page.goto("/#settings");
    const downloadPromise = page.waitForEvent("download");
    await page.getByRole("link", { name: `↓ ${t.export}`, exact: true }).click();
    const download = await downloadPromise;
    const file = await download.path();
    if (!file) throw new Error("Missing portable download");
    const backup = JSON.parse(await readFile(file, "utf8")) as Record<string, unknown>;
    expect(backup).not.toHaveProperty("sessions");
    expect(backup).not.toHaveProperty("roles");
    expect(backup).not.toHaveProperty("admin");
  });
}

test("second synthetic identity cannot browse the first user's library, lists, history or imports", async ({
  page,
  context,
  browser,
}) => {
  await login(context, "cookies-second");
  await page.goto("/#library");
  await expect(page.getByText(en.empty, { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: /Synthetic browser series/ })).toHaveCount(0);
  for (const path of [
    "/api/library",
    "/api/lists",
    "/api/imports",
    "/api/import-reviews",
    "/api/library/tmdb:tv:910000/history",
  ]) {
    const response = await context.request.get(origin + path);
    expect(response.ok()).toBe(true);
    expect(await response.json()).toEqual([]);
  }
  const first = await browser.newContext();
  try {
    await login(first);
    const lists = (await (await first.request.get(origin + "/api/lists")).json()) as {
      id: string;
    }[];
    expect(lists.length).toBeGreaterThan(0);
    expect(
      (await context.request.get(`${origin}/api/lists/${lists[0]!.id}/results`)).status(),
    ).toBe(404);
    const imports = (await (await first.request.get(origin + "/api/imports")).json()) as {
      id: string;
    }[];
    expect(imports.length).toBeGreaterThan(0);
    expect(
      (await context.request.get(`${origin}/api/imports/${imports[0]!.id}/archive`)).status(),
    ).toBe(404);

    // Copy only synthetic portable data through the actual UI. Identity and
    // authorization remain bound to the second independently authenticated user.
    const backup = (await (await first.request.get(origin + "/api/export")).json()) as {
      settings: Record<string, unknown>;
      library: { catalog_id: string }[];
      history: {
        catalog_id: string;
        episode_id: string | null;
        watched_date: string | null;
        watched_time: string | null;
      }[];
      lists: { name: string }[];
    };
    backup.settings.language = "en";
    const firstHistory = await (
      await first.request.get(origin + "/api/library/tmdb:tv:910000/history")
    ).json();
    async function restore() {
      await page.goto("/#settings");
      await page.getByRole("checkbox", { name: en.backupSaved }).check();
      await page.getByLabel(en.restore, { exact: true }).setInputFiles({
        name: "synthetic-cross-account.json",
        mimeType: "application/json",
        buffer: Buffer.from(JSON.stringify(backup)),
      });
      await expect(page.getByText(en.previewReady, { exact: true })).toBeVisible();
      await page.getByRole("button", { name: en.apply, exact: true }).click();
      await expect(page.getByText(en.restoreDone, { exact: true })).toBeVisible();
    }
    await restore();
    const restored = (await (
      await context.request.get(origin + "/api/export")
    ).json()) as typeof backup;
    expect(restored.library).toHaveLength(backup.library.length);
    expect(restored.lists.map((list) => list.name).sort()).toEqual(
      backup.lists.map((list) => list.name).sort(),
    );
    const facts = (snapshot: typeof backup) =>
      snapshot.history
        .map((event) =>
          [event.catalog_id, event.episode_id, event.watched_date, event.watched_time].join("|"),
        )
        .sort();
    expect(facts(restored)).toEqual(facts(backup));
    expect(
      (
        await context.request.put(origin + "/api/library/tmdb:tv:910000", {
          headers: { Origin: origin },
          data: { base: "hold", rating: 3 },
        })
      ).ok(),
    ).toBe(true);
    await page.goto("/#library");
    await expect(
      page.getByRole("link", { name: /Synthetic browser series 0/ }).first(),
    ).toBeVisible();
    await restore();
    const afterRepeat = (await (await context.request.get(origin + "/api/library")).json()) as {
      id: string;
      rating: number;
      base: string;
    }[];
    expect(afterRepeat.find((item) => item.id === "tmdb:tv:910000")).toMatchObject({
      rating: 3,
      base: "hold",
    });
    const repeated = (await (
      await context.request.get(origin + "/api/export")
    ).json()) as typeof backup;
    expect(facts(repeated)).toEqual(facts(restored));
    expect(
      await (await first.request.get(origin + "/api/library/tmdb:tv:910000/history")).json(),
    ).toEqual(firstHistory);
    const firstLibrary = (await (await first.request.get(origin + "/api/library")).json()) as {
      id: string;
      rating: number;
    }[];
    expect(firstLibrary.find((item) => item.id === "tmdb:tv:910000")?.rating).toBe(8);
  } finally {
    await first.close();
  }
  expect((await context.request.get(origin + "/api/admin/accounts")).status()).toBe(403);
});

// Catalog presentation is tested with explicitly synthetic provider responses.
// Private session/library/permissions still use the real application APIs.
for (const scenario of cases) {
  test(`${scenario.language} ${scenario.width}px catalog presentation and discovery controls`, async ({
    page,
    context,
  }) => {
    const t = scenario.language === "tr" ? tr : en;
    await page.setViewportSize({ width: scenario.width, height: scenario.height });
    await login(context);
    expect(
      (
        await context.request.put(origin + "/api/settings", {
          headers: { Origin: origin },
          data: {
            language: scenario.language,
            metadata_language: "en",
            country: "TR",
            timezone: "Europe/Istanbul",
            anime_view: "native",
            anime_title: "romaji",
            spoilers: false,
            services: [8],
          },
        })
      ).ok(),
    ).toBe(true);
    const paths: string[] = [];
    const titles = [
      {
        id: 990001,
        title: "Synthetic catalog film",
        media_type: "movie",
        poster_path: "/synthetic.png",
        release_date: "2026-01-01",
        vote_average: 8,
      },
      {
        id: 990002,
        title: "Synthetic related film",
        media_type: "movie",
        poster_path: "/synthetic.png",
        release_date: "2025-01-01",
      },
    ];
    await page.route("https://image.tmdb.org/**/synthetic.png", (route) =>
      route.fulfill({
        contentType: "image/svg+xml",
        body: '<svg xmlns="http://www.w3.org/2000/svg" width="342" height="513"><rect width="342" height="513" fill="#294c45"/><circle cx="171" cy="180" r="85" fill="#b9eb91"/><text x="171" y="370" text-anchor="middle" fill="white" font-size="25">SYNTHETIC</text></svg>',
      }),
    );
    await page.route("**/api/search?*", (route) =>
      route.fulfill({ json: { results: titles, total_pages: 1 } }),
    );
    await page.route("**/api/discover/movie?*", (route) => {
      paths.push(route.request().url());
      return route.fulfill({ json: { results: titles, total_pages: 2 } });
    });
    await page.route("**/api/catalog/movie/990001?*", (route) =>
      route.fulfill({
        json: {
          metadata: {
            ...titles[0],
            overview: "Synthetic movie synopsis",
            runtime: 100,
            genres: [{ id: 18, name: "Synthetic genre" }],
            credits: {
              cast: [
                {
                  id: 2,
                  name: "Synthetic actor",
                  character: "Synthetic role",
                  profile_path: "/synthetic.png",
                },
              ],
              crew: [{ id: 2, name: "Synthetic actor", job: "Director" }],
            },
            videos: {
              results: [{ site: "YouTube", key: "synthetic-video", name: "Synthetic trailer" }],
            },
            recommendations: { results: [titles[1]] },
          },
          fetched_at: "2026-01-01T22:30:00Z",
          stale: false,
        },
      }),
    );
    await page.route("**/api/catalog/movie/990001/providers?*", (route) =>
      route.fulfill({
        json: {
          offers: {
            link: "https://example.test/synthetic-offers",
            flatrate: [{ provider_id: 8, provider_name: "Synthetic subscription" }],
            rent: [{ provider_id: 9, provider_name: "Synthetic rental" }],
            buy: [{ provider_id: 10, provider_name: "Synthetic purchase" }],
          },
          fetched_at: "2026-01-01T22:30:00Z",
          stale: false,
        },
      }),
    );
    await page.route("**/api/catalog/person/2?*", (route) =>
      route.fulfill({
        json: {
          metadata: {
            id: 2,
            name: "Synthetic actor",
            biography: "Synthetic person biography",
            profile_path: "/synthetic.png",
            combined_credits: { cast: titles, crew: [titles[0]] },
          },
          fetched_at: "2026-01-01T22:30:00Z",
          stale: false,
        },
      }),
    );
    await page.goto("/#search/Synthetic");
    const cards = page.locator("a.poster-card");
    await expect(cards).toHaveCount(2);
    await cards.first().focus();
    await page.keyboard.press("ArrowRight");
    await expect(cards.nth(1)).toBeFocused();
    await cards.first().click();
    await expect(
      page.getByRole("heading", { name: "Synthetic catalog film", exact: true }),
    ).toBeVisible();
    await expect(page.getByText("Synthetic movie synopsis", { exact: true })).toBeVisible();
    for (const name of ["Synthetic subscription", "Synthetic rental", "Synthetic purchase"])
      await expect(page.getByRole("link", { name: `${name} ↗`, exact: true })).toHaveAttribute(
        "href",
        "https://example.test/synthetic-offers",
      );
    await expect(
      page.getByRole("link", { name: "▷ Synthetic trailer", exact: true }),
    ).toHaveAttribute("href", "https://www.youtube.com/watch?v=synthetic-video");
    await expect(page.locator("iframe,video")).toHaveCount(0);
    await expect(page.getByRole("link", { name: /Synthetic related film/ })).toBeVisible();
    await noOverflow(page);
    await page.screenshot({
      path: `test-results/${scenario.language}-${scenario.width}-catalog.png`,
      fullPage: true,
    });
    await accessible(page);
    await page.locator(".people a").first().click();
    await expect(page.getByText("Synthetic person biography", { exact: true })).toBeVisible();
    await page.getByRole("combobox", { name: t.credits, exact: true }).selectOption("crew");
    await expect(page.locator("a.poster-card")).toHaveCount(1);
    await page.getByRole("combobox", { name: t.kind, exact: true }).selectOption("tv");
    await expect(page.getByText(t.empty, { exact: true })).toBeVisible();
    await page.goto("/#discover");
    await page.getByText(t.filters, { exact: true }).click();
    await page.getByLabel(t.minRuntime, { exact: true }).fill("80");
    await page.getByLabel(t.castIDs, { exact: true }).fill("2");
    await page.getByLabel(t.crewIDs, { exact: true }).fill("3");
    await page.getByRole("button", { name: t.myServices, exact: true }).click();
    await expect
      .poll(() =>
        paths.some((path) => {
          const q = new URL(path).searchParams;
          return (
            q.get("with_runtime.gte") === "80" &&
            q.get("with_cast") === "2" &&
            q.get("with_crew") === "3" &&
            q.get("with_watch_providers") === "8" &&
            q.get("watch_region") === "TR"
          );
        }),
      )
      .toBe(true);
    await noOverflow(page);
    await page.screenshot({
      path: `test-results/${scenario.language}-${scenario.width}-discovery.png`,
      fullPage: true,
    });
  });
}
