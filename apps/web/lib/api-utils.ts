export type ProblemDetails = {
  type?: string;
  title: string;
  status: number;
  detail?: string;
  instance?: string;
};

/** Upstream API failure that preserves HTTP status for proxy routes. */
export class PlatformAPIError extends Error {
  status: number;
  retryAfter?: string;

  constructor(message: string, status: number, retryAfter?: string | null) {
    super(message);
    this.name = "PlatformAPIError";
    this.status = status;
    this.retryAfter = retryAfter?.trim() || undefined;
  }
}

export type UpstreamErrorDetails = {
  detail: string;
  retryAfter?: string;
  status: number;
};

function errorName(error: unknown) {
  return error && typeof error === "object" && "name" in error ? error.name : undefined;
}

export function upstreamErrorDetails(
  error: unknown,
  fallback: string,
): UpstreamErrorDetails {
  if (error instanceof PlatformAPIError) {
    return {
      detail: error.message,
      retryAfter: error.retryAfter,
      status: error.status,
    };
  }
  if (errorName(error) === "TimeoutError" || errorName(error) === "AbortError") {
    return {
      detail: "The platform API didn't respond in time. Try again in a moment.",
      status: 504,
    };
  }
  // Node's fetch reports every connection failure with this one message.
  if (error instanceof TypeError && error.message === "fetch failed") {
    return {
      detail: "The platform API can't be reached. Try again in a moment.",
      status: 502,
    };
  }
  return {
    detail: error instanceof Error ? error.message : fallback,
    status: 502,
  };
}

/** Message for a failed browser request. A browser fetch rejects with TypeError when the network fails. */
export function browserRequestErrorMessage(error: unknown, fallback: string) {
  if (error instanceof TypeError) {
    return "Can't reach the platform. Check your connection and try again.";
  }
  return error instanceof Error ? error.message : fallback;
}

export function buildQueryString(
  query: Record<string, string | number | undefined>,
) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === "" || Number.isNaN(value)) {
      continue;
    }
    params.set(key, String(value));
  }
  const encoded = params.toString();
  return encoded ? `?${encoded}` : "";
}

export function firstForwardedAddress(value: string | null): string | undefined {
  return value
    ?.split(",")
    .map((entry) => entry.trim())
    .find((entry) => entry !== "");
}

export async function parseTeamBody(request: Request) {
  const body = (await request.json().catch(() => null)) as {
    name?: string;
    contact_email?: string;
    institution?: unknown;
  } | null;
  const name = body?.name?.trim();
  const contactEmail = body?.contact_email?.trim();
  // undefined = field absent (keep the stored value on update).
  const institution =
    typeof body?.institution === "string" ? body.institution.trim() : undefined;
  return { name, contactEmail, institution };
}

export async function authenticatedFetch<T>(
  baseUrl: string,
  path: string,
  token: string | null,
  init?: RequestInit,
): Promise<T> {
  const requestHeaders = new Headers(init?.headers);
  if (token) {
    requestHeaders.set("Authorization", `Bearer ${token}`);
  }
  if (!requestHeaders.has("Content-Type")) {
    requestHeaders.set("Content-Type", "application/json");
  }
  const response = await fetch(`${baseUrl}${path}`, {
    ...init,
    headers: requestHeaders,
    cache: "no-store",
  });

  return processApiResponse<T>(response, path);
}

export async function processApiResponse<T>(
  response: Response,
  path: string,
): Promise<T> {
  if (!response.ok) {
    throw new PlatformAPIError(
      await parseApiError(response, path),
      response.status,
      response.headers.get("retry-after"),
    );
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

export async function parseApiError(
  response: Response,
  path: string,
): Promise<string> {
  const fallback = `API request to ${path} failed with status ${response.status}`;
  const contentType = response.headers.get("content-type") ?? "";
  const isJsonLike =
    contentType.includes("application/json") || contentType.includes("+json");
  if (!isJsonLike) {
    return fallback;
  }
  const payload = (await response.json().catch(() => null)) as
    | ProblemDetails
    | { message?: string }
    | null;
  if (!payload) {
    return fallback;
  }
  if ("message" in payload && payload.message) {
    return payload.message;
  }
  if ("detail" in payload && payload.detail) {
    return payload.detail;
  }
  if ("title" in payload && payload.title) {
    return payload.title;
  }
  return fallback;
}
