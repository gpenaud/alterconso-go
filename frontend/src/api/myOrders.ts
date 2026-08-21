import api from './client'

export interface MyOrder {
  multiDistribId: number
  date: string
  dateLabel: string
  day: number
  month: string
  place: string
  nbArticles: number
  total: number
  /** Distingue ce qui est joué de ce qui peut encore changer. */
  past: boolean
}

export interface MyOrdersResponse {
  orders: MyOrder[]
  nbOrders: number
  totalYear: number
  yearLabel: number
}

export function fetchMyOrders() {
  return api.get<MyOrdersResponse>('/my-orders').then((r) => r.data)
}
