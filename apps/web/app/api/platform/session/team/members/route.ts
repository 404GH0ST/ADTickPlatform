import { NextResponse } from "next/server";

import { upstreamErrorResponse } from "@/lib/api-handler";
import { listCurrentTeamMembers } from "@/lib/platform-api";

export async function GET() {
  try {
    const members = await listCurrentTeamMembers();
    return NextResponse.json(members);
  } catch (error) {
    return upstreamErrorResponse(
      error,
      "team members unavailable",
      "Team members unavailable",
    );
  }
}
