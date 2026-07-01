import { ClipboardList } from "lucide-react";
import { TodoItem } from "./TodoItem";
import { Todo, Category } from "@/types/todo";

interface TodoListProps {
  todos: Todo[];
  categories: Category[];
  onToggle: (id: string) => void;
  onRemove: (id: string) => void;
}

export function TodoList({ todos, categories, onToggle, onRemove }: TodoListProps) {
  const categoryMap = Object.fromEntries(categories.map((c) => [c.id, c]));

  if (todos.length === 0) {
    return (
      <div className="flex flex-col items-center gap-3 py-12 text-muted-foreground">
        <ClipboardList className="size-10 opacity-40" />
        <p className="text-sm">暂无待办事项</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      {todos.map((todo) => (
        <TodoItem
          key={todo.id}
          todo={todo}
          category={categoryMap[todo.category]}
          onToggle={onToggle}
          onRemove={onRemove}
        />
      ))}
    </div>
  );
}
