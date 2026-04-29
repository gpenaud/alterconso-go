import api from './client'

export interface Member {
  id: number
  firstName: string
  lastName: string
  email: string
  phone?: string
  balance: number
  isManager: boolean
  address: string
}

export interface MembersResponse {
  members: Member[]
  total: number
  totalPages: number
  page: number
  perPage: number
  waitingListCount: number
}

export function getMembers(groupId: number, page = 1, q?: string) {
  return api
    .get<MembersResponse>(`/groups/${groupId}/members`, {
      params: q ? { page, q } : { page },
    })
    .then((r) => r.data)
}
