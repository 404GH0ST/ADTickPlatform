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
  } catch (e) {
    return isoString;
  }
}
