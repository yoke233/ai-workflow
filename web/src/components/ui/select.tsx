import {
  Children,
  isValidElement,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { ChevronDown, Check } from "lucide-react";
import { cn } from "@/lib/utils";

interface OptionData {
  value: string;
  label: ReactNode;
  disabled?: boolean;
}

interface SelectProps {
  value?: string;
  onValueChange?: (value: string) => void;
  children?: ReactNode;
  className?: string;
  disabled?: boolean;
  placeholder?: string;
}

function extractOptions(children: ReactNode): OptionData[] {
  const options: OptionData[] = [];
  Children.forEach(children, (child) => {
    if (
      isValidElement<{ value?: string; children?: ReactNode; disabled?: boolean }>(child) &&
      (child.type === "option" || child.type === SelectItem)
    ) {
      options.push({
        value: String(child.props.value ?? ""),
        label: child.props.children,
        disabled: child.props.disabled,
      });
    }
  });
  return options;
}

export function Select({
  value,
  onValueChange,
  children,
  className,
  disabled,
  placeholder,
}: SelectProps) {
  const [open, setOpen] = useState(false);
  const [menuStyle, setMenuStyle] = useState<CSSProperties | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  const options = extractOptions(children);
  const selected = options.find((option) => option.value === value);
  const displayLabel = selected?.label ?? placeholder ?? "\u00A0";

  const updateMenuPosition = () => {
    if (!triggerRef.current || typeof window === "undefined") return;

    const rect = triggerRef.current.getBoundingClientRect();
    const viewportPadding = 8;
    const gap = 6;
    const estimatedHeight = menuRef.current?.offsetHeight ?? 240;
    const availableBelow = window.innerHeight - rect.bottom - viewportPadding;
    const availableAbove = rect.top - viewportPadding;
    const openAbove =
      availableBelow < Math.min(estimatedHeight, 240) &&
      availableAbove > availableBelow;
    const maxHeight = Math.max(
      120,
      (openAbove ? availableAbove : availableBelow) - gap,
    );
    const resolvedHeight = Math.min(estimatedHeight, maxHeight);
    const width = Math.min(
      Math.max(rect.width, 128),
      window.innerWidth - viewportPadding * 2,
    );
    const left = Math.min(
      Math.max(viewportPadding, rect.left),
      window.innerWidth - viewportPadding - width,
    );
    const top = openAbove
      ? Math.max(viewportPadding, rect.top - resolvedHeight - gap)
      : Math.min(
          window.innerHeight - viewportPadding - resolvedHeight,
          rect.bottom + gap,
        );

    setMenuStyle({
      position: "fixed",
      top,
      left,
      width,
      maxHeight,
    });
  };

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      const clickedTrigger =
        containerRef.current?.contains(target) ?? false;
      const clickedMenu = menuRef.current?.contains(target) ?? false;
      if (!clickedTrigger && !clickedMenu) {
        setOpen(false);
      }
    };
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open]);

  useLayoutEffect(() => {
    if (!open) {
      setMenuStyle(null);
      return;
    }

    updateMenuPosition();

    const handleViewportChange = () => {
      updateMenuPosition();
    };

    window.addEventListener("resize", handleViewportChange);
    window.addEventListener("scroll", handleViewportChange, true);
    return () => {
      window.removeEventListener("resize", handleViewportChange);
      window.removeEventListener("scroll", handleViewportChange, true);
    };
  }, [open, value, children]);

  return (
    <div ref={containerRef} className="relative">
      <button
        ref={triggerRef}
        type="button"
        disabled={disabled}
        className={cn(
          "flex h-10 w-full items-center justify-between gap-2 rounded-[10px] border border-slate-200 bg-white px-3 py-2 text-sm text-slate-950 transition",
          "hover:border-slate-300 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-400",
          "disabled:cursor-not-allowed disabled:opacity-50",
          className,
        )}
        onClick={() => setOpen((prev) => !prev)}
      >
        <span className="truncate text-left">{displayLabel}</span>
        <ChevronDown
          className={cn(
            "h-4 w-4 shrink-0 text-slate-400 transition-transform",
            open && "rotate-180",
          )}
        />
      </button>

      {open &&
        typeof document !== "undefined" &&
        createPortal(
          <div
            ref={menuRef}
            style={menuStyle ?? { position: "fixed", visibility: "hidden" }}
            className="z-[100] overflow-y-auto rounded-xl border border-slate-200/80 bg-white py-1 shadow-[0_8px_30px_rgba(0,0,0,0.08),0_2px_8px_rgba(0,0,0,0.04)] animate-select-in"
          >
            {options.map((option) => (
              <button
                key={option.value}
                type="button"
                disabled={option.disabled}
                className={cn(
                  "flex w-full items-center gap-2 px-3 py-2 text-sm transition-colors",
                  "hover:bg-slate-50",
                  value === option.value
                    ? "font-medium text-slate-950"
                    : "text-slate-600",
                  option.disabled && "cursor-not-allowed opacity-50",
                )}
                onClick={() => {
                  onValueChange?.(option.value);
                  setOpen(false);
                }}
              >
                <Check
                  className={cn(
                    "h-3.5 w-3.5 shrink-0",
                    value === option.value ? "opacity-100" : "opacity-0",
                  )}
                />
                <span className="truncate">{option.label}</span>
              </button>
            ))}
          </div>,
          document.body,
        )}
    </div>
  );
}

export function SelectItem(_props: {
  value: string;
  children?: ReactNode;
  disabled?: boolean;
}) {
  return null;
}

SelectItem.displayName = "SelectItem";
