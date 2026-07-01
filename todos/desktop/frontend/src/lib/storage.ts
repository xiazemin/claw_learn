import { GetAll, Add, Toggle, Remove } from "../../wailsjs/go/main/App";
import type { Todo } from "../types/todo";

export const todoStorage = {
  async getAll(): Promise<Todo[]> {
    return GetAll();
  },

  async add(title: string, category: string): Promise<Todo> {
    return Add(title, category);
  },

  async toggle(id: string): Promise<void> {
    return Toggle(id);
  },

  async remove(id: string): Promise<void> {
    return Remove(id);
  },
};
