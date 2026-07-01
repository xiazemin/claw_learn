import { useState, useCallback } from "react";
import { todoStorage } from "@/lib/storage";
import { Todo, DEFAULT_CATEGORIES } from "@/types/todo";
import { TodoHeader } from "@/components/TodoHeader";
import { TodoInput } from "@/components/TodoInput";
import { TodoFilter } from "@/components/TodoFilter";
import { TodoList } from "@/components/TodoList";

export function TodoPage() {
  const [todos, setTodos] = useState<Todo[]>(todoStorage.getAll);
  const [filter, setFilter] = useState("all");

  const refresh = useCallback(() => {
    setTodos(todoStorage.getAll());
  }, []);

  const handleAdd = useCallback(
    (title: string, category: string) => {
      todoStorage.add(title, category);
      refresh();
    },
    [refresh],
  );

  const handleToggle = useCallback(
    (id: string) => {
      todoStorage.toggle(id);
      refresh();
    },
    [refresh],
  );

  const handleRemove = useCallback(
    (id: string) => {
      todoStorage.remove(id);
      refresh();
    },
    [refresh],
  );

  const filteredTodos =
    filter === "all" ? todos : todos.filter((t) => t.category === filter);

  const completedCount = todos.filter((t) => t.completed).length;

  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col gap-6 p-6 sm:p-8">
      <TodoHeader total={todos.length} completed={completedCount} />

      <div className="flex flex-col gap-4 rounded-xl border border-border bg-card p-4 shadow-sm">
        <TodoInput categories={DEFAULT_CATEGORIES} onAdd={handleAdd} />
        <TodoFilter
          categories={DEFAULT_CATEGORIES}
          activeFilter={filter}
          onFilterChange={setFilter}
        />
      </div>

      <TodoList
        todos={filteredTodos}
        categories={DEFAULT_CATEGORIES}
        onToggle={handleToggle}
        onRemove={handleRemove}
      />
    </main>
  );
}
