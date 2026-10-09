import { ProgressRow } from "@/components/ProgressRow";
import { useTestStore } from "@/stores/testStore";

/**
 * Прогресс-бар массовой проверки. Подписан на стор сам, чтобы частые события
 * прогресса перерисовывали только этот небольшой компонент, а не всю страницу.
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
    <ProgressRow label="Проверка" total={total} completed={completed} current={current} onCancel={onCancel} />
  );
}
