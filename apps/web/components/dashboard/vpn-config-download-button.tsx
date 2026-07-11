"use client";

import type { ReactElement } from "react";
import { Download } from "lucide-react";

import { markVpnConfigDownloaded } from "@/components/dashboard/onboarding-checklist";
import { Button } from "@/components/ui/button";

export function VpnConfigDownloadButton(): ReactElement {
  return (
    <Button asChild data-testid="participant-vpn-config" variant="outline">
      <a
        href="/api/platform/me/wireguard"
        download
        onClick={() => markVpnConfigDownloaded()}
      >
        <Download className="h-4 w-4" />
        VPN Config
      </a>
    </Button>
  );
}
