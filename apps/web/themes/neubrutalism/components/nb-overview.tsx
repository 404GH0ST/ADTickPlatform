"use client";

import Link from "next/link";
import { ArrowUpRight, BookOpen, Server, Trophy, Zap, type LucideIcon } from "lucide-react";

import { cn, type OverviewLink, type OverviewViewProps } from "@/components/theme/kit";

const STYLE: Record<OverviewLink["id"], { fill: string; icon: LucideIcon; layout: string }> = {
  services: { fill: "nb-fill-aqua", icon: Server, layout: "md:col-span-2 md:min-h-[17rem]" },
  scoreboard: { fill: "nb-fill-yellow", icon: Trophy, layout: "md:min-h-[17rem]" },
  attacks: { fill: "nb-fill-pink", icon: Zap, layout: "md:min-h-[14rem]" },
  docs: { fill: "nb-fill-blue", icon: BookOpen, layout: "md:col-span-2 md:min-h-[14rem]" },
};

/** Big, colored destination blocks; Services leads because it is where play happens. */
export function NbOverview({ links, teamName, scoreCount, serviceCount, attackCount }: OverviewViewProps) {
  const counts: Partial<Record<OverviewLink["id"], number>> = {
    services: serviceCount,
    scoreboard: scoreCount,
    attacks: attackCount,
  };
  const order: OverviewLink["id"][] = ["services", "scoreboard", "attacks", "docs"];
  const sorted = [...links].sort((a, b) => order.indexOf(a.id) - order.indexOf(b.id));

  return (
    <div data-testid="nb-overview" className="grid gap-5">
      {teamName ? (
        <p className="text-base text-muted-foreground">
          Playing as <span className="nb-display font-bold text-foreground">{teamName}</span>
        </p>
      ) : null}
      <div className="grid gap-5 md:grid-cols-3">
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
                "nb-pop nb-lift group relative flex flex-col justify-between gap-6 rounded-[var(--nb-radius)] p-5 sm:p-6",
                style.fill,
                style.layout,
              )}
            >
              <div className="flex items-start justify-between gap-4">
                <span className="grid size-11 place-items-center rounded-[var(--nb-radius)] nb-tile border-2 border-current">
                  <Icon className="size-5" strokeWidth={2.4} aria-hidden />
                </span>
                <ArrowUpRight
                  className="size-6 transition-transform duration-150 group-hover:-translate-y-0.5 group-hover:translate-x-0.5"
                  strokeWidth={2.6}
                  aria-hidden
                />
              </div>
              <div className="grid gap-2">
                <div className="grid gap-1.5">
                  <h2 className={cn("nb-display font-extrabold leading-[0.95]", hero ? "text-4xl sm:text-5xl" : "text-3xl")}>
                    {link.title}
                  </h2>
                  <p className={cn("max-w-[46ch] text-sm leading-6 opacity-85", hero && "sm:text-base")}>{link.description}</p>
                </div>
                <p className="flex items-baseline gap-2 pt-1 text-sm font-semibold">
                  {typeof count === "number" ? (
                    <span className={cn("nb-display font-extrabold tabular-nums leading-none", hero ? "text-5xl" : "text-3xl")}>
                      {count}
                    </span>
                  ) : null}
                  <span className="opacity-80">{link.meta.replace(/^\d+\s*/, "")}</span>
                </p>
              </div>
            </Link>
          );
        })}
      </div>
    </div>
  );
}
