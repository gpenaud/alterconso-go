import api from './client'

export interface TopProduct {
  name: string
  quantity: number
}

export interface DistributionSummary {
  multiDistribId: number
  nbOrders: number
  nbMembers: number
  total: number
  averageOrder: number
  nbVendors: number
  volunteerNeeded: number
  topProducts: TopProduct[]
}

export function fetchDistributionSummary(id: number) {
  return api.get<DistributionSummary>(`/admin/distributions/${id}/summary`).then((r) => r.data)
}
