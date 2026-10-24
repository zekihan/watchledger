import { usePreviewRequest } from "./use-preview-request";
import { StatusReplacement } from "./status-replacement";
import { InstallPanel } from "./install";
import type { useInstall } from "./use-install";
import { CustomListImport } from "./custom-list-import";
import { SimklPanel } from "./imports";
import { useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type { LibraryItem, List, Rule, Settings, Status, User } from "./api";
import type { T } from "./App";
import type { Key } from "./i18n";
const bases = ["planned", "watching", "completed", "hold", "dropped"] as const;
type RuleChoice = { value: string; label: string };
const initialRule: Rule = { op: "and", children: [{ op: "eq", field: "base", value: "watching" }] };
function RuleEditor({
  rule,
  onChange,
  t,
  depth = 0,
  choices,
}: {
  rule: Rule;
  onChange: (r: Rule) => void;
  t: T;
  depth?: number;
  choices: Record<string, RuleChoice[]>;
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
              choices={choices}
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
          onChange({ op: "eq", field: e.target.value, value: "" });
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
          onChange({
            ...rule,
            op: e.target.value,
            value: e.target.value === "unknown" ? undefined : rule.value,
          });
        }}
      >
        {(rule.field === "list"
          ? ["eq"]
          : rule.field === "year" || rule.field === "rating"
            ? ["eq", "gte", "lte", "unknown"]
            : ["eq", "unknown"]
        ).map((op) => (
          <option key={op} value={op}>
            {t(op === "unknown" ? "unknownValue" : (op as Key))}
          </option>
        ))}
      </select>
      {rule.op !== "unknown" &&
        (choices[rule.field ?? ""] ? (
          <select
            aria-label={t("value")}
            value={rule.value ?? ""}
            onChange={(event) => {
              onChange({ ...rule, value: event.target.value });
            }}
          >
            <option value="">{t("select")}</option>
            {rule.value &&
              !choices[rule.field ?? ""]?.some((option) => option.value === rule.value) && (
                <option value={rule.value} disabled>
                  {t("unavailableChoice")}: {rule.value}
                </option>
              )}
            {choices[rule.field ?? ""]?.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        ) : (
          <input
            aria-label={t("value")}
            type={
              rule.field === "year" || rule.field === "rating" || rule.field === "service"
                ? "number"
                : "text"
            }
            step="1"
            min={rule.field === "rating" ? 1 : undefined}
            max={rule.field === "rating" ? 10 : undefined}
            value={rule.value ?? ""}
            onChange={(event) => {
              onChange({ ...rule, value: event.target.value });
            }}
          />
        ))}
    </div>
  );
}
export function ListsPage({ t, user }: { t: T; user: User }) {
  const client = useQueryClient();
  const q = useQuery({ queryKey: ["lists"], queryFn: () => api<List[]>("/lists") });
  const library = useQuery({
    queryKey: ["library"],
    queryFn: () => api<LibraryItem[]>("/library"),
  });
  const [edit, setEdit] = useState<List | null>(null);
  const drag = useRef<{ list: string; member: string } | null>(null);
  const [orderNotice, setOrderNotice] = useState("");
  function moveMember(id: string, position: number) {
    if (
      save.isPending ||
      !edit ||
      !edit.members.includes(id) ||
      position < 0 ||
      position >= edit.members.length
    )
      return;
    const members = edit.members.filter((member) => member !== id);
    members.splice(position, 0, id);
    setEdit({ ...edit, members });
    const title = library.data?.find((item) => item.id === id)?.title ?? id;
    setOrderNotice(`${title}: ${t("position")} ${position + 1} / ${members.length}`);
  }
  const [viewID, setViewID] = useState<string | null>(null);
  const statuses = useQuery({ queryKey: ["statuses"], queryFn: () => api<Status[]>("/statuses") });
  const needsProviders = edit?.kind === "smart" && hasRuleField(edit.rule, "service");
  const providers = useQuery({
    queryKey: ["provider-options", user.Settings.country ?? "TR"],
    enabled: needsProviders,
    queryFn: async () => {
      const country = user.Settings.country ?? "TR";
      const responses = await Promise.all([
        api<{ results: { provider_id: number; provider_name: string }[] }>(
          `/providers/movie?country=${country}`,
        ),
        api<{ results: { provider_id: number; provider_name: string }[] }>(
          `/providers/tv?country=${country}`,
        ),
      ]);
      return [
        ...new Map(
          responses
            .flatMap((response) => response.results)
            .map((provider) => [provider.provider_id, provider]),
        ).values(),
      ].sort((a, b) => a.provider_name.localeCompare(b.provider_name));
    },
    retry: false,
  });
  const genres = [
    ...new Map(
      (library.data ?? [])
        .flatMap((item) =>
          Object.values(item.metadata).flatMap((metadata) => metadata.genres ?? []),
        )
        .map((genre) => [genre.id, genre]),
    ).values(),
  ].sort((a, b) => a.name.localeCompare(b.name));
  const choices: Record<string, RuleChoice[]> = {
    kind: ["movie", "tv", "anime"].map((kind) => ({ value: kind, label: t(kind as Key) })),
    base: bases.map((base) => ({ value: base, label: t(base) })),
    status: [
      ...bases.map((base) => ({ value: base, label: t(base) })),
      ...(statuses.data ?? []).map((status) => ({ value: status.id, label: status.name })),
    ],
    list: (q.data ?? [])
      .filter((list) => list.id !== edit?.id)
      .map((list) => ({ value: list.id, label: list.name })),
    genre: genres.map((genre) => ({ value: String(genre.id), label: genre.name })),
    availability: [{ value: user.Settings.country ?? "TR", label: user.Settings.country ?? "TR" }],
  };
  if (providers.data)
    choices.service = providers.data.map((provider) => ({
      value: String(provider.provider_id),
      label: provider.provider_name,
    }));
  const viewed = q.data?.find((list) => list.id === viewID);

  const preview = useQuery({
    queryKey: ["preview", edit?.id, edit?.rule, edit?.sort],
    queryFn: ({ signal }) =>
      api<{ count: number; results: { id: string; title: string }[] }>(
        "/lists/preview",
        "POST",
        {
          id: edit?.id,
          rule: edit?.rule,
          sort: edit?.sort,
        },
        signal,
      ),
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
      await client.invalidateQueries({ queryKey: ["list-results"] });
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
          <p className="eyebrow">{t("curated")}</p>
          <h1>{t("lists")}</h1>
        </div>
        <button
          className="primary"
          disabled={save.isPending}
          onClick={() => {
            setViewID(null);
            drag.current = null;
            setOrderNotice("");
            setEdit({ id: "", name: "", kind: "manual", rule: null, sort: "title", members: [] });
          }}
        >
          ＋ {t("newList")}
        </button>
      </div>
      {[q, library, statuses, ...(needsProviders ? [providers] : [])].map((query, index) =>
        query.error ? (
          <div role="alert" className="error" key={index}>
            {query.error.message}
            <button
              disabled={query.isFetching}
              onClick={() => {
                void query.refetch();
              }}
            >
              {t("retry")}
            </button>
          </div>
        ) : null,
      )}
      {[save.error, remove.error].filter(Boolean).map((error, index) => (
        <p role="alert" className="error" key={index}>
          {error?.message}
        </p>
      ))}
      {q.isPending && <p role="status">{t("loading")}</p>}
      {!edit && !viewed && q.isSuccess && q.data.length === 0 && <p>{t("empty")}</p>}
      {edit ? (
        <section className="panel">
          <fieldset disabled={save.isPending} style={{ border: 0, padding: 0, minWidth: 0 }}>
            <legend>{t("editList")}</legend>
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
              {edit.kind === "smart" && (
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
              )}
            </div>
            {edit.kind === "smart" && edit.rule ? (
              <>
                <RuleEditor
                  rule={edit.rule}
                  choices={choices}
                  t={t}
                  onChange={(rule) => {
                    setEdit({ ...edit, rule });
                  }}
                />
                <div className="preview">
                  <h3>{t("preview")}</h3>
                  {preview.isFetching && <p role="status">{t("loading")}</p>}
                  {preview.error && (
                    <div role="alert">
                      {preview.error.message}
                      <button
                        disabled={preview.isFetching}
                        onClick={() => {
                          void preview.refetch();
                        }}
                      >
                        {t("retry")}
                      </button>
                    </div>
                  )}
                  {!preview.isFetching && !preview.error && preview.data && (
                    <p role="status">
                      {preview.data.count} {t("title")}
                    </p>
                  )}
                  {!preview.isFetching &&
                    !preview.error &&
                    preview.data?.results.slice(0, 20).map((x) => (
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
                <p role="status" aria-live="polite">
                  {orderNotice}
                </p>
                <ol className="ordered-list">
                  {edit.members.map((id, i) => (
                    <li
                      key={id}
                      draggable={!save.isPending}
                      onDragStart={(event) => {
                        drag.current = { list: edit.id, member: id };
                        event.dataTransfer.effectAllowed = "move";
                        event.dataTransfer.setData("text/plain", id);
                      }}
                      onDragEnd={() => {
                        drag.current = null;
                      }}
                      onDragOver={(e) => {
                        e.preventDefault();
                      }}
                      onDrop={(e) => {
                        e.preventDefault();
                        const source = drag.current;
                        drag.current = null;
                        if (source?.list === edit.id) moveMember(source.member, i);
                      }}
                    >
                      <span>⠿ {library.data?.find((x) => x.id === id)?.title ?? id}</span>
                      <button
                        disabled={i === 0}
                        aria-label={`${t("moveUp")}: ${library.data?.find((item) => item.id === id)?.title ?? id}`}
                        onClick={() => {
                          moveMember(id, i - 1);
                        }}
                      >
                        ↑
                      </button>
                      <button
                        disabled={i === edit.members.length - 1}
                        aria-label={`${t("moveDown")}: ${library.data?.find((item) => item.id === id)?.title ?? id}`}
                        onClick={() => {
                          moveMember(id, i + 1);
                        }}
                      >
                        ↓
                      </button>
                      <button
                        aria-label={`${t("remove")}: ${library.data?.find((item) => item.id === id)?.title ?? id}`}
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
                disabled={
                  !edit.name ||
                  save.isPending ||
                  (edit.kind === "smart" &&
                    (!preview.data || preview.isFetching || !!preview.error))
                }
                onClick={() => {
                  save.mutate(edit);
                }}
              >
                {t("save")}
              </button>
              <button
                onClick={() => {
                  drag.current = null;
                  setOrderNotice("");
                  setEdit(null);
                }}
              >
                {t("cancel")}
              </button>
            </div>
          </fieldset>
        </section>
      ) : viewed ? (
        <ListContents
          list={viewed}
          t={t}
          onClose={() => {
            setViewID(null);
          }}
          onEdit={() => {
            drag.current = null;
            setOrderNotice("");
            setEdit(viewed);
          }}
        />
      ) : (
        <div className="list-grid">
          {q.data?.map((l) => (
            <article className="list-card" key={l.id}>
              <span>{l.kind === "smart" ? "✧" : "☷"}</span>
              <h2>{l.name}</h2>
              <small>{t(l.kind === "smart" ? "smart" : "manual")}</small>
              <div className="toolbar">
                <button
                  onClick={() => {
                    drag.current = null;
                    setOrderNotice("");
                    setEdit(l);
                  }}
                >
                  {t("editList")}
                </button>
                <button
                  onClick={() => {
                    setViewID(l.id);
                  }}
                >
                  {t("viewList")}
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
                  {t("deleteBlocked")}{" "}
                  {l.dependents
                    .map((id) => q.data?.find((list) => list.id === id)?.name ?? id)
                    .join(", ")}
                </small>
              )}
            </article>
          ))}
        </div>
      )}
    </>
  );
}
function hasRuleField(rule: Rule | null | undefined, field: string): boolean {
  return Boolean(
    rule && (rule.field === field || rule.children?.some((child) => hasRuleField(child, field))),
  );
}
function ListContents({
  list,
  t,
  onClose,
  onEdit,
}: {
  list: List;
  t: T;
  onClose: () => void;
  onEdit: () => void;
}) {
  const results = useQuery({
    queryKey: ["list-results", list.id],
    queryFn: ({ signal }) =>
      api<{ count: number; results: { id: string; title: string }[] }>(
        `/lists/${list.id}/results`,
        "GET",
        undefined,
        signal,
      ),
  });
  return (
    <section className="panel">
      <h2>{list.name}</h2>
      <div className="toolbar">
        <button onClick={onClose}>{t("back")}</button>
        <button onClick={onEdit}>{t("editList")}</button>
      </div>
      {results.isPending && <p>{t("loading")}</p>}
      {results.error && (
        <div role="alert">
          {results.error.message}
          <button
            onClick={() => {
              void results.refetch();
            }}
          >
            {t("retry")}
          </button>
        </div>
      )}
      {results.data && (
        <>
          <p>
            {results.data.count} {t("title")}
          </p>
          {results.data.count === 0 ? (
            <p>{t("empty")}</p>
          ) : (
            <ol className="ordered-list">
              {results.data.results.map((item) => (
                <li key={item.id}>
                  <a href={`#detail/${item.id.split(":").slice(1).join("/")}`}>{item.title}</a>
                </li>
              ))}
            </ol>
          )}
        </>
      )}
    </section>
  );
}
export function SettingsPage({
  user,
  t,
  installation,
}: {
  user: User;
  t: T;
  installation: ReturnType<typeof useInstall>;
}) {
  const client = useQueryClient();
  const [deleting, setDeleting] = useState<Status | null>(null);
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
  const providers = useQuery({
    queryKey: ["provider-options", settings.country],
    queryFn: async () => {
      const responses = await Promise.all([
        api<{ results: { provider_id: number; provider_name: string }[] }>(
          `/providers/movie?country=${settings.country ?? "TR"}`,
        ),
        api<{ results: { provider_id: number; provider_name: string }[] }>(
          `/providers/tv?country=${settings.country ?? "TR"}`,
        ),
      ]);
      return [
        ...new Map(responses.flatMap((x) => x.results).map((p) => [p.provider_id, p])).values(),
      ].sort((a, b) => a.provider_name.localeCompare(b.provider_name));
    },
    retry: false,
  });
  const save = useMutation({
    mutationFn: () =>
      api(
        "/settings",
        "PUT",
        Object.fromEntries(Object.entries(settings).filter(([key]) => key !== "up_next_order")),
      ),
    onSuccess: () => client.invalidateQueries(),
  });
  const custom = useMutation({
    mutationFn: () => api("/statuses", "POST", { Name: name, Base: base }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["statuses"] });
      setName("");
    },
  });
  const rename = useMutation({
    mutationFn: (s: Status) => api("/statuses", "POST", { ID: s.id, Name: s.name, Base: s.base }),
    onSuccess: () => client.invalidateQueries({ queryKey: ["statuses"] }),
  });
  const remove = useMutation({
    mutationFn: ({ id, replacement }: { id: string; replacement: string }) =>
      api(`/statuses/${id}`, "DELETE", { replacement }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["statuses"] });
      await client.invalidateQueries({ queryKey: ["library"] });
      setDeleting(null);
    },
  });
  const logout = useMutation({
    mutationFn: () => api("/logout", "POST", {}),
    onSuccess: () => {
      client.clear();
      location.reload();
    },
  });
  return (
    <>
      <h1>{t("settings")}</h1>
      {deleting && (
        <StatusReplacement
          key={deleting.id}
          status={deleting}
          statuses={statuses.data ?? []}
          pending={remove.isPending}
          error={remove.error}
          t={t}
          cancel={() => {
            setDeleting(null);
            remove.reset();
          }}
          submit={(replacement) => {
            remove.mutate({ id: deleting.id, replacement });
          }}
        />
      )}
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
            <option value="english">{t("englishTitle")}</option>
          </select>
        </label>
        <label className="wide">
          {t("subscriptions")}
          {providers.data ? (
            <select
              multiple
              size={8}
              value={settings.services?.map(String) ?? []}
              onChange={(e) => {
                setSettings({
                  ...settings,
                  services: Array.from(e.target.selectedOptions).map((o) => Number(o.value)),
                });
              }}
            >
              {providers.data.map((p) => (
                <option key={p.provider_id} value={p.provider_id}>
                  {p.provider_name}
                </option>
              ))}
              {settings.services
                ?.filter((id) => !providers.data.some((p) => p.provider_id === id))
                .map((id) => (
                  <option key={id} value={id}>
                    #{id} ({t("unknownValue")})
                  </option>
                ))}
            </select>
          ) : (
            <>
              <input
                value={settings.services?.join(",")}
                placeholder={t("services")}
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
              {providers.error && <small role="status">{providers.error.message}</small>}
            </>
          )}
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
      {[save.error, custom.error, rename.error, logout.error].filter(Boolean).map((e, i) => (
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
            <button
              onClick={() => {
                const name = window.prompt(t("name"), s.name);
                if (name?.trim()) rename.mutate({ ...s, name: name.trim() });
              }}
            >
              {t("rename")}
            </button>
            <small>
              {t(s.base as Key)} {s.used ? "🔒" : ""}
            </small>
            <button
              onClick={() => {
                remove.reset();
                setDeleting(s);
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
        <PortabilityPanel t={t} settings={user.Settings} />
      </section>
      <InstallPanel t={t} state={installation} />
      <button
        disabled={logout.isPending}
        onClick={() => {
          logout.mutate();
        }}
      >
        {t("logout")}
      </button>
    </>
  );
}

export function PortabilityPanel({ t, settings = {} }: { t: T; settings?: Settings }) {
  const client = useQueryClient();
  const [backed, setBacked] = useState(false);
  const [trustShared, setTrustShared] = useState(false);
  const [localError, setLocalError] = useState("");
  const beginPreview = usePreviewRequest();
  const [preview, setPreview] = useState<{
    id: string;
    additions: number;
    conflicts: string[];
    history_candidates: number;
    shared_metadata_requires_approval?: boolean;
    can_approve_shared_metadata?: boolean;
  } | null>(null);
  const [reading, setReading] = useState(false);
  const apply = useMutation({
    mutationFn: (id: string) =>
      api(`/restore/${id}/apply`, "POST", { trust_shared_metadata: trustShared }),
    onSuccess: async () => {
      setPreview(null);
      await client.invalidateQueries();
    },
  });
  async function read(file: File | undefined) {
    const request = beginPreview();
    setPreview(null);
    setTrustShared(false);
    setReading(false);
    setLocalError("");
    apply.reset();
    if (!file) return;
    if (file.size > 64 * 1024 * 1024) {
      setLocalError(t("restoreSize"));
      return;
    }
    setReading(true);
    try {
      const value: unknown = JSON.parse(await file.text());
      if (!request.current()) return;
      const result = await api<{
        id: string;
        additions: number;
        conflicts: string[];
        history_candidates: number;
        shared_metadata_requires_approval?: boolean;
        can_approve_shared_metadata?: boolean;
      }>("/restore/preview", "POST", value, request.signal);
      if (request.current()) setPreview(result);
    } catch (error) {
      if (request.current()) setLocalError(error instanceof Error ? error.message : t("error"));
    } finally {
      if (request.current()) setReading(false);
    }
  }
  return (
    <div className="panel">
      <p>{t("backupFirst")}</p>
      <a className="button" href="/api/export" download="watchledger-backup.json">
        ↓ {t("export")}
      </a>
      <label className="check-label">
        <input
          type="checkbox"
          checked={backed}
          onChange={(e) => {
            setBacked(e.target.checked);
          }}
        />
        {t("backupSaved")}
      </label>
      <label>
        {t("restore")}
        <input
          type="file"
          accept="application/json,.json"
          disabled={apply.isPending}
          onChange={(e) => {
            void read(e.target.files?.[0]);
          }}
        />
      </label>
      {reading && <p>{t("loading")}</p>}
      {[localError, apply.error?.message].filter(Boolean).map((e, i) => (
        <p role="alert" className="error" key={i}>
          {e}
        </p>
      ))}
      {preview && (
        <div className="preview">
          <h3>{t("previewReady")}</h3>
          <p>
            {t("additions")}: {preview.additions}
          </p>
          <p>
            {t("conflicts")}: {preview.conflicts.length}
          </p>
          <ul>
            {preview.conflicts.map((id) => (
              <li key={id}>{id}</li>
            ))}
          </ul>
          {preview.shared_metadata_requires_approval && (
            <div className="notice">
              <p>{t("sharedRestoreWarning")}</p>
              {preview.can_approve_shared_metadata ? (
                <label className="check-label">
                  <input
                    type="checkbox"
                    checked={trustShared}
                    disabled={apply.isPending}
                    onChange={(event) => setTrustShared(event.target.checked)}
                  />
                  {t("sharedRestoreApprove")}
                </label>
              ) : (
                <p>{t("sharedRestoreBlocked")}</p>
              )}
            </div>
          )}
          <button
            className="primary"
            disabled={
              !backed ||
              apply.isPending ||
              reading ||
              (!!preview.shared_metadata_requires_approval &&
                (!preview.can_approve_shared_metadata || !trustShared))
            }
            onClick={() => {
              if (preview) apply.mutate(preview.id);
            }}
          >
            {t("apply")}
          </button>
        </div>
      )}
      {apply.isSuccess && <p role="status">{t("restoreDone")}</p>}
      <SimklPanel t={t} backed={backed} />
      <CustomListImport t={t} backed={backed} settings={settings} />
    </div>
  );
}
