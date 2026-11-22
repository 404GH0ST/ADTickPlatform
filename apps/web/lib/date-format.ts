export function formatIndonesianDate(
  isoString: string | null | undefined,
): string {
  if (!isoString || isoString === "not applied yet") return isoString || "n/a";

  try {
    const date = new Date(isoString);
    if (isNaN(date.getTime())) {
      return isoString; // Fallback if invalid format
    }

    const formatter = new Intl.DateTimeFormat("id-ID", {
      timeZone: "Asia/Jakarta",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    });

    const parts = formatter.formatToParts(date);
    const getPart = (type: string) => parts.find((p) => p.type === type)?.value;

    return `${getPart("year")}-${getPart("month")}-${getPart("day")} ${getPart("hour")}:${getPart("minute")}:${getPart("second")} WIB`;
  } catch {
    return isoString;
  }
}

const WIB_OFFSET_MS = 7 * 60 * 60 * 1000;
const WIB_INPUT_VALUE = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/;

/** An instant as the `YYYY-MM-DDTHH:mm` a date and time field shows in WIB. WIB has no daylight saving. */
export function toWibInputValue(isoString?: string | null): string {
  if (!isoString) return "";
  const time = new Date(isoString).getTime();
  if (Number.isNaN(time)) return "";
  return new Date(time + WIB_OFFSET_MS).toISOString().slice(0, 16);
}

/** Reads a `YYYY-MM-DDTHH:mm` field value as WIB, whatever zone this device is in. */
export function fromWibInputValue(value: string): string | undefined {
  if (!WIB_INPUT_VALUE.test(value)) return undefined;
  const parsed = new Date(`${value}:00+07:00`);
  if (Number.isNaN(parsed.getTime())) return undefined;
  const iso = parsed.toISOString();
  // Some engines roll a date like February 31 into March instead of rejecting it.
  return toWibInputValue(iso) === value ? iso : undefined;
}

/** Just the clock time, for places where the date is obvious. */
export function formatWibTime(isoString: string | null | undefined): string {
  const formatted = formatIndonesianDate(isoString);
  return /^\d{4}-\d{2}-\d{2} /.test(formatted) ? formatted.slice(11) : formatted;
}

const ISO_TIMESTAMP = /\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})/g;

/** Rewrites ISO timestamps inside a server-written message to the app's WIB format. */
export function formatTimestampsInText(text: string): string {
  return text.replace(ISO_TIMESTAMP, (timestamp) => formatIndonesianDate(timestamp));
}
