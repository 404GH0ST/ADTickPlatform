import { NextResponse } from "next/server";

import { problemResponse } from "@/lib/api-handler";
import {
  changeParticipantPassword,
  PlatformAPIError,
} from "@/lib/platform-api";

export async function PUT(request: Request) {
  const body = (await request.json().catch(() => null)) as {
    current_password?: string;
    new_password?: string;
  } | null;
  const currentPassword = body?.current_password?.trim() ?? "";
  const newPassword = body?.new_password?.trim() ?? "";
  if (!currentPassword || !newPassword) {
    return problemResponse(
      400,
      "Invalid request",
      "current_password and new_password are required.",
    );
  }

  try {
    const data = await changeParticipantPassword({
      currentPassword,
      newPassword,
    });
    return NextResponse.json(data);
  } catch (error) {
    if (error instanceof PlatformAPIError) {
      return problemResponse(
        error.status,
        error.status >= 500
          ? "Password change unavailable"
          : "Password change failed",
        error.message,
      );
    }
    const message =
      error instanceof Error ? error.message : "password change failed";
    return problemResponse(502, "Password change unavailable", message);
  }
}
