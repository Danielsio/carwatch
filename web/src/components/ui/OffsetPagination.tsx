import { Button } from "./button";

export type PaginationProps = {
  offset: number;
  total: number;
  pageSize: number;
  onPrev: () => void;
  onNext: () => void;
};

export function Pagination({
  offset,
  total,
  pageSize,
  onPrev,
  onNext,
}: PaginationProps) {
  if (total <= pageSize && offset === 0) return null;

  const isFirst = offset <= 0;
  const isLast = offset + pageSize >= total;

  return (
    <nav aria-label="ניווט עמודים" className="flex items-center justify-center gap-3 pt-4">
      <Button variant="secondary" size="sm" onClick={onPrev} disabled={isFirst}>
        הקודם
      </Button>
      <span className="text-sm text-muted-foreground tabular-nums">
        {offset + 1}–{Math.min(offset + pageSize, total)} מתוך {total}
      </span>
      <Button variant="secondary" size="sm" onClick={onNext} disabled={isLast}>
        הבא
      </Button>
    </nav>
  );
}
