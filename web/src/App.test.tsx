import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'

const ticket = {
  id: 'tkt_test', sequence: 1001, title: 'Webhook deliveries are failing',
  summary: 'Production webhook deliveries are returning errors.', body: 'Production is down.',
  source: 'slack', customer: 'Maya Chen', customerAvatar: 'MC', status: 'new',
  priority: 'urgent', category: 'incident', assignee: '', confidence: 0.94,
  tags: ['incident'], createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(),
  slaDeadline: new Date(Date.now() + 20 * 60_000).toISOString(), events: [], slaRisk: 'at_risk',
}

describe('RelayDesk dashboard', () => {
  afterEach(() => vi.restoreAllMocks())

  it('loads the triage queue and opens the first ticket', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => [ticket] }))
    render(<App />)
    expect(await screen.findByRole('heading', { name: ticket.title })).toBeInTheDocument()
    expect(screen.getAllByText('Maya Chen')).toHaveLength(2)
    expect(screen.getByText('94% confidence')).toBeInTheDocument()
  })
})
