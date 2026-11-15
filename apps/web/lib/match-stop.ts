import { PlatformAPIError } from "@/lib/api-utils";

type MatchState = { state: string };

export type MatchStopWaitOptions = {
  pollIntervalMs?: number;
  maxWaitMs?: number;
  now?: () => number;
  sleep?: (ms: number) => Promise<void>;
};

const defaultSleep = (ms: number) =>
  new Promise<void>((resolve) => setTimeout(resolve, ms));

// Failures to open the connection at all: the stop request was never sent, so
// there is no stop in progress to wait for.
const connectFailureCodes = new Set([
  "ECONNREFUSED",
  "ENOTFOUND",
  "EAI_AGAIN",
  "EHOSTUNREACH",
  "ENETUNREACH",
  "UND_ERR_CONNECT_TIMEOUT",
]);

function errorCode(value: unknown): string | undefined {
  if (value && typeof value === "object" && "code" in value) {
    const code = (value as { code?: unknown }).code;
    return typeof code === "string" ? code : undefined;
  }
  return undefined;
}

/** Walks the cause chain (and AggregateError members) that undici wraps. */
export function isConnectFailure(error: unknown, depth = 0): boolean {
  if (!error || typeof error !== "object" || depth > 5) {
    return false;
  }
  const code = errorCode(error);
  if (code && connectFailureCodes.has(code)) {
    return true;
  }
  if (error instanceof AggregateError) {
    return error.errors.some((inner) => isConnectFailure(inner, depth + 1));
  }
  return isConnectFailure((error as { cause?: unknown }).cause, depth + 1);
}

/**
 * Stops the match, tolerating a transport timeout on the stop call itself.
 *
 * A stop waits for any running tick and then runs the final retention sweep,
 * which can outlast Node fetch's 300s response-header timeout. The gateway and
 * game-core finish the stop anyway, so when the call dies without an HTTP
 * answer we poll the match state instead of reporting a failure for a match
 * that is about to end. An actual error response from the API, or a failure to
 * connect at all (the stop never left this process), is rethrown at once.
 */
export async function stopMatchTolerantOfTimeouts<T extends MatchState>(
  stop: () => Promise<T>,
  readStatus: () => Promise<T>,
  {
    pollIntervalMs = 3_000,
    maxWaitMs = 15 * 60_000,
    now = Date.now,
    sleep = defaultSleep,
  }: MatchStopWaitOptions = {},
): Promise<T> {
  try {
    return await stop();
  } catch (error) {
    if (error instanceof PlatformAPIError || isConnectFailure(error)) {
      throw error;
    }
    const deadline = now() + maxWaitMs;
    while (now() < deadline) {
      try {
        const status = await readStatus();
        if (status.state === "finished") {
          return status;
        }
      } catch {
        // The gateway may be briefly unavailable; keep waiting until the deadline.
      }
      await sleep(pollIntervalMs);
    }
    throw error;
  }
}
