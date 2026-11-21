import type { Dispatch, SetStateAction } from "react";

/**
 * Ref for a controlled field whose page is server-rendered. Text typed, pasted
 * or autofilled before hydration exists only in the DOM, and React would erase
 * it on the next render. This copies it into state when the field hydrates.
 */
export function keepEarlyInput(setValue: Dispatch<SetStateAction<string>>) {
  return (node: HTMLInputElement | HTMLTextAreaElement | null) => {
    const early = node?.value;
    if (early) {
      setValue((current) => current || early);
    }
  };
}
