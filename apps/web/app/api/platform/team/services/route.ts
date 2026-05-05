import { NextResponse } from "next/server";

import { listTeamServices } from "@/lib/platform-api";

function problemResponse(status: number, title: string, detail: string) {
  return NextResponse.json(
    { title, status, detail },
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

export async function GET() {
  try {
    const data = await listTeamServices();
    return NextResponse.json(data);
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "team services request failed";
    const status =
      message === "participant session is not authenticated." ? 403 : 502;
    return problemResponse(
      status,
      status === 403 ? "Authentication required" : "Team services unavailable",
      message,
    );
  }
}
