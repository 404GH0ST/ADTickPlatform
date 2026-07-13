import { NextResponse } from "next/server";

import { upstreamErrorResponse } from "@/lib/api-handler";
import { listTeamServices } from "@/lib/platform-api";

export async function GET() {
  try {
    const data = await listTeamServices();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(
      error,
      "team services request failed",
      "Team services unavailable",
    );
  }
}
