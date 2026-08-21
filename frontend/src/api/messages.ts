import api from './client'

export interface RecipientOption {
  value: string
  name: string
  count: number
}

export function fetchRecipients() {
  return api.get<{ recipients: RecipientOption[] }>('/messages/recipients').then((r) => r.data.recipients)
}

export function sendMessage(payload: { recipients: string; subject: string; body: string }) {
  return api.post<{ sent: number; failed: number }>('/messages', payload).then((r) => r.data)
}
