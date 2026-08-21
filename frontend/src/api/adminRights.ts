import api from './client'

export interface RightHolder {
  userId: number
  name: string
  email: string
  /** « Responsable de groupe », « Responsable technique », ou absent. */
  role?: string
  delegations: string[]
  /** Faux pour le responsable technique : son rôle vient de la configuration. */
  editable: boolean
}

export function fetchRightHolders() {
  return api.get<{ holders: RightHolder[] }>('/admin/rights').then((r) => r.data.holders)
}
