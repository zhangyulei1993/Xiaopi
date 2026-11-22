import type { Property } from '../../api/properties'

let snapshot: { items: Property[]; loadedAt: number } | undefined

export function cacheMapProperties(items: Property[]) {
  snapshot = { items: items.filter(item => item.status === 'available'), loadedAt: Date.now() }
}

export function getMapPropertyCache() {
  return snapshot && Date.now() - snapshot.loadedAt < 30_000 ? snapshot.items : undefined
}
