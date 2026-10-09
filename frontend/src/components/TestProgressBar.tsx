import { X } from "lucide-react";

import { Button } from "@/components/ui/button";
import { useTestStore } from "@/stores/testStore";

/**
 * Прогресс-бар массовой проверки. Подписан на стор сам, чтобы частые события
 * прогресса (десятки в секунду) перерисовывали только этот небольшой компонент,
 * а не всю страницу с таблицей.
 */
export function TestProgressBar({ onCancel }: { onCancel: () => void }) {
  const running = useTestStore((s) => s.running);
  const total = useTestStore((s) => s.total);
  const completed = useTestStore((s) => s.completed);
  const current = useTestStore((s) => s.current);

  if (!running) {
    return null;
  }

  return (
    <div className="flex items-center gap-3 border-b border-border px-6 py-2 text-sm">
      <span className="whitespace-nowrap">
        Проверка: {completed}
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
