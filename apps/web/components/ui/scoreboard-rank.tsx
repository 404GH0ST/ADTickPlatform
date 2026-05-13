"use client";

import type { ReactElement } from "react";
import { ChevronDown, ChevronUp, Star } from "lucide-react";

type Props = {
  rank: number;
  delta: string;
  isCurrentTeam?: boolean;
};

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
      <div className="flex w-6 items-center justify-center">
        {isCurrentTeam ? (
          <Star className="text-highlight h-4 w-4 fill-current" />
        ) : (
          <Star className="h-4 w-4 text-muted-foreground/20" />
        )}
      </div>

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

      <div className="flex w-8 justify-end pr-1">
        <span className="text-base font-semibold">#{rank}</span>
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
