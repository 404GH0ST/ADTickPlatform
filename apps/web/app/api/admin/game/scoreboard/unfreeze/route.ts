import { NextResponse } from "next/server";

import { upstreamErrorResponse } from "@/lib/api-handler";
import { clearAdminScoreboardFreeze } from "@/lib/admin-api";

export async function POST() {
  try {
    const data = await clearAdminScoreboardFreeze();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, "freeze clear failed");
  }
}
