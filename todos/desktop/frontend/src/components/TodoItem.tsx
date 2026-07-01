import { Trash2 } from "lucide-react";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";
import { Todo, Category } from "@/types/todo";
import { cn } from "@/utils";

interface TodoItemProps {
  todo: Todo;
  category?: Category;
  onToggle: (id: string) => void;
  onRemove: (id: string) => void;
}

export function TodoItem({ todo, category, onToggle, onRemove }: TodoItemProps) {
  return (
    <div
      className={cn(
        "group flex items-center gap-3 rounded-lg border border-border bg-card px-3 py-2.5 transition-all hover:shadow-sm",
        todo.completed && "opacity-60",
      )}
    >
      <Checkbox
        checked={todo.completed}
        onCheckedChange={() => onToggle(todo.id)}
      />
      <span
        className={cn(
          "flex-1 text-sm",
          todo.completed && "text-muted-foreground line-through",
        )}
      >
        {todo.title}
      </span>
      {category && (
        <span
          className={cn(
            "inline-flex h-5 shrink-0 items-center rounded-full px-2 text-xs font-medium",
            category.color,
          )}
        >
          {category.name}
        </span>
      )}
      <Button
        variant="ghost"
        size="icon-xs"
        onClick={() => onRemove(todo.id)}
        className="opacity-0 transition-opacity group-hover:opacity-100"
      >
        <Trash2 className="size-3.5 text-muted-foreground" />
      </Button>
    </div>
  );
}
