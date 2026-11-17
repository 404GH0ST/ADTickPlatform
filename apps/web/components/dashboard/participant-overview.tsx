"use client";

import type { ReactElement } from "react";

import { ThemeSlot } from "@/components/theme/theme-slot";
import { ActionCard } from "@/components/ui/action-card";
import type { OverviewLink, OverviewViewProps } from "@/lib/theme-contract";

/** Participant home body. Themes can replace it via the Overview slot. */
export function ParticipantOverview({
  scoreCount,
  serviceCount,
  attackCount,
  teamName,
}: Omit<OverviewViewProps, "links">): ReactElement {
  const links: OverviewLink[] = [
    {
      id: "scoreboard",
      href: "/scoreboard",
      title: "Scoreboard",
      description: "See the live standings without the rest of the participant controls.",
      meta: `${scoreCount} teams live`,
    },
    {
      id: "services",
      href: "/services",
      title: "Services",
      description: "Submit flags and manage unlocks, SSH access, restarts, and resets.",
      meta: `${serviceCount} services under your team`,
    },
    {
      id: "attacks",
      href: "/attacks",
      title: "Attacks",
      description: "Review accepted submissions and recent attack activity.",
      meta: `${attackCount} accepted events`,
    },
    {
      id: "docs",
      href: "/docs/participant",
      title: "Manual & API",
      description: "Open the participant workflow, Swagger UI, and raw OpenAPI spec.",
      meta: "manual, swagger, openapi",
    },
  ];
  return (
    <ThemeSlot
      name="Overview"
      fallback={DefaultOverviewView}
      props={{ links, teamName, scoreCount, serviceCount, attackCount }}
    />
  );
}

export function DefaultOverviewView({ links }: OverviewViewProps): ReactElement {
  return (
    <section className="surface-inset grid overflow-hidden rounded-sm border md:grid-cols-2 xl:grid-cols-4 [&>*]:border-b [&>*]:border-border [&>*:last-child]:border-b-0 md:[&>*:nth-child(odd)]:border-r md:[&>*:nth-last-child(-n+2)]:border-b-0 xl:[&>*]:border-b-0 xl:[&>*]:border-r xl:[&>*:last-child]:border-r-0">
      {links.map((link) => (
        <ActionCard
          key={link.id}
          href={link.href}
          title={link.title}
          description={link.description}
          meta={link.meta}
        />
      ))}
    </section>
  );
}
