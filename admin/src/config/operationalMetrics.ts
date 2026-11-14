export const isPendingLead = (item: { status: string }) => ['new', 'inspection_pending'].includes(item.status)
export const isNewRentalRequest = (item: { status: string }) => ['new', 'pending'].includes(item.status)
