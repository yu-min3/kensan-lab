/**
 * Array manipulation utilities for reordering and moving items
 */

/**
 * Reorder items in an array by moving an item from one index to another
 * @param items - The array to reorder
 * @param fromIndex - The index of the item to move
 * @param toIndex - The index to move the item to
 * @returns A new array with the item moved
 */
export function reorderByIndex<T>(items: T[], fromIndex: number, toIndex: number): T[] {
  const result = [...items]
  const [removed] = result.splice(fromIndex, 1)
  result.splice(toIndex, 0, removed)
  return result
}

/**
 * Reorder items based on a new order of IDs
 * @param items - The array of items with id property
 * @param newOrder - Array of IDs in the desired order
 * @returns A new array ordered according to newOrder
 */
export function reorderByIds<T extends { id: string }>(items: T[], newOrder: string[]): T[] {
  const itemMap = new Map(items.map((item) => [item.id, item]))
  return newOrder.map((id) => itemMap.get(id)).filter((item): item is T => item !== undefined)
}

/**
 * Move an item by its ID to a specific index
 * @param items - The array of items with id property
 * @param itemId - The ID of the item to move
 * @param toIndex - The target index
 * @returns A new array with the item moved, or the original if item not found
 */
export function moveItemById<T extends { id: string }>(
  items: T[],
  itemId: string,
  toIndex: number
): T[] {
  const fromIndex = items.findIndex((item) => item.id === itemId)
  if (fromIndex === -1) return items
  return reorderByIndex(items, fromIndex, toIndex)
}

/**
 * Update sort order property for items based on their array index
 * @param items - The array of items with sortOrder property
 * @returns A new array with updated sortOrder values
 */
export function updateSortOrders<T extends { sortOrder: number }>(items: T[]): T[] {
  return items.map((item, index) => ({ ...item, sortOrder: index }))
}

/**
 * Insert an item at a specific index
 * @param items - The array to insert into
 * @param item - The item to insert
 * @param index - The index to insert at
 * @returns A new array with the item inserted
 */
export function insertAt<T>(items: T[], item: T, index: number): T[] {
  const result = [...items]
  result.splice(index, 0, item)
  return result
}

/**
 * Remove an item at a specific index
 * @param items - The array to remove from
 * @param index - The index of the item to remove
 * @returns A new array with the item removed
 */
export function removeAt<T>(items: T[], index: number): T[] {
  const result = [...items]
  result.splice(index, 1)
  return result
}

/**
 * Remove an item by its ID
 * @param items - The array of items with id property
 * @param id - The ID of the item to remove
 * @returns A new array with the item removed
 */
export function removeById<T extends { id: string }>(items: T[], id: string): T[] {
  return items.filter((item) => item.id !== id)
}
