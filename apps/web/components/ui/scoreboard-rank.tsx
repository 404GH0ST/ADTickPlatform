"use client";

import type { ReactElement } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";

type Props = {
  rank: number;
  delta: string;
  isCurrentTeam?: boolean;
};

export function RankBadge({ rank }: { rank: number }): ReactElement {
  if (rank === 1) {
    return (
      <div
        className="flex h-5 w-5 items-center justify-center rounded-sm bg-amber-500/10 text-[10px] font-extrabold uppercase tracking-wider text-amber-500 border border-amber-500/30 select-none shadow-sm font-mono"
        title="1st Place (Gold)"
      >
        1
      </div>
    );
  }
  if (rank === 2) {
    return (
      <div
        className="flex h-5 w-5 items-center justify-center rounded-sm bg-slate-400/10 text-[10px] font-extrabold uppercase tracking-wider text-slate-400 border border-slate-400/30 select-none shadow-sm font-mono"
        title="2nd Place (Silver)"
      >
        2
      </div>
    );
  }
  if (rank === 3) {
    return (
      <div
        className="flex h-5 w-5 items-center justify-center rounded-sm bg-orange-600/10 text-[10px] font-extrabold uppercase tracking-wider text-orange-600 border border-orange-600/30 select-none shadow-sm font-mono"
        title="3rd Place (Bronze)"
      >
        3
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
}: Props): ReactElement {
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
        {isStatic && <span className="text-muted-foreground/30">—</span>}
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
