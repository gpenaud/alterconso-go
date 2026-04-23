import api from './client'

export interface Order {
  id: number
  userId: number
  userName: string
  productId: number
  productName: string
  productPrice: number
  quantity: number
  feesRate: number
  fees: number
  subTotal: number
  total: number
  paid: boolean
  catalogId: number
  catalogName: string
  canModify: boolean
}

export interface OrderData {
  productId: number
  quantity: number
}

export const getOrders = (distributionId: number, catalogId?: number, userId?: number) =>
  api.get<{ orders: Order[] }>('/orders', {
    params: { distributionId, catalogId, userId },
  }).then((r) => r.data.orders)

export const saveOrders = (payload: {
  distributionId: number
  catalogId?: number
  userId?: number
  orders: OrderData[]
}) => api.post<{ orders: Order[] }>('/orders', payload).then((r) => r.data.orders)
