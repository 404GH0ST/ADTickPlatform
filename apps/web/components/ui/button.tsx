import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';

import { cn } from '@/lib/utils';

const buttonVariants = cva(
  'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-sm text-sm font-medium outline-none transition-[background-color,border-color,color,box-shadow,opacity] active:translate-y-px disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
  {
    variants: {
      variant: {
        default:
          'border border-primary bg-primary text-primary-foreground hover:bg-primary/90',
        secondary:
          'border border-border bg-secondary text-secondary-foreground hover:bg-secondary/80',
        ghost: 'text-foreground hover:bg-muted hover:text-foreground',
        outline:
          'border border-border bg-background text-foreground hover:border-foreground/30 hover:bg-muted',
      },
      size: {
        default: 'h-9 px-3.5',
        sm: 'h-8 px-3 text-xs',
        touch: 'h-11 px-3 text-sm sm:h-8 sm:text-xs',
        lg: 'h-10 px-4',
        icon: 'size-8 shrink-0 p-0',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
  ref?: React.Ref<HTMLButtonElement>;
  /**
   * While disabled, stay focusable (aria-disabled) instead of using the native
   * attribute, which would drop keyboard focus to the page. For the key whose
   * own action is running.
   */
  holdFocus?: boolean;
}

export function Button({
  className,
  variant,
  size,
  asChild = false,
  holdFocus = false,
  disabled,
  onClick,
  ...props
}: ButtonProps) {
  const Comp = asChild ? Slot : 'button';
  const held = holdFocus && Boolean(disabled);

  return (
    <Comp
      data-slot="button"
      data-variant={variant ?? 'default'}
      data-size={size ?? 'default'}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
      disabled={held ? undefined : disabled}
      aria-disabled={held ? true : props['aria-disabled']}
      onClick={held ? (event: React.MouseEvent<HTMLButtonElement>) => event.preventDefault() : onClick}
    />
  );
}
