import type { ReactElement, SVGProps } from "react";

import type { ChallengeCategory } from "@/lib/challenge-metadata";

type IconProps = Omit<SVGProps<SVGSVGElement>, "children"> & {
  category: ChallengeCategory | "";
};

/*
 * Category glyphs, drawn on the same 24px grid and stroke as the app's other
 * icons (round caps and joins, currentColor) so they sit beside them:
 * - pwn:    a microchip with a corner cracked off (a binary, broken open).
 *           The crack runs edge to edge across a corner: a vertical squiggle
 *           in the 8px body reads as a letter, a horizontal one as a graph.
 * - web:    a browser window holding a pair of code brackets
 * - crypto: a key whose bit is a row of binary teeth
 * - none:   a two-unit server rack, for challenges without a category
 */
const PATHS: Record<ChallengeCategory | "none", ReactElement> = {
  pwn: (
    <>
      <rect x="6" y="6" width="12" height="12" rx="1.5" />
      <path d="M9.5 2v4M14.5 2v4M9.5 18v4M14.5 18v4M2 9.5h4M2 14.5h4M18 9.5h4M18 14.5h4" />
      <path d="M10 6l2 3.5-1.5 2 3 2L18 15" />
    </>
  ),
  web: (
    <>
      <rect x="3" y="4" width="18" height="16" rx="2" />
      <path d="M3 8.5h18" />
      <path d="M6 6.25h.01M8.5 6.25h.01" />
      <path d="m10 12.5-2.5 2.5 2.5 2.5" />
      <path d="m14 12.5 2.5 2.5-2.5 2.5" />
    </>
  ),
  crypto: (
    <>
      <circle cx="7.5" cy="16.5" r="3.5" />
      <path d="M10 14 20 4" />
      <path d="m15.5 8.5 2 2" />
      <path d="m18 6 2 2" />
      <path d="M6.5 16.5h.01M8.5 16.5h.01" />
    </>
  ),
  none: (
    <>
      <rect x="3.5" y="4" width="17" height="7" rx="1.5" />
      <rect x="3.5" y="13" width="17" height="7" rx="1.5" />
      <path d="M7 7.5h.01M7 16.5h.01" />
      <path d="M11 7.5h6M11 16.5h6" />
    </>
  ),
};

/**
 * Category glyph for a challenge. Decorative by default: pair it with a
 * visible label, or pass aria-label (and aria-hidden={false}) to stand alone.
 */
export function ChallengeCategoryIcon({ category, strokeWidth = 2, ...props }: IconProps): ReactElement {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={strokeWidth}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
      data-category={category || "none"}
      {...props}
    >
      {PATHS[category || "none"]}
    </svg>
  );
}
