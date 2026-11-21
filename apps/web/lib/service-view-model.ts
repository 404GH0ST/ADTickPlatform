import type { ServiceRow } from "@/lib/dashboard-types";
import { challengeCategoryLabel, describeConnection, type ServiceConnection } from "@/lib/challenge-metadata";
import type { ServiceActionFeedback } from "@/lib/service-actions";

/*
 * Display rules for one service card, shared by the stock services view and
 * any theme's services view, so every theme shows the same enabled/disabled
 * states, lock reasons and cooldowns.
 */

export type ServiceLockReason = "match_paused" | "match_not_started" | "deferred" | "maintenance";

export function resolveServiceLockReason(
  service: ServiceRow,
  matchState?: string | null,
): ServiceLockReason {
  const explicit = (service.lockReason || "").trim();
  if (
    explicit === "match_paused" ||
    explicit === "match_not_started" ||
    explicit === "deferred" ||
    explicit === "maintenance"
  ) {
    return explicit;
  }

  const match = (matchState || "").toLowerCase();
  const last = (service.lastEvent || "").toLowerCase();
  const reset = (service.resetCooldown || "").toLowerCase();
  const hint = (service.sshHint || "").toLowerCase();

  // Prefer concrete service text (always set by api-gateway on pause) so a
  // stale lock_reason or missing overview state cannot mislabel the card.
  if (
    match === "paused" ||
    reset.includes("paused") ||
    last.includes("paused") ||
    hint.includes("paused")
  ) {
    return "match_paused";
  }
  if (
    match === "not_started" ||
    reset.includes("not started") ||
    last.includes("waiting for match start")
  ) {
    return "match_not_started";
  }
  if (explicit === "deferred" || last.includes("next tick") || reset.includes("next tick")) {
    return "deferred";
  }
  return "maintenance";
}

export function serviceLockBadgeLabel(service: ServiceRow, matchState?: string | null): string {
  switch (resolveServiceLockReason(service, matchState)) {
    case "match_paused":
      return "Match paused";
    case "match_not_started":
      return "Match not started";
    case "deferred":
      return "Opens next tick";
    default:
      return "Under maintenance";
  }
}

export function serviceLockHelpText(service: ServiceRow, matchState?: string | null): string {
  switch (resolveServiceLockReason(service, matchState)) {
    case "match_paused":
      return "Actions and network access are suspended while the match is paused.";
    case "match_not_started":
      return "Service is locked until the organizer starts the match. Network access stays closed.";
    case "deferred":
      return "Challenge is warm-redeployed and will reopen on the next tick.";
    default:
      return "Actions unavailable while this challenge is under maintenance.";
  }
}

/** Live cooldown text when this session knows it; otherwise the API label. */
export function serviceCooldownLabel(service: ServiceRow, feedback: ServiceActionFeedback): string {
  const parts = [
    feedback.resetCooldown > 0 ? `factory reset cooldown: ${feedback.resetCooldown}s` : null,
    feedback.restartCooldown > 0 ? `restart cooldown: ${feedback.restartCooldown}s` : null,
  ].filter((part): part is string => part !== null);
  return parts.length > 0 ? parts.join(" · ") : service.resetCooldown;
}

export const SERVICE_UNLOCK_HINT =
  "Submit a correct flag from another team on this challenge to unlock SSH automatically.";

export type ServiceActionState = {
  disabled: boolean;
  pending: boolean;
};

export type ServiceViewModel = {
  service: ServiceRow;
  /** Present when the service is locked (maintenance, paused, not started, deferred). */
  lock: { reason: ServiceLockReason; label: string; help: string } | null;
  cooldownLabel: string;
  /** Shown when SSH is still locked; null once unlocked. */
  unlockHint: string | null;
  notice: ServiceActionFeedback["notice"];
  ssh: ServiceActionState & { label: string };
  restart: ServiceActionState & { cooldownSeconds: number };
  reset: ServiceActionState & { cooldownSeconds: number };
  /** Download link for the challenge source, or null when unavailable. */
  sourceHref: string | null;
  /** Category label ("Pwn", "Web exploitation", "Crypto", "Uncategorized"). */
  categoryLabel: string;
  /** How to connect (an nc command or a URL), or null when the endpoint is not host:port. */
  connection: ServiceConnection | null;
};

export function describeService(
  service: ServiceRow,
  {
    feedback,
    pendingAction,
    matchState,
  }: {
    feedback: ServiceActionFeedback;
    pendingAction: string | null;
    matchState?: string | null;
  },
): ServiceViewModel {
  const actionsDisabled = pendingAction !== null || service.maintenance;
  return {
    service,
    lock: service.maintenance
      ? {
          reason: resolveServiceLockReason(service, matchState),
          label: serviceLockBadgeLabel(service, matchState),
          help: serviceLockHelpText(service, matchState),
        }
      : null,
    cooldownLabel: serviceCooldownLabel(service, feedback),
    unlockHint: service.unlocked ? null : SERVICE_UNLOCK_HINT,
    notice: feedback.notice,
    ssh: {
      label: service.unlocked ? "SSH Access" : "SSH Locked",
      disabled: actionsDisabled || !service.unlocked,
      pending: pendingAction === `ssh:${service.id}`,
    },
    restart: {
      disabled: actionsDisabled,
      pending: pendingAction === `restart:${service.id}`,
      cooldownSeconds: feedback.restartCooldown,
    },
    reset: {
      disabled: actionsDisabled,
      pending: pendingAction === `reset:${service.id}`,
      cooldownSeconds: feedback.resetCooldown,
    },
    sourceHref:
      service.hasSourceDownload && !service.maintenance
        ? `/api/platform/challenges/${service.challengeId}/source`
        : null,
    categoryLabel: challengeCategoryLabel(service.category),
    connection: describeConnection(service.endpoint, service.protocol),
  };
}
