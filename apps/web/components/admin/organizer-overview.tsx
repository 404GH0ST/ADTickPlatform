"use client";

import type { ReactElement, ReactNode } from "react";

import { ThemeSlot } from "@/components/theme/theme-slot";
import Link from "next/link";
import type { OverviewLink } from "@/lib/theme-contract";

export type OrganizerOverviewLink = Omit<OverviewLink, "id"> & { id: string };

export type OrganizerOverviewViewProps = {
  links: OrganizerOverviewLink[];
  /** Rendered trusted-state panel (themed via OrganizerTrustedState). */
  trustedState: ReactNode;
  /** Rendered announcements panel (themed via OrganizerAnnouncements). */
  announcements: ReactNode;
};

/** Organizer home body (/admin). Themes can replace it via the OrganizerOverview slot. */
export function OrganizerOverview(props: OrganizerOverviewViewProps): ReactElement {
  return <ThemeSlot name="OrganizerOverview" fallback={DefaultOrganizerOverviewView} props={props} />;
}

export function DefaultOrganizerOverviewView({
  links,
  trustedState,
  announcements,
}: OrganizerOverviewViewProps): ReactElement {
  return (
    <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,1fr)_19rem]">
      <div className="grid min-w-0 gap-4">
        {trustedState}
        {announcements}
      </div>
      <nav aria-label="Organizer sections" data-slot="card" className="surface-workroom overflow-hidden rounded-sm border">
        <h2 className="border-b border-border px-3.5 py-2.5 text-sm font-semibold">Sections</h2>
        <ul className="divide-y divide-border">
          {links.map((link) => (
            <li key={link.id}>
              <Link
                href={link.href}
                className="group grid gap-0.5 px-3.5 py-2.5 transition-colors hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
              >
                <span className="flex items-baseline justify-between gap-3">
                  <span className="text-sm font-semibold text-foreground">{link.title}</span>
                  <span className="text-xs tabular-nums text-muted-foreground">{link.meta}</span>
                </span>
                <span className="text-xs leading-5 text-muted-foreground">{link.description}</span>
              </Link>
            </li>
          ))}
        </ul>
      </nav>
    </div>
  );
}
