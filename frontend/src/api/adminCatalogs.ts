import api from './client'

export interface AdminCatalogProduct {
  id: number
  ref?: string
  name: string
  unit?: string
  price: number
  active: boolean
  needsWeighing: boolean
  stockTracked: boolean
  stock?: number
  organic: boolean
}

export interface AdminCatalogDetail {
  catalogId: number
  name: string
  vendorName: string
  products: AdminCatalogProduct[]
}

export function fetchCatalogProducts(id: number) {
  return api.get<AdminCatalogDetail>(`/admin/catalogs/${id}/products`).then((r) => r.data)
}
