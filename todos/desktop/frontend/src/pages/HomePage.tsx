import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import { buttonVariants } from "@/components/ui/button";

type HealthStatus = "loading" | "ok" | "error";

function useHealthCheck() {
  const [status, setStatus] = useState<HealthStatus>("loading");

  useEffect(() => {
    fetch("./api/health")
      .then((res) => {
        setStatus(res.ok ? "ok" : "error");
      })
      .catch(() => {
        setStatus("error");
      });
  }, []);

  return status;
}

const statusColor: Record<HealthStatus, string> = {
  loading: "bg-yellow-400 animate-pulse",
  ok: "bg-green-500",
  error: "bg-red-500",
};

const statusLabel: Record<HealthStatus, string> = {
  loading: "检查中…",
  ok: "服务正常",
  error: "服务异常",
};

export function HomePage() {
  const health = useHealthCheck();

  return (
    <main className="relative flex min-h-screen flex-col items-center justify-center gap-8 p-8">
      <div className="flex flex-col items-center gap-3">
        <h1 className="text-5xl font-bold tracking-tight sm:text-6xl">
          Comate App
        </h1>
      </div>

      <div className="flex items-center gap-2 text-sm">
        <span
          className={`inline-block h-2.5 w-2.5 rounded-full ${statusColor[health]}`}
        />
        <span className="text-muted-foreground">{statusLabel[health]}</span>
      </div>

      <Link
        to="/about"
        className={buttonVariants({ variant: "outline", size: "sm" })}
      >
        About
      </Link>
    </main>
  );
}
