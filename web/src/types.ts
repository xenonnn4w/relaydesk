export type TicketStatus = 'new' | 'in_progress' | 'waiting_customer' | 'resolved'
export type Priority = 'urgent' | 'high' | 'normal' | 'low'
export type SLARisk = 'breached' | 'at_risk' | 'healthy' | 'met'

export interface TicketEvent {
  id: string
  type: string
  body: string
  actor: string
  createdAt: string
}

export interface Ticket {
  id: string
  sequence: number
  title: string
  summary: string
  body: string
  source: string
  customer: string
  customerAvatar: string
  status: TicketStatus
  priority: Priority
  category: string
  assignee: string
  confidence: number
  aiProvider: string
  tags: string[]
  createdAt: string
  updatedAt: string
  slaDeadline: string
  firstResponse?: string
  externalIssue?: string
  events: TicketEvent[]
  slaRisk: SLARisk
}
