import type { ReactElement, ReactNode } from "react";

import type { ServiceRow } from "@/lib/dashboard-types";

/** Compact checker detail under the service state row (not historical series). */
export function ServiceSLADetail({
  service,
}: {
  service: ServiceRow;
}): ReactElement {
  const phase = formatSLAPhaseLabel(service.slaPhase);
  return (
    <div className="space-y-1 text-xs text-muted-foreground">
      {service.slaPhase || service.slaTickId ? (
        <div>
          {phase ? `Last checker phase: ${phase}` : null}
          {service.slaTickId
            ? `${phase ? " · " : ""}tick #${service.slaTickId}`
            : null}
        </div>
      ) : null}
      <div>{service.slaMessage || "No checker detail message yet."}</div>
    </div>
  );
}

export function formatSLAState(service: ServiceRow): ReactNode {
  const status = service.slaStatus;
  const phase = formatSLAPhaseLabel(service.slaPhase);
  const tick = service.slaTickId;
  const tickSuffix = tick ? ` on tick #${tick}` : "";

  switch (status) {
    case "ok":
      return <span className="font-semibold text-positive">ok{tickSuffix}</span>;
    case "recovering":
      if (phase) {
        return (
          <span className="font-semibold text-highlight">
            recovering after {phase}
            {tickSuffix}
          </span>
        );
      }
      return (
        <span className="font-semibold text-highlight">
          recovering{tickSuffix}
        </span>
      );
    case "flag_not_found":
      if (phase) {
        return (
          <span className="font-semibold text-negative">
            flag not found during {phase}
            {tickSuffix}
          </span>
        );
      }
      return (
        <span className="font-semibold text-negative">
          flag not found{tickSuffix}
        </span>
      );
    case "faulty":
      if (phase) {
        return (
          <span className="font-semibold text-negative">
            faulty during {phase}
            {tickSuffix}
          </span>
        );
      }
      return (
        <span className="font-semibold text-negative">faulty{tickSuffix}</span>
      );
    case "down":
      if (phase) {
        return (
          <span className="font-semibold text-negative">
            down during {phase}
            {tickSuffix}
          </span>
        );
      }
      return (
        <span className="font-semibold text-negative">down{tickSuffix}</span>
      );
    default:
      if (tick) {
        return (
          <span className="text-muted-foreground">
            awaiting detail after tick #{tick}
          </span>
        );
      }
      return (
        <span className="text-muted-foreground">awaiting checker detail</span>
      );
  }
}

function formatSLAPhaseLabel(phase: string): string {
  switch (phase.trim().toLowerCase()) {
    case "put":
      return "flag storage";
    case "get":
      return "flag retrieval";
    case "check":
      return "service functionality";
    default:
      return phase.trim();
  }
}
