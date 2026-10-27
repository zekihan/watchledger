import { useMemo, useState } from "react";
import type { T } from "./App";

type Choice = { code: string; name: string };

function displayName(names: Intl.DisplayNames, code: string): string | undefined {
  try {
    return names.of(code);
  } catch {
    return undefined;
  }
}

export function LocaleSelect({
  label,
  type,
  value,
  language,
  onChange,
  t,
}: {
  label: string;
  type: "language" | "region";
  value: string;
  language: string;
  onChange: (value: string) => void;
  t: T;
}) {
  const [custom, setCustom] = useState(false);
  const choices = useMemo(() => {
    const names = new Intl.DisplayNames([language], { type, fallback: "none" });
    const found: Choice[] = [];
    for (let a = 65; a <= 90; a++) {
      for (let b = 65; b <= 90; b++) {
        const raw = String.fromCharCode(a, b);
        const code = type === "language" ? raw.toLowerCase() : raw;
        const name = displayName(names, code);
        if (name) found.push({ code, name });
      }
    }
    if (value && !found.some((choice) => choice.code === value)) {
      found.push({ code: value, name: displayName(names, value) ?? value });
    }
    return found.sort((a, b) => a.name.localeCompare(b.name, language));
  }, [language, type, value]);
  return (
    <div className="locale-field">
      <label>
        {label}
        <select
          value={custom ? "__custom" : value}
          onChange={(event) => {
            if (event.target.value === "__custom") setCustom(true);
            else {
              setCustom(false);
              onChange(event.target.value);
            }
          }}
        >
          {choices.map((choice) => (
            <option key={choice.code} value={choice.code}>
              {choice.name}
            </option>
          ))}
          <option value="__custom">{t("otherCode")}</option>
        </select>
      </label>
      {custom && (
        <label>
          {`${t("customCode")}: ${label}`}
          <input
            value={value}
            maxLength={type === "region" ? 2 : undefined}
            onChange={(event) =>
              onChange(type === "region" ? event.target.value.toUpperCase() : event.target.value)
            }
          />
        </label>
      )}
    </div>
  );
}
