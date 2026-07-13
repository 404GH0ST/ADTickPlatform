import { NextResponse } from "next/server";

import { upstreamErrorResponse } from "@/lib/api-handler";
import { listAnnouncements } from "@/lib/platform-api";

export async function GET() {
  try {
    const data = await listAnnouncements();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(
      error,
      "announcements unavailable",
      "Announcements unavailable",
    );
  }
}
