import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AppShell } from "./components/AppShell";
import { ToastProvider } from "./components/ui/toast";
import { Dashboard } from "./pages/Dashboard";
import { DailyPage } from "./pages/DailyPage";
import { MemoPage } from "./pages/MemoPage";
import { TasksPage } from "./pages/TasksPage";
import { ProjectsPage } from "./pages/ProjectsPage";
import { ReviewsPage } from "./pages/ReviewsPage";
import { NotesPage } from "./pages/NotesPage";
import { LifeGoalsPage } from "./pages/LifeGoalsPage";
import { NotFoundPage } from "./pages/NotFoundPage";
import "./index.css";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 15_000, refetchOnWindowFocus: true },
  },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <ToastProvider>
          <AppShell>
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/tasks" element={<TasksPage />} />
              <Route path="/projects" element={<ProjectsPage />} />
              <Route path="/daily" element={<DailyPage />} />
              <Route path="/memos" element={<MemoPage />} />
              <Route path="/notes" element={<NotesPage />} />
              <Route path="/life" element={<LifeGoalsPage />} />
              <Route path="/reviews" element={<ReviewsPage />} />
              {/* 未定義ルートの受け皿（白画面防止） */}
              <Route path="*" element={<NotFoundPage />} />
            </Routes>
          </AppShell>
        </ToastProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
);
