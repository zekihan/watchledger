import { expect, test } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { ProviderCard } from "./provider-card";

test("provider cards show safe logos with a fallback and do not invent links", () => {
  const provider = { provider_id: 1, provider_name: "Fixture Service", logo_path: "/fixture.png" };
  const { container, rerender } = render(
    <ProviderCard provider={provider} link="https://example.com/title" />,
  );
  expect(screen.getByRole<HTMLAnchorElement>("link", { name: "Fixture Service" }).href).toBe(
    "https://example.com/title",
  );
  const image = container.querySelector("img");
  expect(image?.getAttribute("src")).toBe("https://image.tmdb.org/t/p/w92/fixture.png");
  if (!image) throw new Error("Missing provider logo");
  fireEvent.error(image);
  expect(container.querySelector("img")).toBeNull();
  expect(screen.getByText("F")).toBeTruthy();
  rerender(<ProviderCard provider={provider} />);
  expect(screen.queryByRole("link")).toBeNull();
  expect(screen.getByText("Fixture Service")).toBeTruthy();
});
