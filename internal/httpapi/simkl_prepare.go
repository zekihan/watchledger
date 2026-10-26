package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/catalog"
	"github.com/zekihan/watchledger/internal/simkl"
)

type simklQuery interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type simklEpisodeMatch struct {
	SourceKey string `json:"source_key"`
	EpisodeID string `json:"episode_id"`
	Season    int    `json:"season"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
}
type simklCandidate struct {
	simkl.Item
	Target            string              `json:"target"`
	Action            string              `json:"action"`
	Matches           []simklEpisodeMatch `json:"matches"`
	EpisodeExceptions int                 `json:"episode_exceptions"`
}
type simklSummary struct {
	ID                   string           `json:"id,omitempty"`
	Additions            int              `json:"additions"`
	Conflicts            int              `json:"conflicts"`
	Unresolved           int              `json:"unresolved"`
	PendingCatalog       int              `json:"pending_catalog"`
	EpisodeMatches       int              `json:"episode_matches"`
	EpisodeExceptions    int              `json:"episode_exceptions"`
	EpisodeMatchDigest   string           `json:"episode_match_digest"`
	PreparationTotal     int              `json:"preparation_total"`
	Items                []simklCandidate `json:"items"`
	Warnings             []string         `json:"warnings"`
	BackupRequired       bool             `json:"backup_required"`
	ExistingAppEditsWin  bool             `json:"existing_app_edits_win"`
	CustomListsSupported bool             `json:"custom_lists_supported"`
}

func simklTargets(backup simkl.Backup) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, item := range backup.Items {
		target := item.Target()
		if target != "" && len(item.Issues) == 0 && !seen[target] {
			seen[target] = true
			out = append(out, target)
		}
	}
	return out
}
func (a *API) simklSummary(ctx context.Context, q simklQuery, uid string, backup simkl.Backup) (simklSummary, error) {
	out := simklSummary{Items: []simklCandidate{}, Warnings: backup.Warnings, BackupRequired: true, ExistingAppEditsWin: true, PreparationTotal: len(simklTasks(backup))}
	targets := simklTargets(backup)
	cached, existing := map[string]bool{}, map[string]bool{}
	rows, err := q.Query(ctx, "SELECT id FROM catalog WHERE id=ANY($1::text[])", targets)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		cached[id] = true
	}
	rows.Close()
	if rows.Err() != nil {
		return out, rows.Err()
	}
	rows, err = q.Query(ctx, "SELECT catalog_id FROM library WHERE user_id=$1 AND catalog_id=ANY($2::text[])", uid, targets)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		existing[id] = true
	}
	rows.Close()
	if rows.Err() != nil {
		return out, rows.Err()
	}
	type episode struct {
		id, target, title, kind, source string
		season, number                  int
		retired                         bool
	}
	episodes := map[string]episode{}
	numbered := map[string][]episode{}
	rows, err = q.Query(ctx, "SELECT id,catalog_id,season,number,retired,kind,source,coalesce(metadata->>'name','') FROM episodes WHERE catalog_id=ANY($1::text[])", targets)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var ep episode
		if err = rows.Scan(&ep.id, &ep.target, &ep.season, &ep.number, &ep.retired, &ep.kind, &ep.source, &ep.title); err != nil {
			rows.Close()
			return out, err
		}
		episodes[ep.id] = ep
		if !ep.retired && ep.source == "tmdb" && ep.kind == "regular" && ep.season > 0 {
			key := fmt.Sprintf("%s/%d/%d", ep.target, ep.season, ep.number)
			numbered[key] = append(numbered[key], ep)
		}
	}
	rows.Close()
	if rows.Err() != nil {
		return out, rows.Err()
	}
	known := map[string]string{}
	sourceKeys := []string{}
	for _, item := range backup.Items {
		for _, episode := range item.Episodes {
			sourceKeys = append(sourceKeys, episode.Key)
		}
	}
	rows, err = q.Query(ctx, "SELECT source_key,episode_id FROM simkl_episode_links WHERE user_id=$1 AND source_key=ANY($2::text[])", uid, sourceKeys)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var source, id string
		if err = rows.Scan(&source, &id); err != nil {
			rows.Close()
			return out, err
		}
		known[source] = id
	}
	rows.Close()
	if rows.Err() != nil {
		return out, rows.Err()
	}
	multiplicity := map[string]int{}
	for _, item := range backup.Items {
		if len(item.Issues) == 0 && item.Target() != "" {
			multiplicity[item.Target()]++
		}
	}
	for _, item := range backup.Items {
		candidate := simklCandidate{Item: item, Target: item.Target(), Action: "review", Matches: []simklEpisodeMatch{}}
		if len(item.Issues) == 0 && candidate.Target != "" {
			if cached[candidate.Target] {
				if existing[candidate.Target] {
					candidate.Action = "preserve_existing"
					out.Conflicts++
				} else {
					candidate.Action = "add"
					out.Additions++
				}
			} else {
				candidate.Warnings = append(append([]string{}, item.Warnings...), "catalog_not_cached")
				out.PendingCatalog++
			}
			identityConflict := false
			for _, source := range item.Episodes {
				if id := known[source.Key]; id != "" {
					ep, ok := episodes[id]
					if !ok || ep.target != candidate.Target {
						identityConflict = true
					}
				}
			}
			used := map[string]bool{}
			for _, source := range item.Episodes {
				var ep episode
				ok := false
				if id := known[source.Key]; id != "" {
					ep, ok = episodes[id]
					ok = ok && ep.target == candidate.Target
				} else if !identityConflict && item.Kind == "shows" && multiplicity[candidate.Target] == 1 && strings.HasPrefix(candidate.Target, "tmdb:tv:") && source.Season > 0 {
					matches := numbered[fmt.Sprintf("%s/%d/%d", candidate.Target, source.Season, source.Number)]
					if len(matches) == 1 {
						ep = matches[0]
						ok = true
					}
				}
				if ok && !used[ep.id] {
					used[ep.id] = true
					candidate.Matches = append(candidate.Matches, simklEpisodeMatch{SourceKey: source.Key, EpisodeID: ep.id, Season: source.Season, Number: source.Number, Title: ep.title})
				}
			}
		}
		candidate.EpisodeExceptions = len(item.Episodes) - len(candidate.Matches)
		if len(candidate.Matches) > 0 && candidate.EpisodeExceptions == 0 {
			warnings := []string{}
			for _, warning := range candidate.Warnings {
				if warning != "episode_identity_review_required" {
					warnings = append(warnings, warning)
				}
			}
			candidate.Warnings = warnings
		}
		out.EpisodeMatches += len(candidate.Matches)
		out.EpisodeExceptions += candidate.EpisodeExceptions
		if candidate.Action == "review" || candidate.EpisodeExceptions > 0 {
			out.Unresolved++
		}
		out.Items = append(out.Items, candidate)
	}
	// The version binds approval to this account's source and exact episode identities.
	raw, err := json.Marshal(struct {
		User  string
		Items []simklCandidate
	}{uid, out.Items})
	if err != nil {
		return out, err
	}
	digest := sha256.Sum256(raw)
	out.EpisodeMatchDigest = hex.EncodeToString(digest[:])
	return out, nil
}
func (a *API) savedSimklSource(ctx context.Context, uid, id string) (simkl.Backup, error) {
	var raw []byte
	if err := a.DB.QueryRow(ctx, "SELECT archive FROM imports WHERE user_id=$1 AND id=$2 AND kind='simkl'", uid, id).Scan(&raw); err != nil {
		return simkl.Backup{}, err
	}
	return simkl.Parse(raw)
}
func (a *API) savedSimklPreview(w http.ResponseWriter, r *http.Request) {
	uid := auth.Current(r.Context()).ID
	backup, err := a.savedSimklSource(r.Context(), uid, r.PathValue("id"))
	if err != nil {
		http.Error(w, "preview not found", http.StatusNotFound)
		return
	}
	summary, err := a.simklSummary(r.Context(), a.DB, uid, backup)
	if err != nil {
		http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
		return
	}
	summary.ID = r.PathValue("id")
	if r.Method == http.MethodPost {
		preview, e := json.Marshal(summary)
		if e == nil {
			_, e = a.DB.Exec(r.Context(), "UPDATE imports SET state='preview',preview=$3 WHERE user_id=$1 AND id=$2 AND kind='simkl'", uid, summary.ID, preview)
		}
		if e != nil {
			http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
			return
		}
	}
	writeJSON(w, summary)
}

type simklTask struct {
	Target string
	Season *int
}

func simklTasks(backup simkl.Backup) []simklTask {
	tasks := []simklTask{}
	seen := map[string]bool{}
	for _, item := range backup.Items {
		target := item.Target()
		if target == "" || len(item.Issues) > 0 {
			continue
		}
		if !seen[target] {
			seen[target] = true
			tasks = append(tasks, simklTask{Target: target})
		}
		if item.Kind != "shows" || !strings.HasPrefix(target, "tmdb:tv:") {
			continue
		}
		for _, ep := range item.Episodes {
			if ep.Season < 1 {
				continue
			}
			key := fmt.Sprintf("%s/season/%d", target, ep.Season)
			if !seen[key] {
				seen[key] = true
				season := ep.Season
				tasks = append(tasks, simklTask{Target: target, Season: &season})
			}
		}
	}
	return tasks
}

var simklPreparationGate = make(chan struct{}, 1)
var simklPreparationRetryAt time.Time

func (a *API) prepareSimkl(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Offset int `json:"offset"`
	}
	if !decode(w, r, &in) {
		return
	}
	uid := auth.Current(r.Context()).ID
	backup, err := a.savedSimklSource(r.Context(), uid, r.PathValue("id"))
	if err != nil {
		http.Error(w, "preview not found", http.StatusNotFound)
		return
	}
	tasks := simklTasks(backup)
	if in.Offset < 0 || in.Offset > len(tasks) {
		http.Error(w, "invalid preparation position", http.StatusBadRequest)
		return
	}
	select {
	case simklPreparationGate <- struct{}{}:
		defer func() { <-simklPreparationGate }()
	default:
		http.Error(w, "another import is preparing; retry shortly", http.StatusTooManyRequests)
		return
	}
	var settings struct {
		Language string `json:"metadata_language"`
		Style    string `json:"anime_title"`
	}
	_ = json.Unmarshal(auth.Current(r.Context()).Settings, &settings)
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	next, loaded, failed := in.Offset, 0, 0
	paused := time.Now().Before(simklPreparationRetryAt)
	for !paused && next < len(tasks) && loaded < 6 {
		task := tasks[next]
		var cached bool
		if task.Season == nil {
			err = a.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM catalog WHERE id=$1)", task.Target).Scan(&cached)
		} else {
			err = a.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM season_cache WHERE catalog_id=$1 AND season=$2 AND language=$3)", task.Target, *task.Season, settings.Language).Scan(&cached)
		}
		if err != nil {
			paused = true
			break
		}
		if cached {
			next++
			continue
		}
		parts := strings.Split(task.Target, ":")
		if parts[0] == "anidb" {
			if a.Anime == nil || a.Anime.Provider == nil || !a.Anime.Provider.Available() {
				failed++
				next++
				continue
			}
			_, err = a.Anime.Detail(ctx, parts[2], settings.Language, settings.Style)
		} else {
			if a.Catalog == nil || a.Catalog.Provider == nil || a.Catalog.Provider.Token == "" {
				paused = true
				break
			}
			if task.Season == nil {
				_, err = a.Catalog.Detail(ctx, parts[1], parts[2], settings.Language, false)
			} else {
				_, err = a.Catalog.Season(ctx, parts[2], *task.Season, settings.Language)
			}
		}
		loaded++
		if err != nil {
			var status *catalog.StatusError
			if errors.As(err, &status) && status.Code == http.StatusTooManyRequests {
				simklPreparationRetryAt = time.Now().Add(status.RetryAfter)
			}
			// A deleted source ID is an individual exception; outages pause the batch.
			if errors.As(err, &status) && status.Code == http.StatusNotFound {
				failed++
				next++
				continue
			}
			paused = true
			break
		}
		next++
		if next < len(tasks) {
			select {
			case <-ctx.Done():
				paused = true
			case <-time.After(200 * time.Millisecond):
			}
		}
		if paused {
			break
		}
	}
	response := map[string]any{"next": next, "total": len(tasks), "failed": failed, "paused": paused}
	if delay := time.Until(simklPreparationRetryAt); delay > 0 {
		response["retry_after"] = int(delay.Seconds()) + 1
	}
	if next == len(tasks) || paused {
		summary, e := a.simklSummary(r.Context(), a.DB, uid, backup)
		if e != nil {
			http.Error(w, "preview unavailable", http.StatusServiceUnavailable)
			return
		}
		summary.ID = r.PathValue("id")
		response["preview"] = summary
	}
	writeJSON(w, response)
}
func applySIMKLProposals(ctx context.Context, tx pgx.Tx, uid, importID string, item simkl.Item, matches []simklEpisodeMatch) (int, bool, error) {
	sources := map[string]simkl.Episode{}
	for _, ep := range item.Episodes {
		sources[ep.Key] = ep
	}
	added := 0
	conflicts := false
	seen := map[string]bool{}
	for _, match := range matches {
		source, ok := sources[match.SourceKey]
		if !ok || seen[match.EpisodeID] {
			return 0, false, errors.New("invalid episode proposal")
		}
		seen[match.EpisodeID] = true
		var owner, previous string
		var retired bool
		if err := tx.QueryRow(ctx, "SELECT catalog_id,retired FROM episodes WHERE id=$1", match.EpisodeID).Scan(&owner, &retired); err != nil {
			return 0, false, err
		}
		if err := tx.QueryRow(ctx, "SELECT coalesce((SELECT episode_id FROM simkl_episode_links WHERE user_id=$1 AND source_key=$2),'')", uid, match.SourceKey).Scan(&previous); err != nil {
			return 0, false, err
		}
		if owner != item.Target() || (retired && previous == "") || (previous != "" && previous != match.EpisodeID) {
			return 0, false, errors.New("episode identity changed")
		}
		if _, err := tx.Exec(ctx, "INSERT INTO simkl_episode_links(user_id,source_key,catalog_id,episode_id) VALUES($1,$2,$3,$4) ON CONFLICT(user_id,source_key) DO NOTHING", uid, match.SourceKey, item.Target(), match.EpisodeID); err != nil {
			return 0, false, err
		}
		provenance, err := json.Marshal(map[string]string{"source": "simkl", "source_key": match.SourceKey, "import_id": importID, "mapping": "user_confirmed"})
		if err != nil {
			return 0, false, err
		}
		var day any
		if source.Date.Day != "" {
			day = source.Date.Day
		}
		tag, err := tx.Exec(ctx, `INSERT INTO history(user_id,catalog_id,episode_id,watched_date,watched_time,provenance) SELECT $1,$2,$3,$4,$5,$6 WHERE NOT EXISTS(SELECT 1 FROM history_tombstones WHERE user_id=$1 AND catalog_id=$2 AND episode_key=$3) ON CONFLICT(user_id,catalog_id,episode_id) DO NOTHING`, uid, item.Target(), match.EpisodeID, day, source.Date.Timestamp, provenance)
		if err != nil {
			return 0, false, err
		}
		added += int(tag.RowsAffected())
		if tag.RowsAffected() == 0 {
			conflict, err := simklWatchConflict(ctx, tx, uid, item.Target(), match.EpisodeID, day, source.Date.Timestamp)
			if err != nil {
				return 0, false, err
			}
			conflicts = conflicts || conflict
		}
	}
	return added, conflicts, nil
}

func simklWatchConflict(ctx context.Context, q simklQuery, uid, target, episode string, day any, timestamp *time.Time) (bool, error) {
	var conflict bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM history_tombstones WHERE user_id=$1 AND catalog_id=$2 AND episode_key=$3) OR EXISTS(SELECT 1 FROM history WHERE user_id=$1 AND catalog_id=$2 AND coalesce(episode_id,'')=$3 AND (($4::date IS NOT NULL AND coalesce(watched_date,(watched_time AT TIME ZONE 'UTC')::date) IS DISTINCT FROM $4::date) OR ($5::timestamptz IS NOT NULL AND watched_time IS DISTINCT FROM $5::timestamptz)))`, uid, target, episode, day, timestamp).Scan(&conflict)
	return conflict, err
}
