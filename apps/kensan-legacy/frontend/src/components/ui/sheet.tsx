import * as React from "react"
import { X } from "lucide-react"

import { cn } from "@/lib/utils"

interface SheetContextValue {
  open: boolean
  setOpen: (open: boolean) => void
}

const SheetContext = React.createContext<SheetContextValue | undefined>(undefined)

interface SheetProps {
  open?: boolean
  onOpenChange?: (open: boolean) => void
  children: React.ReactNode
}

const Sheet = ({ open: controlledOpen, onOpenChange, children }: SheetProps) => {
  const [internalOpen, setInternalOpen] = React.useState(false)
  const open = controlledOpen ?? internalOpen
  const setOpen = onOpenChange ?? setInternalOpen

  return (
    <SheetContext.Provider value={{ open, setOpen }}>
      {children}
    </SheetContext.Provider>
  )
}

const SheetContent = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, children, ...props }, ref) => {
  const context = React.useContext(SheetContext)
  if (!context) throw new Error("SheetContent must be used within Sheet")

  // Handle Escape key
  React.useEffect(() => {
    if (!context.open) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") context.setOpen(false)
    }
    document.addEventListener("keydown", handleKeyDown)
    return () => document.removeEventListener("keydown", handleKeyDown)
  }, [context])

  // Prevent body scroll when open
  React.useEffect(() => {
    if (context.open) {
      document.body.style.overflow = "hidden"
    } else {
      document.body.style.overflow = ""
    }
    return () => {
      document.body.style.overflow = ""
    }
  }, [context.open])

  return (
    <>
      {/* Overlay */}
      <div
        className={cn(
          "fixed inset-0 z-50 bg-black/50 transition-opacity duration-200",
          context.open ? "opacity-100" : "opacity-0 pointer-events-none"
        )}
        onClick={() => context.setOpen(false)}
      />
      {/* Panel */}
      <div
        ref={ref}
        className={cn(
          "fixed inset-y-0 right-0 z-50 flex w-full max-w-lg flex-col border-l bg-background shadow-xl transition-transform duration-200 ease-in-out",
          context.open ? "translate-x-0" : "translate-x-full",
          className
        )}
        {...props}
      >
        {children}
      </div>
    </>
  )
})
SheetContent.displayName = "SheetContent"

const SheetHeader = ({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) => (
  <div
    className={cn(
      "flex items-center gap-2 border-b px-4 py-3",
      className
    )}
    {...props}
  />
)
SheetHeader.displayName = "SheetHeader"

const SheetTitle = React.forwardRef<
  HTMLHeadingElement,
  React.HTMLAttributes<HTMLHeadingElement>
>(({ className, ...props }, ref) => (
  <h2
    ref={ref}
    className={cn(
      "flex-1 text-lg font-semibold leading-none tracking-tight",
      className
    )}
    {...props}
  />
))
SheetTitle.displayName = "SheetTitle"

const SheetClose = React.forwardRef<
  HTMLButtonElement,
  React.ButtonHTMLAttributes<HTMLButtonElement>
>(({ className, ...props }, ref) => {
  const context = React.useContext(SheetContext)
  if (!context) throw new Error("SheetClose must be used within Sheet")

  return (
    <button
      ref={ref}
      className={cn(
        "rounded-sm opacity-70 ring-offset-background transition-opacity hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2",
        className
      )}
      onClick={() => context.setOpen(false)}
      {...props}
    >
      <X className="h-4 w-4" />
      <span className="sr-only">Close</span>
    </button>
  )
})
SheetClose.displayName = "SheetClose"

export {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetClose,
}
