import { test, expect } from "@playwright/test";
import type { BrowserContext, Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { en, tr } from "../web/src/i18n";

const origin = "http://127.0.0.1:8080";
async function login(context: BrowserContext) {
  if (
    process.env.GITHUB_ACTIONS !== "true" ||
    process.env.WATCHLEDGER_SYNTHETIC_OIDC !== "1" ||
    !process.env.SMOKE_DIR
  )
    throw new Error("Browser fixtures require the disposable synthetic CI environment");
  // This is the ephemeral session from the CA-verified synthetic OIDC login.
  // Do not print it, save storageState, record traces, or load operator cookies.
  const raw = await readFile(join(process.env.SMOKE_DIR, "cookies"), "utf8");
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

    await page.goto(`/#detail/tv/${base}`);
    await expect(page.getByRole("heading", { name: title, exact: true })).toBeVisible();
    await expect(page.getByText("Hidden synthetic spoiler 2", { exact: true })).not.toBeVisible();
    await page.getByRole("combobox", { name: t.watchDate }).selectOption("unknown");
    const second = page.locator("article.episode").filter({ hasText: "Synthetic episode 2" });
    const watch = second.getByRole("button", { name: `＋ ${t.watched}`, exact: true });
    await watch.click();
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

    for (const route of [
      "home",
      "library",
      "lists",
      "calendar",
      "statistics",
      "settings",
      "credits",
    ]) {
      await page.goto(`/#${route}`);
      await expect(page.locator("main h1").first()).toBeVisible();
      await noOverflow(page);
      await page.screenshot({
        path: `test-results/${scenario.language}-${scenario.width}-${route}.png`,
        fullPage: true,
      });
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
