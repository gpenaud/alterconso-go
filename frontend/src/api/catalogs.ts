import api from './client'

export interface Product {
  id: number
  name: string
  price: number
  unitLabel?: string
  stock?: number
}

export interface Catalog {
  id: number
  name: string
  type: number
  feesRate: number
  vendor?: { id: number; name: string }
  products: Product[]
}

export const getCatalogs = (groupId: number) =>
  api.get<Catalog[]>(`/groups/${groupId}/catalogs`).then((r) => r.data)

export const getCatalog = (id: number) =>
  api.get<Catalog>(`/catalogs/${id}`).then((r) => r.data)
