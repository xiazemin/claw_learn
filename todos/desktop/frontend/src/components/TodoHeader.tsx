import { CheckCircle2, Circle } from "lucide-react";

interface TodoHeaderProps {
  total: number;
  completed: number;
}

export function TodoHeader({ total, completed }: TodoHeaderProps) {
  const pending = total - completed;

  return (
    <div className="text-center">
      <h1 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
        ✅ 个人待办
      </h1>
      {total > 0 && (
        <div className="mt-2 flex items-center justify-center gap-4 text-sm text-muted-foreground">
          <span className="flex items-center gap-1">
            <Circle className="size-3.5" />
            待完成 {pending}
          </span>
          <span className="flex items-center gap-1">
            <CheckCircle2 className="size-3.5 text-green-500" />
            已完成 {completed}
          </span>
          <span>共 {total} 项</span>
        </div>
      )}
    </div>
  );
}
