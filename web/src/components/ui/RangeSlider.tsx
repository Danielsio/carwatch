import { cn } from "@/lib/utils";

export interface RangeSliderProps {
  min: number;
  max: number;
  step: number;
  value: number;
  onChange: (value: number) => void;
  formatLabel?: (value: number) => string;
  className?: string;
}

export function RangeSlider({
  min,
  max,
  step,
  value,
  onChange,
  formatLabel,
  className,
}: RangeSliderProps) {
  const label = formatLabel ? formatLabel(value) : String(value);
  const percent = ((value - min) / (max - min)) * 100;

  return (
    <div className={cn("space-y-3", className)}>
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium text-primary tabular-nums">
          {label}
        </span>
      </div>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="range-slider w-full"
        style={
          { "--range-percent": `${percent}%` } as React.CSSProperties
        }
      />
    </div>
  );
}
