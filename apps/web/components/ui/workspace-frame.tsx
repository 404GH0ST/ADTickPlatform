import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

/**
 * Product workspace panel: solid work surface, 1px border, corner reticles.
 * Decorative guides stay on the panel edge — not full-viewport marketing rails.
 */
export function WorkspaceFrame({
  children,
  className,
  bodyClassName,
}: {
  children: ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <div className={cn("workspace-frame", className)}>
      <span className="frame-reticle frame-reticle--tl" aria-hidden />
      <span className="frame-reticle frame-reticle--tr" aria-hidden />
      <span className="frame-reticle frame-reticle--bl" aria-hidden />
      <span className="frame-reticle frame-reticle--br" aria-hidden />
      <div className={cn("workspace-frame-body", bodyClassName)}>{children}</div>
    </div>
  );
}
