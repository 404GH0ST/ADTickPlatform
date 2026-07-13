import { upstreamErrorResponse } from "@/lib/api-handler";
import { downloadParticipantWireGuard } from "@/lib/platform-api";

function passthroughHeaders(headers: Headers): Headers {
  const next = new Headers();
  for (const name of [
    "content-type",
    "content-disposition",
    "content-length",
    "cache-control",
  ]) {
    const value = headers.get(name);
    if (value) {
      next.set(name, value);
    }
  }
  return next;
}

export async function GET() {
  try {
    const response = await downloadParticipantWireGuard();
    return new Response(response.body, {
      status: response.status,
      headers: passthroughHeaders(response.headers),
    });
  } catch (error) {
    return upstreamErrorResponse(
      error,
      "wireguard config download failed",
      "VPN config download failed",
    );
  }
}
