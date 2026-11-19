import { ReactNode } from "react";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Plus } from "lucide-react";

export function AdminRegistryCard({
  title,
  description,
  createLabel,
  onCreate,
  children,
}: {
  title: string;
  description: string;
  createLabel: string;
  onCreate: () => void;
  children: ReactNode;
}) {
  return (
    <Card className="flex h-full flex-col">
      <CardHeader className="flex flex-col items-start gap-3 space-y-0 pb-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0 max-w-full space-y-1.5 wrap-anywhere">
          <CardTitle as="h2">{title}</CardTitle>
          <CardDescription>{description}</CardDescription>
        </div>
        <Button onClick={onCreate} className="shrink-0" size="sm">
          <Plus className="mr-2 h-4 w-4" />
          {createLabel}
        </Button>
      </CardHeader>
      <CardContent className="flex-1 overflow-auto p-0">
        {children}
      </CardContent>
    </Card>
  );
}
