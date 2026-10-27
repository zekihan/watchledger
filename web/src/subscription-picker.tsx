import { useState } from "react";
import type { T } from "./App";
import { ProviderMark } from "./provider-card";
import type { Provider } from "./provider-card";

// TMDB identifies Rakuten Viki as 344, even where its country list omits the service.
const viki: Provider = {
  provider_id: 344,
  provider_name: "Rakuten Viki",
  logo_path: "/AoNsevRecTdVv4xQEf1oLRGO2yh.png",
};

export function SubscriptionPicker({
  providers,
  loading,
  error,
  selected,
  onChange,
  t,
}: {
  providers?: Provider[];
  loading: boolean;
  error: Error | null;
  selected: number[];
  onChange: (ids: number[]) => void;
  t: T;
}) {
  const [search, setSearch] = useState("");
  const [manual, setManual] = useState(selected.join(","));
  const options = [...(providers ?? [])];
  if (!options.some((provider) => provider.provider_id === viki.provider_id)) options.push(viki);
  for (const id of selected) {
    if (!options.some((provider) => provider.provider_id === id))
      options.push({ provider_id: id, provider_name: `#${id} (${t("unknownValue")})` });
  }
  const visible = options
    .sort((a, b) => a.provider_name.localeCompare(b.provider_name))
    .filter((provider) =>
      provider.provider_name.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase()),
    );
  return (
    <fieldset className="wide subscription-picker">
      <legend>{t("subscriptions")}</legend>
      <div className="subscription-header">
        <label>
          {t("searchServices")}
          <input type="search" value={search} onChange={(event) => setSearch(event.target.value)} />
        </label>
        <small>
          {t("selectedServices")}: {selected.length}
        </small>
      </div>
      {loading && <p role="status">{t("loading")}</p>}
      {error && <p role="status">{error.message}</p>}
      <div className="subscription-grid">
        {visible.map((provider) => (
          <label className="subscription-choice" key={provider.provider_id}>
            <input
              type="checkbox"
              checked={selected.includes(provider.provider_id)}
              onChange={(event) => {
                const ids = event.target.checked
                  ? [...selected, provider.provider_id]
                  : selected.filter((id) => id !== provider.provider_id);
                setManual(ids.join(","));
                onChange(ids);
              }}
            />
            <ProviderMark provider={provider} />
            <span className="subscription-choice-text">
              <strong>{provider.provider_name}</strong>
              {providers &&
                !providers.some((item) => item.provider_id === provider.provider_id) && (
                  <small>{t("providerCountryUnknown")}</small>
                )}
            </span>
          </label>
        ))}
      </div>
      {!visible.length && <p>{t("noMatchingServices")}</p>}
      {error && (
        <label className="subscription-manual">
          {t("services")}
          <input
            value={manual}
            onChange={(event) => {
              const value = event.target.value;
              setManual(value);
              onChange(value.split(",").filter(Boolean).map(Number).filter(Number.isFinite));
            }}
          />
        </label>
      )}
    </fieldset>
  );
}
