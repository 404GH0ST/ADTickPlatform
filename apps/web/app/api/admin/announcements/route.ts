import { NextResponse } from "next/server";

import {
  createAdminAnnouncement,
  listAdminAnnouncements,
} from "@/lib/admin-api";
import { problemResponse } from "@/lib/api-handler";

export async function GET() {
  try {
    const data = await listAdminAnnouncements();
    return NextResponse.json(data);
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "announcements unavailable";
    return problemResponse(502, "Upstream request failed", message);
  }
}

export async function POST(request: Request) {
  const body = (await request.json().catch(() => null)) as { body?: string } | null;
  const text = body?.body?.trim() ?? "";
  if (!text) {
    return problemResponse(400, "Invalid request", "body is required.");
  }
  try {
    const data = await createAdminAnnouncement(text);
    return NextResponse.json(data);
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "announcement create failed";
    return problemResponse(502, "Upstream request failed", message);
  }
}
