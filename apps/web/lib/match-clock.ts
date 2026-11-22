/** A jump bigger than network delay can explain means the device clock changed. */
const CLOCK_CHANGE_THRESHOLD_MS = 5_000;
const OVERDUE_REFRESH_BASE_MS = 3_500;
const OVERDUE_REFRESH_MAX_MS = 15_000;
const HEADER_REFRESH_BASE_MS = 15_000;
const HEADER_REFRESH_MAX_MS = 120_000;

/**
 * How far the server's clock is ahead of this device. Each sample reads low by
 * however long the page took to arrive, so the largest recent one is the most
 * accurate, unless the gap is big enough that the device clock itself moved.
 */
export function nextClockOffset(previous: number | null, sample: number): number {
  if (previous === null || Math.abs(sample - previous) > CLOCK_CHANGE_THRESHOLD_MS) {
    return sample;
  }
  return Math.max(previous, sample);
}

/** Wait before re-reading the page while its status read is still failing. */
export function headerRefreshDelayMs(attempt: number): number {
  return Math.min(HEADER_REFRESH_BASE_MS * 2 ** Math.max(0, attempt), HEADER_REFRESH_MAX_MS);
}

/** Wait before the next refresh while a tick is late, backing off if it stays late. */
export function overdueRefreshDelayMs(attempt: number): number {
  return Math.min(OVERDUE_REFRESH_BASE_MS * 2 ** Math.max(0, attempt), OVERDUE_REFRESH_MAX_MS);
}
