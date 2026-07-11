import Link from 'next/link';

export type ActionCardProps = {
  href: string;
  title: string;
  description: string;
  meta: string;
};

export function ActionCard({ href, title, description, meta }: ActionCardProps) {
  return (
    <Link
      href={href}
      className="group grid min-w-0 grid-rows-[auto_1fr_auto] bg-transparent transition-colors hover:bg-muted/40 focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-card"
    >
      <div className="flex items-start justify-between gap-3 border-b border-border px-3.5 py-2.5">
        <h2 className="text-base font-semibold leading-6 tracking-tight text-foreground">
          {title}
        </h2>
        <span className="shrink-0 text-xs font-medium text-muted-foreground transition-colors group-hover:text-foreground">
          Open
        </span>
      </div>
      <p className="max-w-[56ch] px-3.5 py-2.5 text-sm leading-6 text-muted-foreground">
        {description}
      </p>
      <p className="border-t border-border px-3.5 py-2 text-xs font-medium tabular-nums text-muted-foreground">
        {meta}
      </p>
    </Link>
  );
}
