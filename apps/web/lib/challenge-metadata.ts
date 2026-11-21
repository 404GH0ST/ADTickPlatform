/**
 * Challenge category and connection protocol, shared by the organizer console,
 * the participant service cards and theme views. The gateway owns validation;
 * these helpers only parse what it sends and format it for people.
 */

export const CHALLENGE_CATEGORIES = ["pwn", "web", "crypto"] as const;

export type ChallengeCategory = (typeof CHALLENGE_CATEGORIES)[number];

export const CHALLENGE_PROTOCOLS = ["tcp", "http"] as const;

export type ChallengeProtocol = (typeof CHALLENGE_PROTOCOLS)[number];

export const CHALLENGE_CATEGORY_LABELS: Record<ChallengeCategory, string> = {
  pwn: "Pwn",
  web: "Web exploitation",
  crypto: "Crypto",
};

export const CHALLENGE_PROTOCOL_LABELS: Record<ChallengeProtocol, string> = {
  tcp: "TCP",
  http: "HTTP",
};

/** A known category, or "" when the challenge is uncategorized or the value is unknown. */
export function parseChallengeCategory(value: unknown): ChallengeCategory | "" {
  const normalized = typeof value === "string" ? value.trim().toLowerCase() : "";
  return (CHALLENGE_CATEGORIES as readonly string[]).includes(normalized)
    ? (normalized as ChallengeCategory)
    : "";
}

/** A known protocol; anything else (including a missing field) is TCP, the gateway default. */
export function parseChallengeProtocol(value: unknown): ChallengeProtocol {
  return typeof value === "string" && value.trim().toLowerCase() === "http" ? "http" : "tcp";
}

export function challengeCategoryLabel(category: ChallengeCategory | ""): string {
  return category ? CHALLENGE_CATEGORY_LABELS[category] : "Uncategorized";
}

export type ServiceConnection = {
  protocol: ChallengeProtocol;
  /** "TCP" or "HTTP". */
  label: string;
  host: string;
  port: number;
  /** What the card shows: a URL for HTTP, an `nc` command for TCP. */
  display: string;
  /** What the copy button copies (same as display). */
  copyText: string;
  /** Openable link, HTTP only. */
  href?: string;
};

/**
 * How to reach a service. endpoint is the gateway's `host:port`; null when it
 * cannot be parsed, so callers fall back to showing the raw endpoint.
 */
export function describeConnection(endpoint: string, protocol: ChallengeProtocol): ServiceConnection | null {
  const match = /^([^\s:/]+):(\d{1,5})$/.exec(endpoint.trim());
  if (!match) return null;
  const host = match[1];
  const port = Number(match[2]);
  if (!Number.isInteger(port) || port < 1 || port > 65535) return null;
  if (protocol === "http") {
    const url = `http://${host}:${port}/`;
    return { protocol, label: CHALLENGE_PROTOCOL_LABELS.http, host, port, display: url, copyText: url, href: url };
  }
  const command = `nc ${host} ${port}`;
  return { protocol, label: CHALLENGE_PROTOCOL_LABELS.tcp, host, port, display: command, copyText: command };
}
