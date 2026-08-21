import api from './client'

export interface MembershipRow {
  userId: number
  name: string
  /** 0 : n'a jamais adhéré — autre chose qu'un retard. */
  lastYear: number
  upToDate: boolean
  nbOrdersThisYear: number
}

export interface MembershipsResponse {
  members: MembershipRow[]
  fee: number
  renewalDate: string
  collectedYear: number
  upToDate: number
  late: number
  year: number
  hasMembership: boolean
}

export function fetchMemberships() {
  return api.get<MembershipsResponse>('/admin/memberships').then((r) => r.data)
}
