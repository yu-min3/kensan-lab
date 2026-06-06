import * as React from "react"
import { ChevronDown } from "lucide-react"

import { cn } from "@/lib/utils"

interface SelectContextValue {
  value: string
  onValueChange: (value: string) => void
  open: boolean
  setOpen: (open: boolean) => void
  scrollToSelected: boolean
  registerLabel: (value: string, label: string) => void
  getLabel: (value: string) => string | undefined
  labelsVersion: number
}

const SelectContext = React.createContext<SelectContextValue | undefined>(undefined)

interface SelectProps {
  value?: string
  defaultValue?: string
  onValueChange?: (value: string) => void
  children: React.ReactNode
  scrollToSelected?: boolean
}

const Select = ({ value, defaultValue, onValueChange, children, scrollToSelected = false }: SelectProps) => {
  const [internalValue, setInternalValue] = React.useState(defaultValue || "")
  const [open, setOpen] = React.useState(false)
  const [labelsVersion, setLabelsVersion] = React.useState(0)
  const labelsRef = React.useRef<Map<string, string>>(new Map())
  const currentValue = value ?? internalValue
  const handleValueChange = onValueChange ?? setInternalValue

  const registerLabel = React.useCallback((val: string, label: string) => {
    const existing = labelsRef.current.get(val)
    if (existing !== label) {
      labelsRef.current.set(val, label)
      // Trigger re-render so SelectValue can display the label
      setLabelsVersion(v => v + 1)
    }
  }, [])

  const getLabel = React.useCallback((val: string) => {
    return labelsRef.current.get(val)
  }, [])

  return (
    <SelectContext.Provider value={{ value: currentValue, onValueChange: handleValueChange, open, setOpen, scrollToSelected, registerLabel, getLabel, labelsVersion }}>
      <div className="relative">{children}</div>
    </SelectContext.Provider>
  )
}

const SelectTrigger = React.forwardRef<
  HTMLButtonElement,
  React.ButtonHTMLAttributes<HTMLButtonElement>
>(({ className, children, ...props }, ref) => {
  const context = React.useContext(SelectContext)
  if (!context) throw new Error("SelectTrigger must be used within Select")

  return (
    <button
      ref={ref}
      type="button"
      className={cn(
        "flex h-9 w-full items-center justify-between whitespace-nowrap rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring disabled:cursor-not-allowed disabled:opacity-50 [&>span]:line-clamp-1",
        className
      )}
      onClick={() => context.setOpen(!context.open)}
      {...props}
    >
      {children}
      <ChevronDown className="h-4 w-4 opacity-50" />
    </button>
  )
})
SelectTrigger.displayName = "SelectTrigger"

const SelectValue = ({ placeholder }: { placeholder?: string }) => {
  const context = React.useContext(SelectContext)
  if (!context) throw new Error("SelectValue must be used within Select")

  // Include labelsVersion in dependency to trigger re-render when labels are registered
  const displayValue = React.useMemo(() => {
    return context.value ? (context.getLabel(context.value) || context.value) : placeholder
  }, [context.value, context.labelsVersion, context.getLabel, placeholder])

  return <span>{displayValue}</span>
}

const SelectContent = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, children, ...props }, _ref) => {
  const context = React.useContext(SelectContext)
  const scrollRef = React.useRef<HTMLDivElement>(null)
  if (!context) throw new Error("SelectContent must be used within Select")

  React.useEffect(() => {
    if (context.open && context.scrollToSelected && scrollRef.current) {
      const selectedItem = scrollRef.current.querySelector('[data-selected="true"]')
      if (selectedItem) {
        selectedItem.scrollIntoView({ block: 'center' })
      }
    }
  }, [context.open, context.scrollToSelected])

  // Always render children (hidden when closed) so labels get registered
  return (
    <>
      {context.open && (
        <div className="fixed inset-0 z-40" onClick={() => context.setOpen(false)} />
      )}
      <div
        ref={scrollRef}
        className={cn(
          "absolute z-50 max-h-96 min-w-[8rem] overflow-y-auto rounded-md border bg-popover text-popover-foreground shadow-md top-full mt-1 w-full",
          context.open ? "animate-in fade-in-0 zoom-in-95" : "hidden",
          className
        )}
        {...props}
      >
        <div className="p-1">{children}</div>
      </div>
    </>
  )
})
SelectContent.displayName = "SelectContent"

interface SelectItemProps extends React.HTMLAttributes<HTMLDivElement> {
  value: string
  /** Label to display in SelectValue when selected. If not provided, uses children text content. */
  label?: string
}

const SelectItem = React.forwardRef<HTMLDivElement, SelectItemProps>(
  ({ className, children, value, label, ...props }, ref) => {
    const context = React.useContext(SelectContext)
    if (!context) throw new Error("SelectItem must be used within Select")

    const isSelected = context.value === value

    // Register the label for this value on mount
    React.useLayoutEffect(() => {
      // Use provided label, or extract text from children
      const displayLabel = label || (typeof children === 'string' ? children : undefined)
      if (displayLabel) {
        context.registerLabel(value, displayLabel)
      }
    }, [value, label, children, context.registerLabel])

    return (
      <div
        ref={ref}
        data-selected={isSelected}
        className={cn(
          "relative flex w-full cursor-default select-none items-center rounded-sm py-1.5 pl-2 pr-8 text-sm outline-none hover:bg-accent hover:text-accent-foreground focus:bg-accent focus:text-accent-foreground",
          isSelected && "bg-accent",
          className
        )}
        onClick={() => {
          context.onValueChange(value)
          context.setOpen(false)
        }}
        {...props}
      >
        {children}
      </div>
    )
  }
)
SelectItem.displayName = "SelectItem"

export { Select, SelectContent, SelectItem, SelectTrigger, SelectValue }
