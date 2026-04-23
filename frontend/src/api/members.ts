import api from './client'

export interface Member {
  id: number
  firstName: string
  lastName: string
  email: string
  phone?: string
  balance: number
  rights: string[]
}

export const getMembers = (groupId: number) =>
  api.get<{ members: Member[] }>(`/groups/${groupId}/members`).then((r) => r.data.members)
