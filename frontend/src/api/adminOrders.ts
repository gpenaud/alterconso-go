import api from './client'

export interface AdminOrderLine {
  userId: number
  userName: string
  vendorId: number
  vendorName: string
  productRef?: string
  product: string
  quantity: number
  unitPrice: number
  total: number
  /** Le prix se fixe à la pesée : le tableau serait faux sans cette mention. */
  needsWeighing: boolean
  weighed: boolean
}

export function fetchDistributionOrders(id: number) {
  return api
    .get<{ lines: AdminOrderLine[]; total: number }>(`/admin/distributions/${id}/orders`)
    .then((r) => r.data)
}
