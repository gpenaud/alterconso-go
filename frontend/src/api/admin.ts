import api from './client'

export interface MemberBalance {
  userId: number
  name: string
  email: string
  balance: number
}

export interface GroupFinances {
  members: MemberBalance[]
  totalDebt: number
  totalCredit: number
  pendingCount: number
  pendingSum: number
}

export const getGroupFinances = (groupId: number) =>
  api.get<GroupFinances>(`/groups/${groupId}/finances`).then((r) => r.data)

export const validateDistribution = (distribId: number) =>
  api.post(`/distributions/${distribId}/validate`)
