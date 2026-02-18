import {
  CalendarDays,
  BookOpen,
  FileText,
  BookOpenCheck,
  StickyNote,
  Newspaper,
  NotebookPen,
  Lightbulb,
  type LucideIcon,
} from 'lucide-react'

/**
 * Map of Lucide icon names to icon components.
 * Add new icons here when new note types are created.
 */
const ICON_MAP: Record<string, LucideIcon> = {
  'calendar-days': CalendarDays,
  'book-open': BookOpen,
  'file-text': FileText,
  'book-open-check': BookOpenCheck,
  'sticky-note': StickyNote,
  newspaper: Newspaper,
  'notebook-pen': NotebookPen,
  lightbulb: Lightbulb,
}

/**
 * Get a Lucide icon component by its name string.
 * Falls back to FileText if the name is not found.
 */
export function getNoteTypeIcon(iconName: string): LucideIcon {
  return ICON_MAP[iconName] ?? FileText
}
