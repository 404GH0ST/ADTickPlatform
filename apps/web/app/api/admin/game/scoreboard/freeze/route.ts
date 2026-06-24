import { NextRequest, NextResponse } from "next/server";

import { problemResponse } from "@/lib/api-handler";
import {
  getAdminScoreboardFreeze,
  setAdminScoreboardFreeze,
} from "@/lib/admin-api";

export async function GET() {
  try {
    const data = await getAdminScoreboardFreeze();
    return NextResponse.json(data);
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "freeze status fetch failed";
    return problemResponse(502, "Upstream request failed", message);
  }
}

export async function POST(request: NextRequest) {
  let body: { freeze_at?: unknown; unfreeze_at?: unknown };
  try {
    body = await request.json();
  } catch {
    return problemResponse(400, "Invalid request", "freeze request is invalid.");
  }
  if (typeof body.freeze_at !== "string" || body.freeze_at.trim() === "") {
    return problemResponse(400, "Invalid request", "freeze_at is required.");
  }
  const unfreezeAt =
    typeof body.unfreeze_at === "string" && body.unfreeze_at.trim() !== ""
      ? body.unfreeze_at
      : undefined;
  try {
    const data = await setAdminScoreboardFreeze({
      freeze_at: body.freeze_at,
      unfreeze_at: unfreezeAt,
    });
    return NextResponse.json(data);
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "freeze set failed";
    return problemResponse(502, "Upstream request failed", message);
  }
}
