import { cn } from "@/utils";
import { Category } from "@/types/todo";

interface TodoFilterProps {
  categories: Category[];
  activeFilter: string;
  onFilterChange: (filter: string) => void;
}

export function TodoFilter({ categories, activeFilter, onFilterChange }: TodoFilterProps) {
  const allFilters = [{ id: "all", name: "全部", color: "" }, ...categories];

  return (
    <div className="flex flex-wrap gap-2">
      {allFilters.map((cat) => (
        <button
          key={cat.id}
          onClick={() => onFilterChange(cat.id)}
          className={cn(
            "inline-flex h-7 items-center rounded-full border px-3 text-xs font-medium transition-all",
            activeFilter === cat.id
              ? "border-primary bg-primary text-primary-foreground"
              : "border-border bg-background text-muted-foreground hover:bg-muted hover:text-foreground",
          )}
        >
          {cat.name}
        </button>
      ))}
    </div>
  );
}
