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

/**
 * Stops the match, tolerating a transport timeout on the stop call itself.
 *
 * A stop waits for any running tick and then runs the final retention sweep,
 * which can outlast Node fetch's 300s response-header timeout. The gateway and
 * game-core finish the stop anyway, so when the call dies without an HTTP
 * answer we poll the match state instead of reporting a failure for a match
 * that is about to end. An actual error response from the API is rethrown.
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
    if (error instanceof PlatformAPIError) {
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
