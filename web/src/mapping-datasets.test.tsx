import { afterEach, expect, test, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MappingDatasets } from "./mapping-datasets";
import { en } from "./i18n";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
test("dataset proposals require explicit review and clear when the selected source changes", async () => {
  const review = vi.fn();
  const checksum = "a".repeat(64);
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) =>
      Promise.resolve(
        new Response(
          JSON.stringify(
            path.endsWith("/resolve")
              ? {
                  proposal_id: "p1",
                  existing_id: "m1",
                  existing_revision: "old",
                  revision: "new",
                  relationships: [{ from: "anidb:episode:1", to: "tmdb:episode:2" }],
                  unresolved: [
                    {
                      candidate: {
                        source_episode: "anidb:episode:3",
                        native_number: 3,
                        inferred: true,
                      },
                      reason: "inferred_default_requires_manual_review",
                    },
                  ],
                  requires_consent: true,
                  provenance: { source: "animap", version: "vsynthetic" },
                  attribution: { notice: "Synthetic attribution" },
                }
              : [
                  {
                    checksum,
                    version: "vsynthetic",
                    source_revision: "r1",
                    attribution: { notice: "Synthetic attribution" },
                  },
                ],
          ),
          { status: 200 },
        ),
      ),
    ),
  );
  render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
        })
      }
    >
      <MappingDatasets t={(k) => en[k]} onReview={review} />
    </QueryClientProvider>,
  );
  await screen.findByRole("option", { name: "vsynthetic · r1" });
  await userEvent.selectOptions(
    screen.getByRole("combobox", { name: en.datasetVersion }),
    checksum,
  );
  await userEvent.type(screen.getByRole("spinbutton", { name: "AniDB ID" }), "42");
  await userEvent.click(screen.getByRole("button", { name: en.resolveDataset }));
  const button = await screen.findByRole("button", { name: en.reviewMapping });
  expect(review).not.toHaveBeenCalled();
  expect(screen.getByText(/inferred_default_requires_manual_review/)).toBeTruthy();
  await userEvent.click(button);
  expect(review).toHaveBeenCalledWith(
    expect.objectContaining({
      id: "m1",
      revision: "old",
      proposal_id: "p1",
      provenance: { source: "animap", version: "vsynthetic" },
    }),
  );
  await userEvent.clear(screen.getByRole("spinbutton", { name: "AniDB ID" }));
  await waitFor(() => expect(screen.queryByRole("button", { name: en.reviewMapping })).toBeNull());
});
