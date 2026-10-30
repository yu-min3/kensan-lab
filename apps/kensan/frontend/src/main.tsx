import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AppShell } from "./components/AppShell";
import { ToastProvider } from "./components/ui/toast";
import { MemoPage } from "./pages/MemoPage";
import { ReviewsPage } from "./pages/ReviewsPage";
import { LifeGoalsPage } from "./pages/LifeGoalsPage";
import { MarkportRedirect } from "./pages/MarkportRedirect";
import { WatchPage } from "./pages/WatchPage";
import { HomePage } from "./pages/HomePage";
import { HomeRedirect } from "./pages/HomeRedirect";
import { NotFoundPage } from "./pages/NotFoundPage";
import { DailyPage } from "./pages/DailyPage";
import { FeedPage } from "./pages/FeedPage";
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
              <Route path="/" element={<HomePage />} />
              <Route path="/watch" element={<WatchPage />} />
              <Route path="/today" element={<HomeRedirect panel="activity" />} />
              <Route path="/tasks" element={<HomeRedirect panel="tasks" />} />
              <Route path="/projects" element={<HomeRedirect panel="projects" />} />
              <Route path="/feed" element={<FeedPage />} />
              <Route path="/daily" element={<DailyPage />} />
              <Route path="/memos" element={<MemoPage />} />
              <Route path="/life" element={<LifeGoalsPage />} />
              <Route path="/reviews" element={<ReviewsPage />} />
              {/* 旧ノート・ビューアの URL は markport へ案内する */}
              <Route path="/notes" element={<MarkportRedirect />} />
              <Route path="/view" element={<MarkportRedirect />} />
              {/* 未定義ルートの受け皿（白画面防止） */}
              <Route path="*" element={<NotFoundPage />} />
            </Routes>
          </AppShell>
        </ToastProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
);
