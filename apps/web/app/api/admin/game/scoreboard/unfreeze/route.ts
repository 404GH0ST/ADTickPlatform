import { NextResponse } from "next/server";

import { problemResponse } from "@/lib/api-handler";
import { clearAdminScoreboardFreeze } from "@/lib/admin-api";

export async function POST() {
  try {
    const data = await clearAdminScoreboardFreeze();
    return NextResponse.json(data);
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "freeze clear failed";
    return problemResponse(502, "Upstream request failed", message);
  }
}
