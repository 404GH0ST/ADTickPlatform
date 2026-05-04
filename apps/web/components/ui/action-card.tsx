import Link from 'next/link';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';

export type ActionCardProps = {
  href: string;
  title: string;
  description: string;
  meta: string;
};

export function ActionCard({ href, title, description, meta }: ActionCardProps) {
  return (
    <Link href={href} className="block min-w-0">
      <Card className="h-full border-border/80 transition-colors hover:border-foreground/30 hover:bg-muted/20">
        <CardHeader>
          <CardTitle>{title}</CardTitle>
          <CardDescription>{description}</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">{meta}</p>
        </CardContent>
      </Card>
    </Link>
  );
}
