import { NextResponse } from "next/server";

import { deleteAdminAnnouncement } from "@/lib/admin-api";
import { problemResponse, upstreamErrorResponse } from "@/lib/api-handler";

type Params = { params: Promise<{ announcementId: string }> };

export async function DELETE(_request: Request, { params }: Params) {
  const { announcementId } = await params;
  const id = Number(announcementId);
  if (!Number.isFinite(id) || id <= 0) {
    return problemResponse(400, "Invalid request", "announcement_id is invalid.");
  }
  try {
    const data = await deleteAdminAnnouncement(id);
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, "announcement delete failed");
  }
}
