import { forwardUpstreamResponse, upstreamErrorResponse } from "@/lib/api-handler";
import { adminProxyFetch } from "@/lib/admin-api";

async function proxyJob(method: "GET" | "POST") {
  try {
    const response = await adminProxyFetch("/api/v2/admin/deployments/reconcile/job", { method });
    return forwardUpstreamResponse(response);
  } catch (error) {
    return upstreamErrorResponse(error, "deployment reconcile unavailable", "Upstream request failed");
  }
}

export function GET() {
  return proxyJob("GET");
}

export function POST() {
  return proxyJob("POST");
}
