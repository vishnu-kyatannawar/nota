import { useEffect, useRef } from "react";

export type MenuItem = { label: string; onSelect: () => void; danger?: boolean; disabled?: boolean };

type Props = { x: number; y: number; items: MenuItem[]; onClose: () => void };

export function ContextMenu({ x, y, items, onClose }: Props) {
  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [onClose]);

  // Keep the menu on screen near the bottom or right edge.
  const left = Math.min(x, window.innerWidth - 200);
  const top = Math.min(y, window.innerHeight - items.length * 32 - 16);

  return (
    <div
      ref={ref}
      role="menu"
      style={{ left, top }}
      className="fixed z-50 min-w-44 rounded-md border border-border bg-surface-raised py-1 text-sm shadow-xl"
    >
      {items.map((it) => (
        <button
          key={it.label}
          type="button"
          role="menuitem"
          disabled={it.disabled}
          onClick={() => {
            onClose();
            it.onSelect();
          }}
          className={`block w-full px-3 py-1.5 text-left hover:bg-surface-sunken disabled:opacity-40 ${it.danger ? "text-danger" : ""}`}
        >
          {it.label}
        </button>
      ))}
    </div>
  );
}
