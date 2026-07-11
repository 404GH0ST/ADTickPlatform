"use client";

import {
  useLayoutEffect,
  useMemo,
  useState,
  type ReactElement,
} from "react";
import { CheckCircle2, ChevronDown, Circle, ListChecks } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { PlatformOverview } from "@/lib/dashboard-types";
import { cn } from "@/lib/utils";
import { VPN_DOWNLOADED_STORAGE_KEY } from "@/components/dashboard/onboarding-checklist";

type ChecklistItem = {
  id: string;
  label: string;
  detail: string;
  done: boolean;
};

function readVpnDownloaded(): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  try {
    return window.localStorage.getItem(VPN_DOWNLOADED_STORAGE_KEY) === "1";
  } catch {
    return false;
  }
}

function buildItems(
  overview: PlatformOverview,
  vpnDownloaded: boolean,
): ChecklistItem[] {
  const hasTeam = (overview.teamID ?? 0) > 0;
  // Prefer overview count (available immediately on navigation) over a
  // services fetch that would flash "incomplete" while empty.
  const hasServices = (overview.ownServiceCount ?? 0) > 0;
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
      detail: "Use the VPN Config button in the header, then bring the tunnel up.",
      done: hasTeam && vpnDownloaded,
    },
    {
      id: "services",
      label: "Confirm owned services are listed",
      detail: "Open Services after the organizer deploys challenges.",
      done: hasServices,
    },
  ];
}

export function useMatchReadiness(overview: PlatformOverview): {
  visible: boolean;
  remaining: number;
  items: ChecklistItem[];
  open: boolean;
  setOpen: (open: boolean) => void;
  toggle: () => void;
} {
  const [open, setOpen] = useState(false);
  // Wait for client hydrate before showing: SSR/localStorage gap would flash
  // the nav chip when readiness is already finished.
  const [clientReady, setClientReady] = useState(false);
  const [vpnDownloaded, setVpnDownloaded] = useState(false);

  useLayoutEffect(() => {
    setVpnDownloaded(readVpnDownloaded());
    setClientReady(true);

    const onVpn = () => setVpnDownloaded(true);
    window.addEventListener("adplatform:vpn-downloaded", onVpn);
    return () => window.removeEventListener("adplatform:vpn-downloaded", onVpn);
  }, []);

  const items = useMemo(
    () => buildItems(overview, vpnDownloaded),
    [overview, vpnDownloaded],
  );
  const remaining = items.filter((item) => !item.done).length;
  const visible =
    clientReady &&
    overview.authenticated &&
    overview.role !== "organizer" &&
    remaining > 0;

  useLayoutEffect(() => {
    if (!visible && open) {
      setOpen(false);
    }
  }, [visible, open]);

  return {
    visible,
    remaining,
    items,
    open: visible && open,
    setOpen,
    toggle: () => setOpen((current) => !current),
  };
}

export function MatchReadinessNavButton({
  remaining,
  open,
  onToggle,
}: {
  remaining: number;
  open: boolean;
  onToggle: () => void;
}): ReactElement {
  return (
    <Button
      type="button"
      variant={open ? "secondary" : "outline"}
      size="sm"
      data-testid="match-readiness-toggle"
      aria-expanded={open}
      aria-controls="match-readiness-panel"
      onClick={onToggle}
      className="gap-1.5"
    >
      <ListChecks className="h-4 w-4" />
      <span>Match readiness</span>
      <span className="rounded-sm bg-background/60 px-1.5 py-0.5 text-xs font-semibold tabular-nums text-highlight">
        {remaining}
      </span>
      <ChevronDown
        className={cn(
          "h-3.5 w-3.5 text-muted-foreground transition-transform",
          open && "rotate-180",
        )}
      />
    </Button>
  );
}

export function MatchReadinessPanel({
  items,
  remaining,
}: {
  items: ChecklistItem[];
  remaining: number;
}): ReactElement {
  return (
    <section
      id="match-readiness-panel"
      data-testid="onboarding-checklist"
      className="surface-workroom rounded-sm border border-border bg-card p-3"
    >
      <div className="mb-2 flex flex-wrap items-baseline justify-between gap-2">
        <div>
          <h2 className="text-sm font-semibold">Match progress</h2>
          <p className="text-xs text-muted-foreground">
            Finish these before the first scoring tick. {remaining} remaining.
          </p>
        </div>
      </div>
      <ul className="grid gap-2 sm:grid-cols-2">
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
    </section>
  );
}
