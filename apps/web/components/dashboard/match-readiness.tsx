"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactElement,
} from "react";
import { CheckCircle2, ChevronDown, Circle, ListChecks } from "lucide-react";

import { Button } from "@/components/ui/button";
import { processApiResponse } from "@/lib/api-utils";
import type { PlatformOverview, ServiceRow } from "@/lib/dashboard-types";
import { cn } from "@/lib/utils";
import { VPN_DOWNLOADED_STORAGE_KEY } from "@/components/dashboard/onboarding-checklist";

/** Minimal team-service payload used client-side (avoid importing server platform-api). */
type TeamServiceState = {
  challenge_id: number;
  team_id: number;
  name: string;
  endpoint: string;
  status: ServiceRow["status"];
  checker: ServiceRow["checker"];
  unlocked: boolean;
  ssh_hint: string;
  last_event: string;
  reset_cooldown: string;
  maintenance?: boolean;
  sla_status?: ServiceRow["slaStatus"];
  sla_phase?: string;
  sla_tick_id?: number;
  sla_message?: string;
};

type ChecklistItem = {
  id: string;
  label: string;
  detail: string;
  done: boolean;
};

function buildItems(
  overview: PlatformOverview,
  services: ServiceRow[],
  vpnDownloaded: boolean,
): ChecklistItem[] {
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
      detail: "Use the VPN Config button in the header, then bring the tunnel up.",
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
}

function teamStatesToServiceRows(states: TeamServiceState[]): ServiceRow[] {
  return states.map((state) => ({
    id: `svc-${state.challenge_id}`,
    challengeId: state.challenge_id,
    teamId: state.team_id,
    name: state.name,
    endpoint: state.endpoint,
    port: Number(state.endpoint.split(":").at(-1) ?? 0),
    status: state.status,
    checker: state.checker,
    hasSourceDownload: false,
    unlocked: state.unlocked,
    sshHint: state.ssh_hint,
    lastEvent: state.last_event,
    resetCooldown: state.reset_cooldown,
    maintenance: state.maintenance ?? false,
    slaStatus: state.sla_status ?? "unknown",
    slaPhase: state.sla_phase ?? "",
    slaTickId: state.sla_tick_id ?? null,
    slaMessage: state.sla_message ?? "",
  }));
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
  const [vpnDownloaded, setVpnDownloaded] = useState(false);
  const [services, setServices] = useState<ServiceRow[]>([]);

  const loadServices = useCallback(async () => {
    if (!overview.authenticated || overview.role === "organizer") {
      setServices([]);
      return;
    }
    if ((overview.teamID ?? 0) <= 0) {
      setServices([]);
      return;
    }
    try {
      const response = await fetch("/api/platform/team/services", {
        cache: "no-store",
      });
      if (!response.ok) {
        return;
      }
      const states = await processApiResponse<TeamServiceState[]>(
        response,
        "/api/platform/team/services",
      );
      setServices(teamStatesToServiceRows(states));
    } catch {
      // keep last known services
    }
  }, [overview.authenticated, overview.role, overview.teamID]);

  useEffect(() => {
    try {
      setVpnDownloaded(
        window.localStorage.getItem(VPN_DOWNLOADED_STORAGE_KEY) === "1",
      );
    } catch {
      setVpnDownloaded(false);
    }
    const onVpn = () => setVpnDownloaded(true);
    window.addEventListener("adplatform:vpn-downloaded", onVpn);
    return () => window.removeEventListener("adplatform:vpn-downloaded", onVpn);
  }, []);

  useEffect(() => {
    void loadServices();
    const timer = window.setInterval(() => {
      void loadServices();
    }, 15000);
    return () => window.clearInterval(timer);
  }, [loadServices]);

  const items = useMemo(
    () => buildItems(overview, services, vpnDownloaded),
    [overview, services, vpnDownloaded],
  );
  const remaining = items.filter((item) => !item.done).length;
  const visible =
    overview.authenticated &&
    overview.role !== "organizer" &&
    remaining > 0;

  useEffect(() => {
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

