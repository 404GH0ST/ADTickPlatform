"use client";

import { useEffect, useMemo, useState, type ReactElement } from "react";
import { CheckCircle2, Circle } from "lucide-react";

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { PlatformOverview, ServiceRow } from "@/lib/dashboard-types";

export const VPN_DOWNLOADED_STORAGE_KEY =
  "adplatform.participant.vpn-downloaded";

type ChecklistItem = {
  id: string;
  label: string;
  detail: string;
  done: boolean;
};

const DISMISS_KEY = "adplatform.participant.onboarding.dismissed";

export function markVpnConfigDownloaded() {
  try {
    window.localStorage.setItem(VPN_DOWNLOADED_STORAGE_KEY, "1");
    window.dispatchEvent(new Event("adplatform:vpn-downloaded"));
  } catch {
    // ignore storage failures
  }
}

export function OnboardingChecklist({
  overview,
  services,
}: {
  overview: PlatformOverview;
  services: ServiceRow[];
}): ReactElement | null {
  const [dismissed, setDismissed] = useState(false);
  const [vpnDownloaded, setVpnDownloaded] = useState(false);

  useEffect(() => {
    try {
      setDismissed(window.localStorage.getItem(DISMISS_KEY) === "1");
      setVpnDownloaded(
        window.localStorage.getItem(VPN_DOWNLOADED_STORAGE_KEY) === "1",
      );
    } catch {
      setDismissed(false);
      setVpnDownloaded(false);
    }

    const onVpn = () => setVpnDownloaded(true);
    window.addEventListener("adplatform:vpn-downloaded", onVpn);
    return () => window.removeEventListener("adplatform:vpn-downloaded", onVpn);
  }, []);

  const items = useMemo<ChecklistItem[]>(() => {
    const unlocked = services.some((service) => service.unlocked);
    const hasServices = services.length > 0;
    const hasTeam = (overview.teamID ?? 0) > 0;
    return [
      {
        id: "team",
        label: "Join a team",
        detail: "Use a team join key if you registered without one.",
        done: hasTeam,
      },
      {
        id: "vpn",
        label: "Download WireGuard config",
        detail:
          "Use the VPN Config button in the header, then bring the tunnel up.",
        done: hasTeam && vpnDownloaded,
      },
      {
        id: "services",
        label: "Confirm owned services are listed",
        detail: "Open Services after the organizer deploys challenges.",
        done: hasServices,
      },
      {
        id: "unlock",
        label: "Unlock at least one service for SSH",
        detail:
          "Exploit your own service, submit the unlock proof, then open SSH.",
        done: unlocked,
      },
    ];
  }, [overview.teamID, services, vpnDownloaded]);

  const remaining = items.filter((item) => !item.done).length;
  if (dismissed || overview.role === "organizer" || !overview.authenticated) {
    return null;
  }
  if (remaining === 0) {
    return null;
  }

  return (
    <Card data-testid="onboarding-checklist">
      <CardHeader className="flex flex-row items-start justify-between gap-3 space-y-0">
        <div>
          <CardTitle className="text-base">Match readiness</CardTitle>
          <CardDescription>
            Complete these steps before the first scoring tick. {remaining}{" "}
            remaining.
          </CardDescription>
        </div>
        <button
          type="button"
          className="text-xs font-medium text-muted-foreground underline-offset-2 hover:underline"
          onClick={() => {
            try {
              window.localStorage.setItem(DISMISS_KEY, "1");
            } catch {
              // ignore storage failures
            }
            setDismissed(true);
          }}
        >
          Dismiss
        </button>
      </CardHeader>
      <CardContent>
        <ul className="grid gap-2">
          {items.map((item) => (
            <li
              key={item.id}
              className="flex items-start gap-2 rounded-sm border border-border/60 px-3 py-2 text-sm"
            >
              {item.done ? (
                <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-positive" />
              ) : (
                <Circle className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
              )}
              <div>
                <p
                  className={
                    item.done
                      ? "font-medium text-muted-foreground line-through"
                      : "font-medium"
                  }
                >
                  {item.label}
                </p>
                <p className="text-xs text-muted-foreground">{item.detail}</p>
              </div>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}
