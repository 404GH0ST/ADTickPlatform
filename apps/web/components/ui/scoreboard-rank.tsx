"use client";

import type { ReactElement } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";

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
      {/* Rank Medal or Number */}
      <div className="flex w-8 items-center justify-center">
        {rank === 1 ? (
          <span className="text-xl" title="Gold Medal" role="img" aria-label="gold medal">🥇</span>
        ) : rank === 2 ? (
          <span className="text-xl" title="Silver Medal" role="img" aria-label="silver medal">🥈</span>
        ) : rank === 3 ? (
          <span className="text-xl" title="Bronze Medal" role="img" aria-label="bronze medal">🥉</span>
        ) : (
          <span className="text-sm font-bold text-muted-foreground">{rank}</span>
        )}
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
