"use client";

import type { ReactElement } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";

import { cn } from "@/lib/utils";

type Props = {
  rank: number;
  delta: string;
  isCurrentTeam?: boolean;
  disqualified?: boolean;
};

const medalClass = {
  1: "medal-gold",
  2: "medal-silver",
  3: "medal-bronze",
} as const;

const medalTitle = {
  1: "1st Place (Gold)",
  2: "2nd Place (Silver)",
  3: "3rd Place (Bronze)",
} as const;

export function DisqualifiedBadge(): ReactElement {
  return (
    <span
      data-testid="scoreboard-dq-badge"
      title="Disqualified"
      className="inline-flex h-5 items-center rounded-sm border border-negative/50 bg-negative/10 px-1.5 font-mono text-[10px] font-extrabold uppercase tracking-wider text-negative select-none"
    >
      <span aria-hidden>DQ</span>
      <span className="sr-only">Disqualified</span>
    </span>
  );
}

export function RankBadge({
  rank,
  disqualified,
}: {
  rank: number;
  disqualified?: boolean;
}): ReactElement {
  if (disqualified) {
    return <DisqualifiedBadge />;
  }
  if (rank === 1 || rank === 2 || rank === 3) {
    return (
      <div
        className={cn(
          "flex h-5 w-5 items-center justify-center rounded-sm border text-[10px] font-extrabold uppercase tracking-wider select-none shadow-sm font-mono",
          medalClass[rank],
        )}
        title={medalTitle[rank]}
      >
        {rank}
      </div>
    );
  }
  return (
    <span className="text-xs font-mono font-bold text-muted-foreground/80">#{rank}</span>
  );
}

export function ScoreboardRank({
  rank,
  delta,
  isCurrentTeam,
  disqualified,
}: Props): ReactElement {
  if (disqualified) {
    return (
      <div
        className="flex items-center gap-2"
        aria-label={`Disqualified${isCurrentTeam ? ", current team" : ""}`}
      >
        <div className="flex w-8 items-center justify-center">
          <DisqualifiedBadge />
        </div>
      </div>
    );
  }

  const deltaValue = parseInt(delta, 10);
  const isNew = delta === "new";
  const isUp = !isNaN(deltaValue) && deltaValue > 0;
  const isDown = !isNaN(deltaValue) && deltaValue < 0;
  const isStatic = !isNaN(deltaValue) && deltaValue === 0;

  return (
    <div
      className="flex items-center gap-2"
      aria-label={`Rank ${rank}, ${describeDelta(delta)}${isCurrentTeam ? ", current team" : ""}`}
    >
      {/* Rank Medal or Number */}
      <div className="flex w-8 items-center justify-center">
        <RankBadge rank={rank} />
      </div>

      {/* Rank Delta */}
      <div className="flex w-12 items-center justify-center gap-0.5 font-mono text-xs font-bold">
        {isUp && (
          <>
            <ChevronUp className="text-positive h-3.5 w-3.5" />
            <span className="text-positive">{Math.abs(deltaValue)}</span>
          </>
        )}
        {isDown && (
          <>
            <ChevronDown className="text-negative h-3.5 w-3.5" />
            <span className="text-negative">{Math.abs(deltaValue)}</span>
          </>
        )}
        {isStatic && (
          <span className="text-muted-foreground" aria-hidden>
            —
          </span>
        )}
        {isNew && (
          <span className="text-info text-[10px] font-semibold uppercase">
            New
          </span>
        )}
      </div>
    </div>
  );
}

function describeDelta(delta: string): string {
  const deltaValue = parseInt(delta, 10);
  if (delta === "new") {
    return "new entry";
  }
  if (Number.isNaN(deltaValue) || deltaValue === 0) {
    return "unchanged";
  }
  if (deltaValue > 0) {
    return `up ${deltaValue}`;
  }
  return `down ${Math.abs(deltaValue)}`;
}
