import api from './client'

export interface AdminMemberOrder {
  multiDistribId: number
  date: string
  dateLabel: string
  summary: string
  total: number
  past: boolean
  /** N'a de sens que pour une distribution passée. */
  delivered: boolean
}

export interface AdminMemberDetail {
  userId: number
  name: string
  email: string
  phone?: string
  address?: string
  balance: number
  memberSince?: string
  membershipYear: number
  membershipFee: number
  membershipUpToDate: boolean
  role?: string
  delegations: string[]
  nbOrdersThisYear: number
  totalThisYear: number
  nbVolunteering: number
  orders: AdminMemberOrder[]
}

export function fetchMember(id: number) {
  return api.get<AdminMemberDetail>(`/admin/members/${id}`).then((r) => r.data)
}
