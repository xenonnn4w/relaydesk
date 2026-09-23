import type { Priority, Ticket, TicketStatus } from './types'

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  })
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: 'Request failed' }))
    throw new Error(body.error || `Request failed (${response.status})`)
  }
  return response.json()
}

export const api = {
  listTickets: () => request<Ticket[]>('/api/tickets'),
  getTicket: (id: string) => request<Ticket>(`/api/tickets/${id}`),
  createTicket: (input: { body: string; source: string; customer: string }) =>
    request<Ticket>('/api/intake', { method: 'POST', body: JSON.stringify(input) }),
  updateTicket: (id: string, input: { status?: TicketStatus; priority?: Priority; assignee?: string }) =>
    request<Ticket>(`/api/tickets/${id}`, { method: 'PATCH', body: JSON.stringify(input) }),
  reply: (id: string, body: string) =>
    request<Ticket>(`/api/tickets/${id}/reply`, { method: 'POST', body: JSON.stringify({ body }) }),
  customerMessage: (id: string, body: string) =>
    request<Ticket>(`/api/tickets/${id}/customer-message`, { method: 'POST', body: JSON.stringify({ body }) }),
  escalate: (id: string) => request<Ticket>(`/api/tickets/${id}/escalate`, { method: 'POST' }),
}
