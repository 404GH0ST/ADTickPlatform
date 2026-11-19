import type { KeyboardEvent } from "react";

export function moveCompositeFocus(
  event: KeyboardEvent<HTMLElement>,
  selector: string,
  vertical = false,
): HTMLButtonElement | undefined {
  const buttons = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>(selector));
  const current = buttons.findIndex((button) => button === event.target);
  if (current === -1) return;

  let next: number;
  switch (event.key) {
    case "ArrowRight":
    case "ArrowDown":
      if (event.key === "ArrowDown" && !vertical) return;
      next = (current + 1) % buttons.length;
      break;
    case "ArrowLeft":
    case "ArrowUp":
      if (event.key === "ArrowUp" && !vertical) return;
      next = (current + buttons.length - 1) % buttons.length;
      break;
    case "Home":
      next = 0;
      break;
    case "End":
      next = buttons.length - 1;
      break;
    default:
      return;
  }

  event.preventDefault();
  const button = buttons[next];
  button.focus();
  return button;
}
