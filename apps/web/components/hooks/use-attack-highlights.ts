import { useCallback, useRef, useState, useEffect } from "react";

export function useAttackHighlights(timeoutMs = 4000) {
  const [highlightedAttackIDs, setHighlightedAttackIDs] = useState<string[]>([]);
  const attackHighlightTimeoutRef = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (attackHighlightTimeoutRef.current !== null) {
        window.clearTimeout(attackHighlightTimeoutRef.current);
      }
    };
  }, []);

  const scheduleAttackHighlights = useCallback((ids: string[]): void => {
    if (attackHighlightTimeoutRef.current !== null) {
      window.clearTimeout(attackHighlightTimeoutRef.current);
    }

    setHighlightedAttackIDs(ids);
    attackHighlightTimeoutRef.current = window.setTimeout(() => {
      setHighlightedAttackIDs([]);
      attackHighlightTimeoutRef.current = null;
    }, timeoutMs);
  }, [timeoutMs]);

  const clearAttackHighlights = useCallback((): void => {
    if (attackHighlightTimeoutRef.current !== null) {
      window.clearTimeout(attackHighlightTimeoutRef.current);
      attackHighlightTimeoutRef.current = null;
    }
    setHighlightedAttackIDs([]);
  }, []);

  return {
    highlightedAttackIDs,
    scheduleAttackHighlights,
    clearAttackHighlights,
  };
}
