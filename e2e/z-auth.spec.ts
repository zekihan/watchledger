import { test, expect } from "@playwright/test";
import type { Page } from "@playwright/test";
import { get } from "node:https";
import { readFile } from "node:fs/promises";
import { join } from "node:path";

const origin = "http://127.0.0.1:8080";
async function authorize(page: Page, identity: "denied" | "revoke-member" | "revoke-admin") {
  if (
    process.env.GITHUB_ACTIONS !== "true" ||
    process.env.WATCHLEDGER_SYNTHETIC_OIDC !== "1" ||
    !process.env.SMOKE_DIR
  )
    throw new Error("Synthetic CI environment required");
  // Start with the real app state/PKCE cookies. Only the fixture's TLS endpoint
  // is accessed through Node so the ephemeral CA is explicitly trusted, without
  // disabling browser certificate checks or installing a persistent root.
  const start = await page.request.get(origin + "/auth/login", { maxRedirects: 0 });
  expect(start.status()).toBe(302);
  const url = new URL(start.headers().location!);
  expect(url.origin + url.pathname).toBe("https://localhost:8443/authorize");
  url.searchParams.set("fixture_user", identity);
  const ca = await readFile(join(process.env.SMOKE_DIR, "ca.crt"));
  const callback = await new Promise<string>((resolve, reject) => {
    const request = get(url, { ca, family: 4 }, (response) => {
      response.resume();
      if (response.statusCode !== 302 || !response.headers.location) {
        reject(new Error("Synthetic authorization did not redirect"));
        return;
      }
      resolve(response.headers.location);
    });
    request.setTimeout(10000, () => request.destroy(new Error("Synthetic authorization timeout")));
    request.on("error", reject);
  });
  expect(new URL(callback).origin + new URL(callback).pathname).toBe(origin + "/auth/callback");
  return page.goto(callback);
}

test("denied admission cannot create a usable browser session", async ({ page }) => {
  const response = await authorize(page, "denied");
  expect(response?.status()).toBe(403);
  await expect(page.getByText("access not admitted", { exact: true })).toBeVisible();
  expect((await page.request.get(origin + "/api/library")).status()).toBe(401);
  await page.goto("/");
  await expect(page.getByRole("link", { name: /Sign in with your account/ })).toBeVisible();
});

test("refresh removes admission and returns the browser to sign-in", async ({ page }) => {
  await authorize(page, "revoke-member");
  expect((await page.request.get(origin + "/api/me")).ok()).toBe(true);
  await expect
    .poll(async () => (await page.request.get(origin + "/api/me")).status(), {
      timeout: 45000,
      intervals: [1000],
    })
    .toBe(401);
  await page.reload();
  await expect(page.getByRole("link", { name: /Sign in with your account/ })).toBeVisible();
  expect((await page.request.get(origin + "/api/export")).status()).toBe(401);
});

test("refresh removes admin but preserves admitted identity and private settings", async ({
  page,
}) => {
  await authorize(page, "revoke-admin");
  const before = (await (await page.request.get(origin + "/api/me")).json()) as {
    ID: string;
    Admin: boolean;
  };
  expect(before.Admin).toBe(true);
  expect((await page.request.get(origin + "/api/admin/accounts")).ok()).toBe(true);
  expect(
    (
      await page.request.put(origin + "/api/settings", {
        headers: { Origin: origin },
        data: { language: "en", country: "TR", timezone: "UTC" },
      })
    ).ok(),
  ).toBe(true);
  await expect
    .poll(
      async () => {
        const response = await page.request.get(origin + "/api/me");
        expect(response.ok()).toBe(true);
        return ((await response.json()) as { Admin: boolean }).Admin;
      },
      { timeout: 45000, intervals: [1000] },
    )
    .toBe(false);
  const after = (await (await page.request.get(origin + "/api/me")).json()) as {
    ID: string;
    Settings: { timezone: string };
  };
  expect(after.ID).toBe(before.ID);
  expect(after.Settings.timezone).toBe("UTC");
  expect((await page.request.get(origin + "/api/admin/accounts")).status()).toBe(403);
  await page.goto("/#settings");
  await expect(page.getByRole("heading", { name: "Settings", exact: true })).toBeVisible();
  expect((await page.request.get(origin + "/api/library")).ok()).toBe(true);
});
