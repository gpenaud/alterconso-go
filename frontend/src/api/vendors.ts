import api from './client'

export interface VendorProduct {
  id: number
  name: string
  price: number
  /** Conditionnement en mots : « le kg », « env. 500 g », « la pièce ». */
  unit?: string
  organic: boolean
}

export interface VendorDetail {
  id: number
  name: string
  city?: string
  description?: string
  organic: boolean
  nbProducts: number
  products: VendorProduct[]
}

export function fetchVendor(id: number) {
  return api.get<VendorDetail>(`/vendors/${id}`).then((r) => r.data)
}
