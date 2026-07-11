"use client";

/** localStorage flag set when participant downloads WireGuard config. */
export const VPN_DOWNLOADED_STORAGE_KEY =
  "adplatform.participant.vpn-downloaded";

export function markVpnConfigDownloaded() {
  try {
    window.localStorage.setItem(VPN_DOWNLOADED_STORAGE_KEY, "1");
    window.dispatchEvent(new Event("adplatform:vpn-downloaded"));
  } catch {
    // ignore storage failures
  }
}
