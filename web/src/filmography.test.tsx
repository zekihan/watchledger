import { afterEach, expect, test } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Filmography } from "./filmography";
import { en } from "./i18n";
afterEach(cleanup);
test("filmography deduplicates source identities, paginates all supplied credits and resets filters", async () => {
  const cast = Array.from({ length: 26 }, (_, id) => ({
    id,
    title: `Film ${id}`,
    media_type: "movie",
    release_date: `2025-01-${String(id + 1).padStart(2, "0")}`,
    popularity: id,
  }));
  render(
    <Filmography
      t={(key) => en[key]}
      credits={{
        cast: [
          ...cast,
          cast[0]!,
          { id: 0, name: "TV credit", media_type: "tv", first_air_date: "2026-01-01" },
        ],
        crew: [{ id: 2, title: "Crew film", media_type: "movie", popularity: 100 }],
      }}
    />,
  );
  expect(screen.getByRole("status").textContent).toBe(`27 ${en.title}`);
  expect(screen.getAllByRole("link")).toHaveLength(24);
  expect(screen.getByRole("link", { name: /TV credit/ }).getAttribute("href")).toBe("#detail/tv/0");
  await userEvent.click(screen.getByRole("button", { name: en.nextPage }));
  expect(screen.getAllByRole("link")).toHaveLength(3);
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.kind }), "tv");
  expect(screen.getAllByRole("link")).toHaveLength(1);
  expect(screen.getByRole("link", { name: /TV credit/ })).toBeTruthy();
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.credits }), "crew");
  expect(screen.getByText(en.empty)).toBeTruthy();
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.kind }), "movie");
  expect(screen.getByRole("link", { name: /Crew film/ })).toBeTruthy();
});
test("person credit sorting uses release dates or popularity without mutating source order", async () => {
  const cast = [
    {
      id: 1,
      title: "Earlier popular",
      media_type: "movie",
      release_date: "2020-01-01",
      popularity: 20,
    },
    { id: 2, title: "Recent", media_type: "movie", release_date: "2025-01-01", popularity: 1 },
  ];
  render(<Filmography t={(key) => en[key]} credits={{ cast, crew: [] }} />);
  expect(screen.getAllByRole("link")[0]?.textContent).toContain("Recent");
  await userEvent.selectOptions(screen.getByRole("combobox", { name: en.sort }), "popularity");
  expect(screen.getAllByRole("link")[0]?.textContent).toContain("Earlier popular");
  expect(cast.map((item) => item.id)).toEqual([1, 2]);
});
