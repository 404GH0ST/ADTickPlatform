export type ProblemDetails = {
  type?: string;
  title: string;
  status: number;
  detail?: string;
  instance?: string;
};

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

export async function parseTeamBody(request: Request) {
  const body = (await request.json().catch(() => null)) as { name?: string; contact_email?: string } | null;
  const name = body?.name?.trim();
  const contactEmail = body?.contact_email?.trim();
  return { name, contactEmail };
}

export async function authenticatedFetch<T>(
  baseUrl: string,
  path: string,
  token: string | null,
  init?: RequestInit,
): Promise<T> {
  const response = await fetch(`${baseUrl}${path}`, {
    ...init,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      "Content-Type": "application/json",
      ...init?.headers,
    },
    cache: "no-store",
  });

  return processApiResponse<T>(response, path);
}

export async function processApiResponse<T>(
  response: Response,
  path: string,
): Promise<T> {
  if (!response.ok) {
    throw new Error(await parseApiError(response, path));
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
