/**
 * The unsent flag text, kept for this tab. A reload, a dropped connection or
 * signing back in after a session ends must not cost a participant their flags.
 * A draft belongs to one player on one team: anyone else signing in on the same
 * tab never sees it. Storage can be blocked, in which case the field starts empty.
 */
const FLAG_DRAFT_KEY = "ad-platform-flag-draft";

type StoredDraft = { owner: string; value: string };

function parseDraft(raw: string | null): StoredDraft | null {
  if (!raw) return null;
  try {
    const draft = JSON.parse(raw) as Partial<StoredDraft>;
    return typeof draft.owner === "string" && typeof draft.value === "string"
      ? { owner: draft.owner, value: draft.value }
      : null;
  } catch {
    return null;
  }
}

/** The draft saved by `owner`. A draft from anyone else is deleted. */
export function readFlagDraft(owner: string): string {
  try {
    const raw = window.sessionStorage.getItem(FLAG_DRAFT_KEY);
    const draft = parseDraft(raw);
    if (draft?.owner === owner) return draft.value;
    if (raw !== null) window.sessionStorage.removeItem(FLAG_DRAFT_KEY);
  } catch {
    // Storage blocked: nothing was saved.
  }
  return "";
}

export function writeFlagDraft(owner: string, value: string) {
  try {
    if (value) {
      window.sessionStorage.setItem(FLAG_DRAFT_KEY, JSON.stringify({ owner, value }));
    } else {
      window.sessionStorage.removeItem(FLAG_DRAFT_KEY);
    }
  } catch {
    // Storage blocked: the text still lives in the field.
  }
}

export function clearFlagDraft() {
  try {
    window.sessionStorage.removeItem(FLAG_DRAFT_KEY);
  } catch {
    // Storage blocked: nothing was saved.
  }
}
