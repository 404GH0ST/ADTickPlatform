import { forwardUpstreamResponse, upstreamErrorResponse } from '@/lib/api-handler';
import { adminProxyFetch } from '@/lib/admin-api';

export async function POST() {
  try {
    const response = await adminProxyFetch("/api/v2/admin/wireguard/reconcile", {
      method: "POST",
    });
    return forwardUpstreamResponse(response);
  } catch (error) {
    return upstreamErrorResponse(error, 'wireguard gateway reconcile failed', 'Upstream request failed');
  }
}
