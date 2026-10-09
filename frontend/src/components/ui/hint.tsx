import { useRef, useState, type ReactNode } from "react";

import { cn } from "@/lib/utils";

interface HintProps {
  /** Содержимое подсказки: заголовок + пояснение. */
  content: ReactNode;
  /** Текст/элемент, на который наводят. */
  children: ReactNode;
  className?: string;
  /** Не добавлять оформление триггера (клавиша-кнопка и т.п.). */
  plain?: boolean;
  /** Делать триггер фокусируемым (для неинтерактивных полей). По умолчанию — да. */
  focusable?: boolean;
}

const TIP_WIDTH = 288; // w-72

/**
 * Всплывающая подсказка без сторонних зависимостей. Позиционируется через
 * position: fixed по координатам триггера, поэтому не обрезается в таблицах с
 * overflow. Показывается по наведению и по фокусу (доступность с клавиатуры).
 */
export function Hint({ content, children, className, plain, focusable = true }: HintProps) {
  const ref = useRef<HTMLSpanElement>(null);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);

  const show = () => {
    const el = ref.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    const half = TIP_WIDTH / 2;
    const left = Math.min(Math.max(r.left + r.width / 2, half + 8), window.innerWidth - half - 8);
    setPos({ left, top: r.bottom + 8 });
  };

  const hide = () => setPos(null);

  return (
    <span
      ref={ref}
      tabIndex={focusable ? 0 : undefined}
      className={cn(
        "outline-none",
        !plain &&
          "cursor-help underline decoration-dotted decoration-muted-foreground/60 underline-offset-4",
        className,
      )}
      onMouseEnter={show}
      onMouseLeave={hide}
      onFocus={show}
      onBlur={hide}
    >
      {children}
      {pos && (
        <span
          role="tooltip"
          style={{ left: pos.left, top: pos.top, width: TIP_WIDTH }}
          className="pointer-events-none fixed z-50 -translate-x-1/2 rounded-md border border-border bg-popover px-3 py-2 text-left text-xs leading-relaxed font-normal whitespace-normal text-popover-foreground shadow-md"
        >
          {content}
        </span>
      )}
    </span>
  );
}
