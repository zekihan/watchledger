import type { T } from "./App";
import type { Key } from "./i18n";
const messages: Record<string, Key> = {
  add: "importMessageAdd",
  review: "importMessageReview",
  preserve_existing: "importMessagePreserveExisting",
  preview: "importMessagePreview",
  applied: "importMessageApplied",
  invalid_item: "importMessageInvalidItem",
  duplicate_source_identity: "importMessageDuplicateSourceIdentity",
  missing_simkl_identity: "importMessageMissingSimklIdentity",
  missing_title: "importMessageMissingTitle",
  unknown_status: "importMessageUnknownStatus",
  invalid_rating: "importMessageInvalidRating",
  source_added_date_retained_in_archive: "importMessageSourceAddedDateRetainedInArchive",
  rewatch_retained_in_archive: "importMessageRewatchRetainedInArchive",
  invalid_movie_watch_date: "importMessageInvalidMovieWatchDate",
  invalid_episode_number: "importMessageInvalidEpisodeNumber",
  duplicate_episode_record: "importMessageDuplicateEpisodeRecord",
  invalid_episode_watch_date: "importMessageInvalidEpisodeWatchDate",
  summary_count_differs_from_explicit_history:
    "importMessageSummaryCountDiffersFromExplicitHistory",
  completed_without_episode_records: "importMessageCompletedWithoutEpisodeRecords",
  episode_identity_review_required: "importMessageEpisodeIdentityReviewRequired",
  custom_lists_absent_from_backup: "importMessageCustomListsAbsentFromBackup",
  catalog_not_cached: "importMessageCatalogNotCached",
  unresolved_catalog_identity: "importMessageUnresolvedCatalogIdentity",
  existing_app_library_preserved: "importMessageExistingAppLibraryPreserved",
  existing_app_watch_decision_preserved: "importMessageExistingAppWatchDecisionPreserved",
  existing_app_list_preserved: "importMessageExistingAppListPreserved",
  existing_app_list_deletion_preserved: "importMessageExistingAppListDeletionPreserved",
  conflicting_source_identity: "importMessageConflictingSourceIdentity",
  source_identity_unresolved: "importMessageSourceIdentityUnresolved",
  add_source_title_to_private_library_first: "importMessageAddSourceTitleToPrivateLibraryFirst",
  multiple_source_members_share_target: "importMessageMultipleSourceMembersShareTarget",
  simkl_source_review: "importMessageSimklSourceReview",
};
export function importMessage(code: string, t: T): string {
  const key = messages[code];
  if (key) return t(key);
  if (code.startsWith("unsupported_section:"))
    return `${t("importUnsupportedSection")}: ${code.slice("unsupported_section:".length)}`;
  return `${t("importUnrecognizedMessage")}: ${code}`;
}
