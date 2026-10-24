import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useAlternateTracking } from "./alternate-tracking";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
function Harness({ watched = false }: { watched?: boolean }) {
  const track = useAlternateTracking((k) => en[k], "UTC");
  return (
    <>
      {track.controls}
      <button
        disabled={track.pending}
        onClick={() =>
          track.run({
            catalog_id: "tmdb:tv:1",
            episode_id: "tmdb:episode:2",
            view_anchor: "anidb:anime:1",
            watched,
          })
        }
      >
        {watched ? "Watch alternate" : "Unwatch alternate"}
      </button>
    </>
  );
}
test("alternate unwatch requires linked confirmation and keeps native anchor in the request", async () => {
  HTMLDialogElement.prototype.showModal = function () {
    this.open = true;
  };
  const writes: Record<string, unknown>[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((_path: string, init?: RequestInit) => {
      if (typeof init?.body !== "string") throw new Error("Missing body");
      const body = JSON.parse(init.body) as Record<string, unknown>;
      writes.push(body);
      return Promise.resolve(
        new Response(
          JSON.stringify(
            body.preview
              ? {
                  earlier: [],
                  offer_return_watching: false,
                  linked_plan: "snapshot",
                  linked_changes: [{ title: "Native cour", episode_id: "n1" }],
                }
              : {},
          ),
          { status: 200 },
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
      <Harness />
    </QueryClientProvider>,
  );
  await userEvent.click(screen.getByRole("button", { name: "Unwatch alternate" }));
  await screen.findByRole("dialog");
  expect(screen.getByRole<HTMLInputElement>("radio", { name: en.cancel }).checked).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: en.cancel }));
  expect(writes).toHaveLength(1);
  await waitFor(() =>
    expect(
      screen.getByRole<HTMLButtonElement>("button", { name: "Unwatch alternate" }).disabled,
    ).toBe(false),
  );
  await userEvent.click(screen.getByRole("button", { name: "Unwatch alternate" }));
  await screen.findByRole("dialog");
  await userEvent.click(screen.getByRole("radio", { name: en.unwatchLinked }));
  await userEvent.click(screen.getByRole("button", { name: en.save }));
  await waitFor(() => expect(writes).toHaveLength(3));
  expect(writes[2]).toMatchObject({
    view_anchor: "anidb:anime:1",
    episode_id: "tmdb:episode:2",
    watched: false,
    include_linked: true,
    linked_plan: "snapshot",
  });
});

test("alternate quick tracking requires a chosen date unless unknown is explicitly selected", async () => {
  const writes: Record<string, unknown>[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((_path: string, init?: RequestInit) => {
      if (typeof init?.body !== "string") throw new Error("Missing body");
      writes.push(JSON.parse(init.body) as Record<string, unknown>);
      return Promise.resolve(
        new Response(JSON.stringify({ earlier: [], offer_return_watching: false }), {
          status: 200,
        }),
      );
    }),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { mutations: { retry: false } } })}
    >
      <Harness watched />
    </QueryClientProvider>,
  );
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.watchDate }), "chosen");
  fireEvent.change(screen.getByLabelText(en.watchDate, { selector: "input" }), {
    target: { value: "" },
  });
  await userEvent.click(screen.getByRole("button", { name: "Watch alternate" }));
  expect(await screen.findByRole("alert")).toHaveProperty("textContent", en.chooseWatchDate);
  expect(writes).toHaveLength(0);
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.watchDate }), "unknown");
  await userEvent.click(screen.getByRole("button", { name: "Watch alternate" }));
  await waitFor(() => expect(writes).toHaveLength(2));
  expect(writes[1]).toMatchObject({ date: {}, watched: true });
});
