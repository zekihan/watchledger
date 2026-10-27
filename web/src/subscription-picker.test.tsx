import { useState } from "react";
import { afterEach, expect, test } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SubscriptionPicker } from "./subscription-picker";
import { en } from "./i18n";

afterEach(cleanup);
test("subscription cards toggle independently, preserve unknown IDs and include Viki", async () => {
  function Fixture() {
    const [selected, setSelected] = useState([8, 9999]);
    return (
      <SubscriptionPicker
        providers={[
          { provider_id: 8, provider_name: "Netflix", logo_path: "/netflix.png" },
          { provider_id: 9, provider_name: "Prime" },
        ]}
        loading={false}
        error={null}
        selected={selected}
        onChange={setSelected}
        t={(key) => en[key]}
      />
    );
  }
  const { container } = render(<Fixture />);
  const netflix = screen.getByRole<HTMLInputElement>("checkbox", { name: "Netflix" });
  const prime = screen.getByRole<HTMLInputElement>("checkbox", { name: "Prime" });
  const unknown = screen.getByRole<HTMLInputElement>("checkbox", { name: /#9999/ });
  const viki = screen.getByRole<HTMLInputElement>("checkbox", { name: /Rakuten Viki/ });
  expect(netflix.checked && unknown.checked).toBe(true);
  expect(viki.checked).toBe(false);
  expect(screen.getAllByText(en.providerCountryUnknown)).toHaveLength(2);
  expect(
    container.querySelector('img[src="https://image.tmdb.org/t/p/w92/netflix.png"]'),
  ).toBeTruthy();
  await userEvent.click(prime);
  await userEvent.click(viki);
  expect(netflix.checked && prime.checked && viki.checked && unknown.checked).toBe(true);
  await userEvent.type(screen.getByRole("searchbox", { name: en.searchServices }), "viki");
  expect(screen.queryByRole("checkbox", { name: "Netflix" })).toBeNull();
  expect(screen.getByRole<HTMLInputElement>("checkbox", { name: /Rakuten Viki/ }).checked).toBe(
    true,
  );
  expect(screen.getByText(`${en.selectedServices}: 4`)).toBeTruthy();
});

test("country provider data wins for Viki and selected services survive provider failure", () => {
  const onChange = () => {};
  const { rerender } = render(
    <SubscriptionPicker
      providers={[{ provider_id: 344, provider_name: "Rakuten Viki" }]}
      loading={false}
      error={null}
      selected={[344]}
      onChange={onChange}
      t={(key) => en[key]}
    />,
  );
  expect(screen.getAllByRole("checkbox", { name: /Rakuten Viki/ })).toHaveLength(1);
  expect(screen.queryByText(en.providerCountryUnknown)).toBeNull();
  rerender(
    <SubscriptionPicker
      loading={false}
      error={new Error("Unavailable")}
      selected={[344, 8]}
      onChange={onChange}
      t={(key) => en[key]}
    />,
  );
  expect(screen.getByRole<HTMLInputElement>("checkbox", { name: /Rakuten Viki/ }).checked).toBe(
    true,
  );
  expect(screen.getByRole<HTMLInputElement>("checkbox", { name: /#8/ }).checked).toBe(true);
  expect(screen.getByRole("textbox", { name: en.services })).toBeTruthy();
});
