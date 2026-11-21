"use client";

import Link from "next/link";
import { ArrowUpRight, BookOpen, Server, Trophy, Zap, type LucideIcon } from "lucide-react";

import { cn, LinkPendingSignal, type OverviewLink, type OverviewViewProps } from "@/components/theme/kit";

const STYLE: Record<OverviewLink["id"], { icon: LucideIcon; layout: string }> = {
  services: { icon: Server, layout: "md:col-span-2 md:min-h-[17rem]" },
  scoreboard: { icon: Trophy, layout: "md:min-h-[17rem]" },
  attacks: { icon: Zap, layout: "md:min-h-[14rem]" },
  docs: { icon: BookOpen, layout: "md:col-span-2 md:min-h-[14rem]" },
};

/**
 * Destination blocks: white sheets with a blue (chrome) icon key, so no
 * block borrows a status colour. Services leads because it is where play
 * happens.
 */
export function NbOverview({ links, teamName, scoreCount, serviceCount, attackCount, unavailable }: OverviewViewProps) {
  // A failed count has no number: the link's meta line says it is unavailable.
  const counts: Partial<Record<OverviewLink["id"], number>> = {
    services: unavailable?.services ? undefined : serviceCount,
    scoreboard: unavailable?.scores ? undefined : scoreCount,
    attacks: unavailable?.attacks ? undefined : attackCount,
  };
  const order: OverviewLink["id"][] = ["services", "scoreboard", "attacks", "docs"];
  const sorted = [...links].sort((a, b) => order.indexOf(a.id) - order.indexOf(b.id));

  return (
    <div data-testid="nb-overview" className="grid grid-cols-1 gap-5">
      {teamName ? (
        <p className="wrap-anywhere text-base text-muted-foreground">
          Playing as <span className="nb-display font-bold text-foreground">{teamName}</span>
        </p>
      ) : null}
      <div className="grid grid-cols-1 gap-5 md:grid-cols-3">
        {sorted.map((link) => {
          const style = STYLE[link.id];
          const Icon = style.icon;
          const count = counts[link.id];
          const hero = link.id === "services";
          return (
            <Link
              key={link.id}
              href={link.href}
              className={cn(
                "nb-pop nb-lift group relative flex min-w-0 flex-col justify-between gap-6 rounded-[var(--nb-radius)] bg-card p-5 text-card-foreground sm:p-6",
                style.layout,
              )}
            >
              <div className="flex items-start justify-between gap-4">
                <span className="nb-icon-key grid size-11 place-items-center rounded-[var(--nb-radius-sm)]">
                  <Icon className="size-5" strokeWidth={2.4} aria-hidden />
                </span>
                <ArrowUpRight
                  className="size-6 transition-transform duration-150 group-hover:-translate-y-0.5 group-hover:translate-x-0.5"
                  strokeWidth={2.6}
                  aria-hidden
                />
              </div>
              <div className="grid grid-cols-1 gap-2 wrap-anywhere">
                <div className="grid gap-1.5">
                  <h2 className={cn("nb-display font-extrabold leading-[0.95]", hero ? "text-4xl sm:text-5xl" : "text-3xl")}>
                    {link.title}
                  </h2>
                  <p className={cn("max-w-[46ch] text-pretty text-sm font-medium leading-6 text-muted-foreground", hero && "sm:text-base")}>{link.description}</p>
                </div>
                <p className="flex flex-wrap items-baseline gap-2 pt-1 text-sm font-semibold">
                  {typeof count === "number" ? (
                    <span className={cn("nb-display font-extrabold tabular-nums leading-none", hero ? "text-5xl" : "text-3xl")}>
                      {count}
                    </span>
                  ) : null}
                  <span>{link.meta.replace(/^\d+\s*/, "")}</span>
                </p>
              </div>
              <LinkPendingSignal />
            </Link>
          );
        })}
      </div>
    </div>
  );
}
