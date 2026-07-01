import { Todo } from "../types/todo";

const STORAGE_KEY = "personal-todos";

export const todoStorage = {
  getAll(): Todo[] {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      return raw ? JSON.parse(raw) : [];
    } catch {
      return [];
    }
  },

  save(todos: Todo[]): void {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(todos));
  },

  add(title: string, category: string): Todo {
    const todos = this.getAll();
    const newTodo: Todo = {
      id: crypto.randomUUID(),
      title,
      category,
      completed: false,
      createdAt: Date.now(),
    };
    this.save([newTodo, ...todos]);
    return newTodo;
  },

  toggle(id: string): void {
    const todos = this.getAll().map((t) =>
      t.id === id ? { ...t, completed: !t.completed } : t,
    );
    this.save(todos);
  },

  remove(id: string): void {
    this.save(this.getAll().filter((t) => t.id !== id));
  },
};
