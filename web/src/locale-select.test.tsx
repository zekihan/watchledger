import { afterEach, expect, test } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { LocaleSelect } from "./locale-select";
import { en, tr } from "./i18n";

afterEach(cleanup);
test("language and country choices use localized names while retaining codes and custom values", async () => {
  function Fixture() {
    const [language, setLanguage] = useState("en");
    const [country, setCountry] = useState("TR");
    return (
      <>
        <LocaleSelect
          label="Metadata language"
          type="language"
          value={language}
          language="en"
          onChange={setLanguage}
          t={(k) => en[k]}
        />
        <LocaleSelect
          label="Country"
          type="region"
          value={country}
          language="en"
          onChange={setCountry}
          t={(k) => en[k]}
        />
      </>
    );
  }
  render(<Fixture />);
  const language = screen.getByRole<HTMLSelectElement>("combobox", { name: "Metadata language" });
  const country = screen.getByRole<HTMLSelectElement>("combobox", { name: "Country" });
  expect(language.selectedOptions[0]?.textContent).toBe(
    new Intl.DisplayNames(["en"], { type: "language" }).of("en"),
  );
  expect(country.selectedOptions[0]?.textContent).toBe(
    new Intl.DisplayNames(["en"], { type: "region" }).of("TR"),
  );
  await userEvent.selectOptions(language, "tr");
  expect(language.value).toBe("tr");
  await userEvent.selectOptions(country, "US");
  expect(country.value).toBe("US");
  await userEvent.selectOptions(language, "__custom");
  const input = screen.getByRole("textbox", { name: "Custom code: Metadata language" });
  await userEvent.clear(input);
  await userEvent.type(input, "en-US");
  expect((input as HTMLInputElement).value).toBe("en-US");
});

test("existing regional metadata languages remain selectable and use Turkish names", () => {
  render(
    <LocaleSelect
      label="İçerik dili"
      type="language"
      value="en-US"
      language="tr"
      onChange={() => {}}
      t={(k) => tr[k]}
    />,
  );
  const control = screen.getByRole<HTMLSelectElement>("combobox", { name: "İçerik dili" });
  expect(control.value).toBe("en-US");
  expect(control.selectedOptions[0]?.textContent).toBe(
    new Intl.DisplayNames(["tr"], { type: "language" }).of("en-US"),
  );
});
