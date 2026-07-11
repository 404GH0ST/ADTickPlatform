import { NextResponse } from "next/server";

import { problemResponse } from "@/lib/api-handler";
import { listAnnouncements } from "@/lib/platform-api";

export async function GET() {
  try {
    const data = await listAnnouncements();
    return NextResponse.json(data);
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "announcements unavailable";
    return problemResponse(502, "Announcements unavailable", message);
  }
}
