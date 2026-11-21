/**
 * Participant service lifecycle actions (restart / factory reset): cooldown
 * windows and the result messages that tell the two actions apart.
 *
 * The api-gateway enforces the cooldowns per team service and answers a
 * repeated click with 429 + Retry-After; these windows only drive the local
 * countdown after a successful action.
 */
export type ServiceAction = "restart" | "reset";

export const SERVICE_ACTION_COOLDOWN_SECONDS: Record<ServiceAction, number> = {
  restart: 30,
  reset: 90,
};

export const SERVICE_ACTION_LABEL: Record<ServiceAction, string> = {
  restart: "Restart",
  reset: "Factory reset",
};

export type ServiceNotice = {
  action: ServiceAction;
  variant: "success" | "warning" | "error";
  message: string;
};

/** Per-service action feedback: latest result notice and live cooldowns. */
export type ServiceActionFeedback = {
  notice: ServiceNotice | null;
  restartCooldown: number;
  resetCooldown: number;
};

export function cooldownKey(action: ServiceAction, serviceId: string): string {
  return `${action}:${serviceId}`;
}

export function remainingCooldownSeconds(
  until: number | undefined,
  now: number,
): number {
  if (!until || until <= now) {
    return 0;
  }
  return Math.ceil((until - now) / 1000);
}

/** Parse a Retry-After header (delta-seconds or HTTP date) into seconds. */
export function parseRetryAfterSeconds(
  value: string | undefined | null,
  now = Date.now(),
): number | null {
  const trimmed = value?.trim();
  if (!trimmed) {
    return null;
  }
  if (/^\d+$/.test(trimmed)) {
    const seconds = Number(trimmed);
    return seconds > 0 ? seconds : null;
  }
  const date = Date.parse(trimmed);
  if (Number.isNaN(date)) {
    return null;
  }
  const seconds = Math.ceil((date - now) / 1000);
  return seconds > 0 ? seconds : null;
}

function pluralSeconds(seconds: number): string {
  return `${seconds} ${seconds === 1 ? "second" : "seconds"}`;
}

export function cooldownMessage(action: ServiceAction, seconds: number): string {
  return `${SERVICE_ACTION_LABEL[action]} is on cooldown for this service. Wait ${pluralSeconds(seconds)} before trying again.`;
}

/** Shown when an action outlasts a normal wait, so a slow one doesn't look frozen. */
export function slowActionMessage(pendingAction: string): string | null {
  if (pendingAction.startsWith("restart:")) {
    return "Restart is still running. The result will appear here when it finishes.";
  }
  if (pendingAction.startsWith("reset:")) {
    return "Factory reset is still running. The result will appear here when it finishes.";
  }
  if (pendingAction.startsWith("ssh:")) {
    return "Still requesting root access. The credential will appear here when it's ready.";
  }
  return null;
}

export function restartSuccessMessage(): string {
  return `Restart complete: the service process was restarted with your current patches kept. The checker re-verifies it next round. Restart is available again in ${pluralSeconds(SERVICE_ACTION_COOLDOWN_SECONDS.restart)}.`;
}

export function factoryResetSuccessMessage(payload: {
  unlock_preserved?: boolean;
  penalty_points?: number;
}): string {
  const parts = [
    "Factory reset complete: the service was rebuilt from the organizer baseline image and all patches were discarded.",
  ];
  if (payload.penalty_points && payload.penalty_points > 0) {
    parts.push(
      `The stored flag was deleted: tick SLA forfeited and a ${payload.penalty_points}-point defense penalty applied.`,
    );
  }
  if (payload.unlock_preserved) {
    parts.push("SSH unlock is preserved.");
  }
  parts.push(
    `Factory reset is available again in ${pluralSeconds(SERVICE_ACTION_COOLDOWN_SECONDS.reset)}.`,
  );
  return parts.join(" ");
}
