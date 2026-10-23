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

test("movie proposals retain alias semantics and expose missing movie identities", async () => {
  const review = vi.fn();
  const paths: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string) => {
      paths.push(path);
      return Promise.resolve(
        new Response(
          JSON.stringify(
            path.includes("/resolve?")
              ? {
                  kind: "movie-aliases",
                  proposal_id: "movie-proposal",
                  existing_id: "",
                  existing_revision: "",
                  revision: "alias-revision",
                  relationships: [{ from: "anidb:episode:1", to: "tmdb:movie:2" }],
                  requires_consent: true,
                  provenance: { source: "animap", version: "vsynthetic" },
                  attribution: { notice: "Synthetic" },
                  unresolved: [
                    {
                      candidate: {
                        source_episode: "anidb:episode:1",
                        native_number: 1,
                        tmdb_movie_id: 3,
                      },
                      reason: "load_exact_native_episode_and_movie_metadata_first",
                    },
                  ],
                }
              : [
                  {
                    checksum: "a".repeat(64),
                    version: "vsynthetic",
                    source_revision: "r1",
                    attribution: { notice: "Synthetic" },
                  },
                ],
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
      <MappingDatasets t={(k) => en[k]} onReview={review} />
    </QueryClientProvider>,
  );
  await screen.findByRole("option", { name: "vsynthetic · r1" });
  await userEvent.selectOptions(
    screen.getByRole("combobox", { name: en.datasetVersion }),
    "a".repeat(64),
  );
  await userEvent.selectOptions(
    screen.getByRole("combobox", { name: en.mappingKind }),
    "movie-aliases",
  );
  await userEvent.type(screen.getByRole("spinbutton", { name: "AniDB ID" }), "42");
  await userEvent.click(screen.getByRole("button", { name: en.resolveDataset }));
  expect(await screen.findByText(en.movieAliasSafety)).toBeTruthy();
  expect(screen.getByRole("link", { name: /TMDB 3/ }).getAttribute("href")).toBe("#detail/movie/3");
  expect(paths.some((path) => path.endsWith("/resolve?kind=movie-aliases"))).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: en.reviewMapping }));
  expect(review).toHaveBeenCalledWith(
    expect.objectContaining({ kind: "movie-aliases", proposal_id: "movie-proposal" }),
  );
});
