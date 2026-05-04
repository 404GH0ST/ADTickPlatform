export type SuccessEnvelope<T> = {
  status: "success";
  data: T;
};

export type ErrorEnvelope = {
  status: "failed" | "forbidden" | "too many request";
  message: string;
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

async function processApiResponse<T>(
  response: Response,
  path: string,
): Promise<T> {
  const payload = (await response.json()) as
    | SuccessEnvelope<T>
    | ErrorEnvelope;
  if (!response.ok || payload.status !== "success") {
    throw new Error(
      "message" in payload
        ? payload.message
        : `API request to ${path} failed with status ${response.status}`,
    );
  }

  return payload.data;
}
