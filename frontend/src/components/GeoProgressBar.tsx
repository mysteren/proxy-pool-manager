import { ProgressRow } from "@/components/ProgressRow";
import { useGeoStore } from "@/stores/geoStore";

/** Прогресс-бар определения гео для MTProto-прокси. */
export function GeoProgressBar({ onCancel }: { onCancel: () => void }) {
  const running = useGeoStore((s) => s.running);
  const total = useGeoStore((s) => s.total);
  const completed = useGeoStore((s) => s.completed);
  const current = useGeoStore((s) => s.current);

  if (!running) {
    return null;
  }
  return (
    <ProgressRow
      label="Определение гео"
      total={total}
      completed={completed}
      current={current}
      onCancel={onCancel}
    />
  );
}
