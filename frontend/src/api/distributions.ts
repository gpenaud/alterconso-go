import api from './client'

export interface Distribution {
  id: number
  catalogId: number
  catalog: { id: number; name: string }
}

export interface MultiDistrib {
  id: number
  distribStartDate: string
  distribEndDate: string
  orderStartDate?: string
  orderEndDate?: string
  validated: boolean
  place: { id: number; name: string }
  distributions: Distribution[]
}

export const getDistributions = (groupId: number) =>
  api.get<MultiDistrib[]>(`/groups/${groupId}/distributions`)
    .then((r) => r.data)
