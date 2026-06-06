import { Outlet } from 'react-router-dom'
import { Header } from './Header'
import { Sidebar } from './Sidebar'
import { FloatingMemoButton } from '@/components/common/FloatingMemoButton'
import { ChatPanel } from '@/components/agent/ChatPanel'
import { DemoTour } from '@/components/guide/DemoTour'
import { useDocumentTitle } from '@/hooks/useDocumentTitle'

export function Layout() {
  // Update document title based on timer state
  useDocumentTitle()

  return (
    <div className="h-screen flex flex-col">
      <Header />
      <div className="flex-1 flex overflow-hidden">
        <Sidebar />
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
        <ChatPanel />
      </div>
      <FloatingMemoButton />
      <DemoTour />
    </div>
  )
}
