import { create } from 'zustand'
import { persist } from 'zustand/middleware'

// DEV: demoTourCompletedUsers はユーザーごとにツアー完了状態を保持する開発用の仕組み。
// 同一オリジンで複数ユーザーを切り替えてテストする際に localStorage が共有される問題を回避する。
// 本番で不要になったら boolean の demoTourCompleted に戻して良い。
interface GuideState {
  dismissedCards: Record<string, boolean>
  completedTours: Record<string, boolean>
  isDemoTourActive: boolean
  /** @deprecated 後方互換用。demoTourCompletedUsers を参照 */
  demoTourCompleted?: boolean
  demoTourCompletedUsers: Record<string, boolean>
  dismissCard: (pageId: string) => void
  completeTour: (pageId: string) => void
  startDemoTour: () => void
  completeDemoTour: (userId: string) => void
  skipDemoTour: (userId: string) => void
  resetAll: () => void
  isCardDismissed: (pageId: string) => boolean
  isTourCompleted: (pageId: string) => boolean
  isDemoTourCompletedFor: (userId: string) => boolean
}

export const useGuideStore = create<GuideState>()(
  persist(
    (set, get) => ({
      dismissedCards: {},
      completedTours: {},
      isDemoTourActive: false,
      demoTourCompletedUsers: {},
      dismissCard: (pageId) =>
        set((state) => ({
          dismissedCards: { ...state.dismissedCards, [pageId]: true },
        })),
      completeTour: (pageId) =>
        set((state) => ({
          completedTours: { ...state.completedTours, [pageId]: true },
        })),
      startDemoTour: () => set({ isDemoTourActive: true }),
      completeDemoTour: (userId) =>
        set((state) => ({
          isDemoTourActive: false,
          demoTourCompletedUsers: { ...state.demoTourCompletedUsers, [userId]: true },
        })),
      skipDemoTour: (userId) =>
        set((state) => ({
          isDemoTourActive: false,
          demoTourCompletedUsers: { ...state.demoTourCompletedUsers, [userId]: true },
        })),
      resetAll: () => set({ dismissedCards: {}, completedTours: {}, demoTourCompletedUsers: {}, isDemoTourActive: false }),
      isCardDismissed: (pageId) => !!get().dismissedCards[pageId],
      isTourCompleted: (pageId) => !!get().completedTours[pageId],
      isDemoTourCompletedFor: (userId) => !!get().demoTourCompletedUsers[userId],
    }),
    {
      name: 'kensan-guide',
      partialize: (state) => ({
        dismissedCards: state.dismissedCards,
        completedTours: state.completedTours,
        demoTourCompletedUsers: state.demoTourCompletedUsers,
      }),
    }
  )
)
