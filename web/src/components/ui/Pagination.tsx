import { Button } from "./Button";

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

  return (
    <nav aria-label="ניווט עמודים" className="flex items-center justify-center gap-3 pt-4">
      {offset > 0 && (
        <Button variant="secondary" size="sm" onClick={onPrev}>
          הקודם
        </Button>
      )}
      <span className="text-sm text-muted-foreground tabular-nums">
        {offset + 1}–{Math.min(offset + pageSize, total)} מתוך {total}
      </span>
      {offset + pageSize < total && (
        <Button variant="secondary" size="sm" onClick={onNext}>
          הבא
        </Button>
      )}
    </nav>
  );
}
