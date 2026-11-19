'use client';

import { useEffect, useRef, useState, type ComponentProps } from 'react';

import { cn } from '@/lib/utils';

type ScrollableRegionProps = ComponentProps<'div'> & {
  axis: 'horizontal' | 'vertical';
  label: string;
};

export function ScrollableRegion({ axis, label, className, ...props }: ScrollableRegionProps) {
  const ref = useRef<HTMLDivElement>(null);
  const [overflowing, setOverflowing] = useState(false);

  useEffect(() => {
    const region = ref.current;
    if (!region) return;

    const update = () => setOverflowing(axis === 'horizontal'
      ? region.scrollWidth > region.clientWidth
      : region.scrollHeight > region.clientHeight);

    update();
    const observer = new ResizeObserver(update);
    observer.observe(region);
    if (region.firstElementChild) observer.observe(region.firstElementChild);
    return () => observer.disconnect();
  }, [axis]);

  return (
    <div
      {...props}
      ref={ref}
      className={cn('focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring', className)}
      role={overflowing ? 'region' : undefined}
      aria-label={overflowing ? label : undefined}
      tabIndex={overflowing ? 0 : undefined}
    />
  );
}
