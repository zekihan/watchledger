import type { T } from "./App";
export function PageControls({
  page,
  count,
  size,
  label,
  change,
  t,
}: {
  page: number;
  count: number;
  size: number;
  label: string;
  change: (page: number) => void;
  t: T;
}) {
  if (count <= size) return null;
  const last = Math.ceil(count / size) - 1;
  return (
    <nav className="toolbar" aria-label={label}>
      <button disabled={page === 0} onClick={() => change(page - 1)}>
        {t("previousPage")}
      </button>
      <span role="status">
        {page + 1} / {last + 1}
      </span>
      <button disabled={page >= last} onClick={() => change(page + 1)}>
        {t("nextPage")}
      </button>
    </nav>
  );
}
