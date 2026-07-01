export interface Todo {
  id: string;
  title: string;
  category: string;
  completed: boolean;
  createdAt: number;
}

export interface Category {
  id: string;
  name: string;
  color: string;
}

export const DEFAULT_CATEGORIES: Category[] = [
  { id: "work", name: "工作", color: "bg-blue-100 text-blue-700" },
  { id: "life", name: "生活", color: "bg-green-100 text-green-700" },
  { id: "study", name: "学习", color: "bg-purple-100 text-purple-700" },
  { id: "other", name: "其他", color: "bg-gray-100 text-gray-700" },
];
