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
      className="group grid min-w-0 grid-rows-[auto_1fr_auto] bg-card transition-colors hover:bg-muted/35"
    >
      <div className="flex items-start justify-between gap-3 border-b px-3 py-2.5">
        <h2 className="text-base font-semibold leading-6">{title}</h2>
        <span className="shrink-0 text-xs font-medium text-muted-foreground transition-colors group-hover:text-foreground">
          Open
        </span>
      </div>
      <p className="max-w-[56ch] px-3 py-2.5 text-sm leading-6 text-muted-foreground">
        {description}
      </p>
      <p className="border-t px-3 py-2 text-sm text-muted-foreground">
        {meta}
      </p>
    </Link>
  );
}
