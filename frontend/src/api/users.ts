import api from './client'
import type { User } from './auth'

export const getMe = () => api.get<User>('/users/me').then((r) => r.data)

export const updateMe = (payload: {
  firstName?: string
  lastName?: string
  phone?: string
  address1?: string
  zipCode?: string
  city?: string
}) => api.put<User>('/users/me', payload).then((r) => r.data)
