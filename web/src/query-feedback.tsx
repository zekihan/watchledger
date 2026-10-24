import type { T } from "./App";

export function QueryFeedback({
  query,
  t,
}: {
  query: {
    isPending: boolean;
    isFetching: boolean;
    error: Error | null;
    refetch: () => Promise<unknown>;
  };
  t: T;
}) {
  if (query.error)
    return (
      <div className="error" role="alert">
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
    );
  if (query.isPending) return <p role="status">{t("loading")}</p>;
  return null;
}
