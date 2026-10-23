import { afterEach, expect, test } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { CreditsPage } from "./credits";
import { en, tr } from "./i18n";
afterEach(cleanup);
test.each([en, tr])(
  "credits retain provider notices and source links in both languages",
  (messages) => {
    render(<CreditsPage t={(k) => messages[k]} />);
    expect(screen.getByRole("heading", { name: messages.sourceCredits })).toBeTruthy();
    expect(screen.getByText(messages.attribution)).toBeTruthy();
    expect(screen.getByRole("link", { name: "TMDB ↗" }).getAttribute("href")).toBe(
      "https://www.themoviedb.org",
    );
    expect(screen.getByRole("link", { name: "JustWatch ↗" }).getAttribute("href")).toBe(
      "https://www.justwatch.com",
    );
    expect(screen.getByText(messages.sourceRights)).toBeTruthy();
  },
);
