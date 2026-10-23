import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type { LibraryItem, List, Rule, Settings, Status, User } from "./api";
import type { T } from "./App";
import type { Key } from "./i18n";
const bases = ["planned", "watching", "completed", "hold", "dropped"] as const;
const initialRule: Rule = { op: "and", children: [{ op: "eq", field: "base", value: "watching" }] };
function RuleEditor({
  rule,
  onChange,
  t,
  depth = 0,
}: {
  rule: Rule;
  onChange: (r: Rule) => void;
  t: T;
  depth?: number;
}) {
  if (rule.op === "and" || rule.op === "or" || rule.op === "not")
    return (
      <fieldset className="rule-group">
        <legend>
          <select
            aria-label={t("condition")}
            value={rule.op}
            onChange={(e) => {
              onChange({
                op: e.target.value,
                children:
                  e.target.value === "not"
                    ? [rule.children?.[0] ?? { op: "eq", field: "base", value: "watching" }]
                    : rule.children,
              });
            }}
          >
            {["and", "or", "not"].map((op) => (
              <option key={op} value={op}>
                {t(op as Key)}
              </option>
            ))}
          </select>
        </legend>
        {rule.children?.map((c, i) => (
          <div className="rule-child" key={i}>
            <RuleEditor
              rule={c}
              depth={depth + 1}
              t={t}
              onChange={(r) => {
                onChange({
                  ...rule,
                  children: rule.children?.map((old, j) => (j === i ? r : old)),
                });
              }}
            />
            {rule.op !== "not" && (
              <button
                aria-label={t("remove")}
                onClick={() => {
                  onChange({ ...rule, children: rule.children?.filter((_, j) => j !== i) });
                }}
              >
                ×
              </button>
            )}
          </div>
        ))}
        {rule.op !== "not" && (
          <div className="toolbar">
            <button
              disabled={depth >= 10}
              onClick={() => {
                onChange({
                  ...rule,
                  children: [
                    ...(rule.children ?? []),
                    { op: "eq", field: "base", value: "watching" },
                  ],
                });
              }}
            >
              ＋ {t("addCondition")}
            </button>
            <button
              disabled={depth >= 10}
              onClick={() => {
                onChange({ ...rule, children: [...(rule.children ?? []), initialRule] });
              }}
            >
              ＋ {t("addGroup")}
            </button>
          </div>
        )}
      </fieldset>
    );
  return (
    <div className="rule-leaf">
      <select
        aria-label={t("field")}
        value={rule.field}
        onChange={(e) => {
          onChange({ ...rule, field: e.target.value });
        }}
      >
        {[
          "kind",
          "status",
          "base",
          "genre",
          "year",
          "rating",
          "availability",
          "service",
          "list",
        ].map((f) => (
          <option key={f} value={f}>
            {t(f as Key)}
          </option>
        ))}
      </select>
      <select
        aria-label={t("condition")}
        value={rule.op}
        onChange={(e) => {
          onChange({ ...rule, op: e.target.value });
        }}
      >
        {["eq", "gte", "lte", "unknown"].map((op) => (
          <option key={op} value={op}>
            {t(op === "unknown" ? "unknownValue" : (op as Key))}
          </option>
        ))}
      </select>
      {rule.op !== "unknown" && (
        <input
          aria-label={t("value")}
          value={rule.value ?? ""}
          onChange={(e) => {
            onChange({ ...rule, value: e.target.value });
          }}
        />
      )}
    </div>
  );
}
export function ListsPage({ t }: { t: T }) {
  const client = useQueryClient();
  const q = useQuery({ queryKey: ["lists"], queryFn: () => api<List[]>("/lists") });
  const library = useQuery({
    queryKey: ["library"],
    queryFn: () => api<LibraryItem[]>("/library"),
  });
  const [edit, setEdit] = useState<List | null>(null),
    [drag, setDrag] = useState<string | null>(null);
  const preview = useQuery({
    queryKey: ["preview", edit?.rule, edit?.sort],
    queryFn: () =>
      api<{ count: number; results: { id: string; title: string }[] }>("/lists/preview", "POST", {
        rule: edit?.rule,
        sort: edit?.sort,
      }),
    enabled: edit?.kind === "smart",
    retry: false,
  });
  const save = useMutation({
    mutationFn: (l: List) =>
      api("/lists", "POST", {
        id: l.id,
        name: l.name,
        kind: l.kind,
        rule: l.rule,
        sort: l.sort,
        members: l.members,
      }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["lists"] });
      setEdit(null);
    },
  });
  const remove = useMutation({
    mutationFn: (id: string) => api(`/lists/${id}`, "DELETE", {}),
    onSuccess: () => client.invalidateQueries({ queryKey: ["lists"] }),
  });
  return (
    <>
      <div className="page-heading">
        <div>
          <p className="eyebrow">CURATED BY YOU</p>
          <h1>{t("lists")}</h1>
        </div>
        <button
          className="primary"
          onClick={() => {
            setEdit({ id: "", name: "", kind: "manual", rule: null, sort: "title", members: [] });
          }}
        >
          ＋ {t("newList")}
        </button>
      </div>
      {[q.error, save.error, remove.error].filter(Boolean).map((e, i) => (
        <p role="alert" className="error" key={i}>
          {e?.message}
        </p>
      ))}
      {q.isPending && <p>{t("loading")}</p>}
      {edit ? (
        <section className="panel">
          <label>
            {t("name")}
            <input
              value={edit.name}
              onChange={(e) => {
                setEdit({ ...edit, name: e.target.value });
              }}
            />
          </label>
          <div className="toolbar">
            <select
              aria-label={t("kind")}
              value={edit.kind}
              onChange={(e) => {
                setEdit({
                  ...edit,
                  kind: e.target.value,
                  rule: e.target.value === "smart" ? initialRule : null,
                });
              }}
            >
              <option value="manual">{t("manual")}</option>
              <option value="smart">{t("smart")}</option>
            </select>
            <select
              aria-label={t("sort")}
              value={edit.sort}
              onChange={(e) => {
                setEdit({ ...edit, sort: e.target.value });
              }}
            >
              {["title", "release", "rating", "added"].map((s) => (
                <option key={s} value={s}>
                  {t(s as Key)}
                </option>
              ))}
            </select>
          </div>
          {edit.kind === "smart" && edit.rule ? (
            <>
              <RuleEditor
                rule={edit.rule}
                t={t}
                onChange={(rule) => {
                  setEdit({ ...edit, rule });
                }}
              />
              <div className="preview">
                <h3>
                  {t("preview")}: {preview.data?.count ?? "…"}
                </h3>
                {preview.error && <p role="alert">{preview.error.message}</p>}
                {preview.data?.results.slice(0, 20).map((x) => (
                  <a
                    className="chip"
                    key={x.id}
                    href={`#detail/${x.id.split(":").slice(1).join("/")}`}
                  >
                    {x.title}
                  </a>
                ))}
              </div>
            </>
          ) : (
            <>
              <p>{t("memberHint")}</p>
              <div className="member-picker">
                {library.data
                  ?.filter((x) => !edit.members.includes(x.id))
                  .map((x) => (
                    <button
                      key={x.id}
                      onClick={() => {
                        setEdit({ ...edit, members: [...edit.members, x.id] });
                      }}
                    >
                      ＋ {x.title}
                    </button>
                  ))}
              </div>
              <ol className="ordered-list">
                {edit.members.map((id, i) => (
                  <li
                    key={id}
                    draggable
                    onDragStart={() => {
                      setDrag(id);
                    }}
                    onDragOver={(e) => {
                      e.preventDefault();
                    }}
                    onDrop={(e) => {
                      e.preventDefault();
                      if (drag) {
                        const members = edit.members.filter((x) => x !== drag);
                        members.splice(i, 0, drag);
                        setEdit({ ...edit, members });
                      }
                    }}
                  >
                    <span>⠿ {library.data?.find((x) => x.id === id)?.title ?? id}</span>
                    <button
                      disabled={i === 0}
                      aria-label="Move up"
                      onClick={() => {
                        const members = [...edit.members];
                        members.splice(i, 1);
                        members.splice(i - 1, 0, id);
                        setEdit({ ...edit, members });
                      }}
                    >
                      ↑
                    </button>
                    <button
                      onClick={() => {
                        setEdit({ ...edit, members: edit.members.filter((x) => x !== id) });
                      }}
                    >
                      {t("remove")}
                    </button>
                  </li>
                ))}
              </ol>
            </>
          )}
          <div className="toolbar">
            <button
              className="primary"
              disabled={!edit.name || save.isPending || (!!preview.error && edit.kind === "smart")}
              onClick={() => {
                save.mutate(edit);
              }}
            >
              {t("save")}
            </button>
            <button
              onClick={() => {
                setEdit(null);
              }}
            >
              {t("cancel")}
            </button>
          </div>
        </section>
      ) : (
        <div className="list-grid">
          {q.data?.map((l) => (
            <article className="list-card" key={l.id}>
              <span>{l.kind === "smart" ? "✧" : "☷"}</span>
              <h2>{l.name}</h2>
              <small>
                {t(l.kind === "smart" ? "smart" : "manual")} · {l.id}
              </small>
              <div className="toolbar">
                <button
                  onClick={() => {
                    setEdit(l);
                  }}
                >
                  {t("details")}
                </button>
                <button
                  disabled={!!l.dependents?.length}
                  title={t("deleteBlocked")}
                  onClick={() => {
                    if (window.confirm(t("confirmDelete"))) remove.mutate(l.id);
                  }}
                >
                  {t("remove")}
                </button>
              </div>
              {!!l.dependents?.length && (
                <small>
                  {t("deleteBlocked")} {l.dependents.join(", ")}
                </small>
              )}
            </article>
          ))}
        </div>
      )}
    </>
  );
}
export function SettingsPage({ user, t }: { user: User; t: T }) {
  const client = useQueryClient();
  const [settings, setSettings] = useState<Settings>({
    language: "en",
    metadata_language: "en",
    country: "TR",
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    anime_view: "native",
    anime_title: "romaji",
    spoilers: false,
    services: [],
    ...user.Settings,
  });
  const [name, setName] = useState(""),
    [base, setBase] = useState("watching");
  const statuses = useQuery({ queryKey: ["statuses"], queryFn: () => api<Status[]>("/statuses") });
  const save = useMutation({
    mutationFn: () => api("/settings", "PUT", settings),
    onSuccess: () => client.invalidateQueries({ queryKey: ["me"] }),
  });
  const custom = useMutation({
    mutationFn: () => api("/statuses", "POST", { Name: name, Base: base }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["statuses"] });
      setName("");
    },
  });
  const remove = useMutation({
    mutationFn: (id: string) => api(`/statuses/${id}`, "DELETE", { replacement: base }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["statuses"] });
      await client.invalidateQueries({ queryKey: ["library"] });
    },
  });
  return (
    <>
      <h1>{t("settings")}</h1>
      <section className="panel settings-grid">
        <label>
          {t("language")}
          <select
            value={settings.language}
            onChange={(e) => {
              setSettings({ ...settings, language: e.target.value });
            }}
          >
            <option value="en">English</option>
            <option value="tr">Türkçe</option>
          </select>
        </label>
        <label>
          {t("metadataLanguage")}
          <input
            value={settings.metadata_language}
            onChange={(e) => {
              setSettings({ ...settings, metadata_language: e.target.value });
            }}
          />
        </label>
        <label>
          {t("country")}
          <input
            maxLength={2}
            value={settings.country}
            onChange={(e) => {
              setSettings({ ...settings, country: e.target.value.toUpperCase() });
            }}
          />
        </label>
        <label>
          {t("timezone")}
          <input
            value={settings.timezone}
            onChange={(e) => {
              setSettings({ ...settings, timezone: e.target.value });
            }}
          />
        </label>
        <label>
          {t("animeView")}
          <select
            value={settings.anime_view}
            onChange={(e) => {
              setSettings({ ...settings, anime_view: e.target.value });
            }}
          >
            <option value="native">{t("native")}</option>
            <option value="show">{t("show")}</option>
          </select>
        </label>
        <label>
          {t("animeTitle")}
          <select
            value={settings.anime_title}
            onChange={(e) => {
              setSettings({ ...settings, anime_title: e.target.value });
            }}
          >
            <option value="romaji">Romaji</option>
            <option value="native">{t("native")}</option>
            <option value="english">English</option>
          </select>
        </label>
        <label className="wide">
          {t("services")}
          <input
            value={settings.services?.join(",")}
            onChange={(e) => {
              setSettings({
                ...settings,
                services: e.target.value
                  .split(",")
                  .filter(Boolean)
                  .map(Number)
                  .filter(Number.isFinite),
              });
            }}
          />
        </label>
        <label className="check-label">
          <input
            type="checkbox"
            checked={settings.spoilers}
            onChange={(e) => {
              setSettings({ ...settings, spoilers: e.target.checked });
            }}
          />
          {t("spoilers")}
        </label>
        <div className="wide">
          <button
            className="primary"
            disabled={save.isPending}
            onClick={() => {
              save.mutate();
            }}
          >
            {t("save")}
          </button>
          {save.isSuccess && <span role="status"> {t("updated")}</span>}
        </div>
      </section>
      {[save.error, custom.error, remove.error].filter(Boolean).map((e, i) => (
        <p className="error" role="alert" key={i}>
          {e?.message}
        </p>
      ))}
      <section>
        <h2>{t("customStatuses")}</h2>
        <div className="toolbar">
          <input
            aria-label={t("name")}
            placeholder={t("name")}
            value={name}
            onChange={(e) => {
              setName(e.target.value);
            }}
          />
          <select
            aria-label={t("base")}
            value={base}
            onChange={(e) => {
              setBase(e.target.value);
            }}
          >
            {bases.map((b) => (
              <option key={b} value={b}>
                {t(b)}
              </option>
            ))}
          </select>
          <button
            disabled={!name || custom.isPending}
            onClick={() => {
              custom.mutate();
            }}
          >
            {t("save")}
          </button>
        </div>
        {statuses.data?.map((s) => (
          <div className="activity-row" key={s.id}>
            <strong>{s.name}</strong>
            <small>
              {s.base} {s.used ? "🔒" : ""}
            </small>
            <button
              onClick={() => {
                if (window.confirm(`${t("replacement")}: ${base}?`)) remove.mutate(s.id);
              }}
            >
              {t("remove")}
            </button>
          </div>
        ))}
      </section>
      <section>
        <h2>
          {t("export")} / {t("restore")}
        </h2>
        <p className="notice">{t("notReady")}</p>
      </section>
      <button
        onClick={() => {
          void api("/logout", "POST", {})
            .then(() => {
              location.reload();
            })
            .catch(() => {
              save.reset();
            });
        }}
      >
        {t("logout")}
      </button>
    </>
  );
}
