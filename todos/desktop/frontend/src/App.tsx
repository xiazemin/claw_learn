import { HashRouter, Routes, Route } from "react-router-dom";
import { Toaster } from "@/components/ui/sonner";
import { TodoPage } from "@/pages/TodoPage";

export function App() {
  return (
    <HashRouter>
      <Routes>
        <Route path="/" element={<TodoPage />} />
      </Routes>
      <Toaster position="top-center" />
    </HashRouter>
  );
}
