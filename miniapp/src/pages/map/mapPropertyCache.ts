import type { Property } from '../../api/properties'

let snapshot: { items: Property[]; loadedAt: number; confirmed: boolean } | undefined

export function cacheMapProperties(items: Property[], confirmed = false) {
  if (!confirmed && snapshot?.confirmed && getMapPropertyCache()) return
  snapshot = { items: items.filter(item => item.status === 'available'), loadedAt: Date.now(), confirmed }
}

export function getMapPropertyCache() {
  return snapshot && Date.now() - snapshot.loadedAt < 30_000 ? snapshot.items : undefined
}
