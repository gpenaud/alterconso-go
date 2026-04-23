import api from './client'

export interface Operation {
  id: number
  date: string
  amount: number
  type: string
  description?: string
  pending: boolean
  paymentType?: string
}

export const getBalance = (groupId: number) =>
  api.get<{ balance: number }>(`/groups/${groupId}/balance`).then((r) => r.data.balance)

export const getOperations = (groupId: number, limit = 50) =>
  api.get<{ operations: Operation[] }>(`/groups/${groupId}/operations`, { params: { limit } })
    .then((r) => r.data.operations)
