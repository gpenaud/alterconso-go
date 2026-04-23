import api from './client'

export interface Group {
  id: number
  name: string
  txtIntro?: string
  txtHome?: string
}

export const getGroups = () => api.get<Group[]>('/groups').then((r) => r.data)
export const getGroup = (id: number) => api.get<Group>(`/groups/${id}`).then((r) => r.data)
