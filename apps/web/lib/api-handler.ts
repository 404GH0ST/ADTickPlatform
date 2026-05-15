import { NextResponse } from "next/server";

export async function handleAdminIdRoute<T>(
  context: { params: Promise<Record<string, string>> },
  idParamName: string,
  idName: string,
  actionName: string,
  actionFn: (id: number) => Promise<T>,
) {
  const params = await context.params;
  const idStr = params[idParamName];
  if (!idStr) {
    return problemResponse(400, "Invalid request", `${idName} is invalid.`);
  }

  const id = Number(idStr);
  if (!Number.isInteger(id) || id <= 0) {
    return problemResponse(400, "Invalid request", `${idName} is invalid.`);
  }

  try {
    const data = await actionFn(id);
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : `${actionName} failed`;
    return problemResponse(502, "Upstream request failed", message);
  }
}

export async function handleParticipantIdRoute<T>(
  context: { params: Promise<Record<string, string>> },
  idParamName: string,
  idName: string,
  actionName: string,
  actionFn: (id: number) => Promise<T>,
) {
  const params = await context.params;
  const idStr = params[idParamName];
  if (!idStr) {
    return problemResponse(400, "Invalid request", `${idName} is invalid.`);
  }

  const id = Number(idStr);
  if (!Number.isInteger(id) || id <= 0) {
    return problemResponse(400, "Invalid request", `${idName} is invalid.`);
  }

  try {
    const data = await actionFn(id);
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : `${actionName} failed`;
    const status =
      message === "participant session is not authenticated." ? 403 : 502;
    return problemResponse(
      status,
      status === 403 ? "Authentication required" : "Upstream request failed",
      message,
    );
  }
}

export function problemResponse(status: number, title: string, detail: string) {
  return NextResponse.json(
    { title, status, detail },
    {
      status,
      headers: {
        "Content-Type": "application/problem+json",
      },
    },
  );
}
