import { afterEach, expect, test, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SimklPanel } from "./imports";
import { PortabilityPanel } from "./pages";
import { CustomListImport } from "./custom-list-import";
import { en } from "./i18n";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
const cases = [
  { name: "SIMKL", label: en.simklFile, panel: <SimklPanel t={(key) => en[key]} backed /> },
  { name: "restore", label: en.restore, panel: <PortabilityPanel t={(key) => en[key]} /> },
  {
    name: "custom lists",
    label: en.customListFiles,
    panel: <CustomListImport t={(key) => en[key]} backed />,
  },
];
function mount(panel: React.ReactNode) {
  return render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      {panel}
    </QueryClientProvider>,
  );
}
function pendingFile() {
  let finish!: (text: string) => void;
  const promise = new Promise<string>((resolve) => {
    finish = resolve;
  });
  const file = new File(["{}"], "synthetic.json", { type: "application/json" });
  Object.defineProperty(file, "text", { value: () => promise });
  return { file, finish };
}
test.each(cases)(
  "$name stops a delayed file read before uploading after navigation",
  async ({ label, panel }) => {
    const fetch = vi.fn(() => Promise.resolve(new Response("[]", { status: 200 })));
    vi.stubGlobal("fetch", fetch);
    const view = mount(panel);
    const pending = pendingFile();
    await userEvent.upload(screen.getByLabelText(label), pending.file);
    fetch.mockClear();
    view.unmount();
    await act(async () => {
      pending.finish("{}");
      await Promise.resolve();
    });
    expect(fetch).not.toHaveBeenCalled();
  },
);
test.each(cases)(
  "$name aborts an in-flight preview on navigation without applying",
  async ({ label, panel }) => {
    let signal: AbortSignal | null | undefined;
    const writes: string[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn((path: string, init?: RequestInit) => {
        if (init?.method === "POST") {
          writes.push(path);
          signal = init.signal;
          return new Promise<Response>(() => {});
        }
        return Promise.resolve(new Response("[]", { status: 200 }));
      }),
    );
    const view = mount(panel);
    const pending = pendingFile();
    pending.finish("{}");
    await userEvent.upload(screen.getByLabelText(label), pending.file);
    expect(signal?.aborted).toBe(false);
    view.unmount();
    expect(signal?.aborted).toBe(true);
    expect(writes).toHaveLength(1);
    expect(writes[0]).toMatch(/\/preview$/);
  },
);

test.each(cases)(
  "$name cancels an earlier request when the replacement file is invalid",
  async ({ label, panel }) => {
    let signal: AbortSignal | null | undefined;
    let finish!: (response: Response) => void;
    vi.stubGlobal(
      "fetch",
      vi.fn((path: string, init?: RequestInit) => {
        if (path.endsWith("/preview")) {
          signal = init?.signal;
          return new Promise<Response>((resolve) => {
            finish = resolve;
          });
        }
        return Promise.resolve(new Response("[]", { status: 200 }));
      }),
    );
    mount(panel);
    const first = pendingFile();
    first.finish("{}");
    await userEvent.upload(screen.getByLabelText(label), first.file);
    const replacement = pendingFile();
    replacement.finish("invalid");
    await userEvent.upload(screen.getByLabelText(label), replacement.file);
    expect(signal?.aborted).toBe(true);
    expect(await screen.findByRole("alert")).toBeTruthy();
    await act(async () => {
      finish(new Response("stale failure", { status: 500 }));
      await Promise.resolve();
    });
    expect(screen.queryByText("stale failure")).toBeNull();
    expect(screen.queryByRole("button", { name: en.apply })).toBeNull();
  },
);

test("restore uploads original JSON numbers without breaking catalog proof", async () => {
  const raw =
    '{"version":1,"catalog_proof":"signed-fixture","catalog":[{"metadata":{"en":{"vote_average":8.0}}}]}';
  let uploaded: BodyInit | null | undefined;
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) => {
      if (path === "/api/restore/preview") uploaded = init?.body;
      return Promise.resolve(
        new Response(
          JSON.stringify(
            path === "/api/restore/preview"
              ? { id: "proof", additions: 1, conflicts: [], history_candidates: 0 }
              : [],
          ),
          { status: 200 },
        ),
      );
    }),
  );
  mount(<PortabilityPanel t={(key) => en[key]} />);
  const file = new File([raw], "backup.json", { type: "application/json" });
  Object.defineProperty(file, "text", { value: () => Promise.resolve(raw) });
  await userEvent.upload(screen.getByLabelText(en.restore), file);
  expect(await screen.findByText(en.previewReady)).toBeTruthy();
  expect(uploaded).toBe(raw);
});
