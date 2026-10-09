import { X } from "lucide-react";

import { Button } from "@/components/ui/button";

interface ProgressRowProps {
  label: string;
  total: number;
  completed: number;
  current: string;
  onCancel: () => void;
}

/** Презентационная строка прогресса массовой операции (тест, определение гео). */
export function ProgressRow({ label, total, completed, current, onCancel }: ProgressRowProps) {
  return (
    <div className="flex items-center gap-3 border-b border-border px-6 py-2 text-sm">
      <span className="whitespace-nowrap">
        {label}: {completed}
        {total ? ` / ${total}` : ""}
      </span>
      <div className="h-1.5 flex-1 overflow-hidden rounded bg-muted">
        <div
          className="h-full rounded bg-primary transition-[width]"
          style={{ width: `${total ? (completed / total) * 100 : 0}%` }}
        />
      </div>
      <span className="w-40 truncate text-xs text-muted-foreground">{current}</span>
      <Button size="sm" variant="outline" onClick={onCancel}>
        <X className="size-3.5" />
        Отмена
      </Button>
    </div>
  );
}
