import { NextResponse } from "next/server";

import { bulkImportAdminTeams } from "@/lib/admin-api";
import { problemResponse, upstreamErrorResponse } from "@/lib/api-handler";

export async function POST(request: Request) {
  const body = (await request.json().catch(() => null)) as {
    teams?: Array<{
      name: string;
      contact_email: string;
      players?: Array<{
        display_name: string;
        email: string;
        password: string;
        role?: string;
      }>;
    }>;
  } | null;
  if (!Array.isArray(body?.teams) || body.teams.length === 0) {
    return problemResponse(400, "Invalid request", "teams array is required.");
  }
  try {
    const { status, body: result } = await bulkImportAdminTeams(body.teams);
    return NextResponse.json(result, { status });
  } catch (error) {
    return upstreamErrorResponse(error, "bulk import failed");
  }
}
