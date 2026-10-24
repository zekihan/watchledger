import { defineConfig } from "@playwright/test";

const browser = process.env.WATCHLEDGER_TEST_BROWSER ?? "chromium";
if (!["chromium", "firefox", "webkit"].includes(browser))
  throw new Error("Unknown synthetic test browser");

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 150000,
  globalTimeout: 12 * 60 * 1000,
  reporter: "list",
  use: {
    baseURL: "http://127.0.0.1:8080",
    browserName: browser as "chromium" | "firefox" | "webkit",
    trace: "off",
    video: "off",
    screenshot: "only-on-failure",
  },
});
