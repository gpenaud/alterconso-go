import api from './client'

export interface AdminDistribution {
  id: number
  startAt: string
  dayOfWeek: string
  day: number
  month: string
  startHour: string
  endHour: string
  place: string
  past: boolean
  open: boolean
  orderEndAt?: string
  orderStartLabel?: string
  nbVendors: number
  nbOrders: number
  total: number
  volunteerNeeded: number
}

export function fetchAdminDistributions() {
  return api
    .get<{ distributions: AdminDistribution[] }>('/admin/distributions')
    .then((r) => r.data.distributions)
}
