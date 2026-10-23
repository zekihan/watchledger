import { afterEach, expect, test, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { InstallPanel } from "./install";
import { useInstall } from "./use-install";
import { en, tr } from "./i18n";
import type { Key } from "./i18n";
function Harness({
  visible = true,
  language = "en",
}: {
  visible?: boolean;
  language?: "en" | "tr";
}) {
  const state = useInstall();
  return visible ? (
    <InstallPanel state={state} t={(key: Key) => (language === "en" ? en : tr)[key]} />
  ) : null;
}
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
test("installation waits for a browser event and explicit click, retaining availability across routes", async () => {
  const view = render(<Harness visible={false} />);
  const prompt = vi.fn(() => Promise.resolve({ outcome: "dismissed" }));
  const event = Object.assign(new Event("beforeinstallprompt", { cancelable: true }), { prompt });
  act(() => {
    window.dispatchEvent(event);
  });
  expect(event.defaultPrevented).toBe(true);
  expect(prompt).not.toHaveBeenCalled();
  view.rerender(<Harness />);
  fireEvent.click(screen.getByRole("button", { name: en.installApp }));
  await waitFor(() => {
    expect(prompt).toHaveBeenCalledTimes(1);
  });
  expect(screen.queryByRole("button", { name: en.installApp })).toBeNull();
  expect(screen.queryByText(en.installedApp)).toBeNull();
  act(() => {
    window.dispatchEvent(new Event("appinstalled"));
  });
  expect(screen.getByText(en.installedApp)).toBeTruthy();
});
test("unsupported browsers get translated manual guidance and prompt failures remain visible", async () => {
  render(<Harness language="tr" />);
  expect(screen.getByText(tr.installHint)).toBeTruthy();
  expect(screen.queryByRole("button")).toBeNull();
  act(() => {
    window.dispatchEvent(
      Object.assign(new Event("beforeinstallprompt"), {
        prompt: () => Promise.reject(new Error("unavailable")),
      }),
    );
  });
  fireEvent.click(screen.getByRole("button", { name: tr.installApp }));
  expect((await screen.findByRole("alert")).textContent).toContain(tr.installFailed);
});
test("manifest has real dimension-matched PNG icons and a separate Apple touch icon", () => {
  const manifest = JSON.parse(readFileSync(resolve("public/manifest.webmanifest"), "utf8")) as {
    id: string;
    start_url: string;
    scope: string;
    display: string;
    icons: { src: string; sizes: string; type: string; purpose: string }[];
  };
  expect(manifest).toMatchObject({ id: "/", start_url: "/", scope: "/", display: "standalone" });
  for (const size of [192, 512]) {
    const icon = manifest.icons.find((icon) => icon.sizes === `${size}x${size}`);
    expect(icon?.type).toBe("image/png");
    expect(icon?.purpose).toContain("maskable");
    const png = readFileSync(resolve(`public${icon?.src}`));
    expect(png.subarray(0, 8).toString("hex")).toBe("89504e470d0a1a0a");
    expect(png.readUInt32BE(16)).toBe(size);
    expect(png.readUInt32BE(20)).toBe(size);
  }
  const apple = readFileSync(resolve("public/apple-touch-icon.png"));
  expect(apple.readUInt32BE(16)).toBe(180);
  expect(apple.readUInt32BE(20)).toBe(180);
  expect(readFileSync(resolve("index.html"), "utf8")).toContain('rel="apple-touch-icon"');
});
