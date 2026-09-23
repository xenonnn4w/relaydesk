import { FormEvent, useEffect, useMemo, useState } from 'react'
import {
  Activity, AlertTriangle, ArrowUpRight, Bot, Check, CheckCircle2, ChevronDown,
  CircleDot, Clock3, GitBranch, Inbox, Layers3, LifeBuoy, MessageSquareText, MoreHorizontal,
  Plus, Radio, RefreshCw, Search, Send, Settings, ShieldCheck, Sparkles, Tag, Users, X,
} from 'lucide-react'
import { api } from './api'
import type { Priority, SLARisk, Ticket, TicketStatus } from './types'

type QueueFilter = 'all' | 'mine' | 'unassigned' | 'at_risk' | 'resolved'

const statusLabel: Record<TicketStatus, string> = {
  new: 'New', in_progress: 'In progress', waiting_customer: 'Waiting', resolved: 'Resolved',
}

const priorityLabel: Record<Priority, string> = {
  urgent: 'Urgent', high: 'High', normal: 'Normal', low: 'Low',
}

function relativeTime(date: string) {
  const minutes = Math.round((new Date(date).getTime() - Date.now()) / 60000)
  const formatter = new Intl.RelativeTimeFormat('en', { numeric: 'auto' })
  if (Math.abs(minutes) < 60) return formatter.format(minutes, 'minute')
  return formatter.format(Math.round(minutes / 60), 'hour')
}

function slaCopy(ticket: Ticket) {
  if (ticket.slaRisk === 'met') return 'SLA met'
  const minutes = Math.round((new Date(ticket.slaDeadline).getTime() - Date.now()) / 60000)
  if (minutes <= 0) return `${Math.abs(minutes)}m overdue`
  if (minutes < 60) return `${minutes}m left`
  return `${Math.round(minutes / 60)}h left`
}

function SourceMark({ source }: { source: string }) {
  const glyph = source === 'slack' ? 'S' : source === 'discord' ? 'D' : 'W'
  return <span className={`source-mark source-${source}`} aria-label={`${source} source`}>{glyph}</span>
}

function QueueNav({ tickets, active, onChange }: { tickets: Ticket[]; active: QueueFilter; onChange: (value: QueueFilter) => void }) {
  const items: { id: QueueFilter; label: string; icon: typeof Inbox; count?: number }[] = [
    { id: 'all', label: 'All requests', icon: Inbox, count: tickets.filter((ticket) => ticket.status !== 'resolved').length },
    { id: 'mine', label: 'Assigned to me', icon: CircleDot, count: tickets.filter((ticket) => ticket.assignee === 'You' && ticket.status !== 'resolved').length },
    { id: 'unassigned', label: 'Unassigned', icon: Users, count: tickets.filter((ticket) => !ticket.assignee && ticket.status !== 'resolved').length },
    { id: 'at_risk', label: 'SLA attention', icon: AlertTriangle, count: tickets.filter((ticket) => ['at_risk', 'breached'].includes(ticket.slaRisk)).length },
    { id: 'resolved', label: 'Resolved', icon: CheckCircle2 },
  ]
  return (
    <nav className="queue-nav" aria-label="Ticket queues">
      <p className="eyebrow sidebar-label">Queues</p>
      {items.map(({ id, label, icon: Icon, count }) => (
        <button key={id} className={active === id ? 'nav-item active' : 'nav-item'} onClick={() => onChange(id)}>
          <Icon size={16} strokeWidth={1.8} /> <span>{label}</span>
          {count !== undefined && <span className="nav-count">{count}</span>}
        </button>
      ))}
    </nav>
  )
}

function TicketRow({ ticket, selected, onSelect }: { ticket: Ticket; selected: boolean; onSelect: () => void }) {
  return (
    <button className={selected ? 'ticket-row selected' : 'ticket-row'} onClick={onSelect}>
      <div className="ticket-row-top">
        <div className="customer-line"><SourceMark source={ticket.source} /><span>{ticket.customer}</span></div>
        <span className="ticket-time">{relativeTime(ticket.updatedAt)}</span>
      </div>
      <div className="ticket-row-title"><span className={`priority-dot ${ticket.priority}`} />{ticket.title}</div>
      <p>{ticket.summary}</p>
      <div className="ticket-row-meta">
        <span>#{ticket.sequence}</span>
        <span className={`status-pill status-${ticket.status}`}>{statusLabel[ticket.status]}</span>
        <span className={`sla-pill sla-${ticket.slaRisk}`}><Clock3 size={12} />{slaCopy(ticket)}</span>
      </div>
    </button>
  )
}

function StatCard({ label, value, note, tone, icon: Icon }: { label: string; value: string | number; note: string; tone: string; icon: typeof Inbox }) {
  return (
    <article className={`stat-card ${tone}`}>
      <div className="stat-icon"><Icon size={18} /></div>
      <div><p>{label}</p><strong>{value}</strong><span>{note}</span></div>
    </article>
  )
}

function TicketDetail({ ticket, busy, onUpdate, onReply, onEscalate }: {
  ticket: Ticket; busy: boolean
  onUpdate: (update: { status?: TicketStatus; priority?: Priority; assignee?: string }) => void
  onReply: (body: string) => void; onEscalate: () => void
}) {
  const [reply, setReply] = useState('')
  useEffect(() => setReply(''), [ticket.id])
  const submitReply = (event: FormEvent) => {
    event.preventDefault()
    if (!reply.trim()) return
    onReply(reply.trim())
    setReply('')
  }
  return (
    <section className="ticket-detail">
      <header className="detail-header">
        <div className="detail-heading">
          <div className="detail-kicker"><span>#{ticket.sequence}</span><span>·</span><SourceMark source={ticket.source} /><span>{ticket.source}</span></div>
          <h2>{ticket.title}</h2>
        </div>
        <button className="icon-button" aria-label="More ticket actions"><MoreHorizontal size={20} /></button>
      </header>

      <div className="ticket-controls">
        <label>Status<select value={ticket.status} disabled={busy} onChange={(event) => onUpdate({ status: event.target.value as TicketStatus })}>
          <option value="new">New</option><option value="in_progress">In progress</option><option value="waiting_customer">Waiting on customer</option><option value="resolved">Resolved</option>
        </select><ChevronDown size={14} /></label>
        <label>Priority<select value={ticket.priority} disabled={busy} onChange={(event) => onUpdate({ priority: event.target.value as Priority })}>
          <option value="urgent">Urgent</option><option value="high">High</option><option value="normal">Normal</option><option value="low">Low</option>
        </select><ChevronDown size={14} /></label>
        <label>Assignee<select value={ticket.assignee || ''} disabled={busy} onChange={(event) => onUpdate({ assignee: event.target.value })}>
          <option value="">Unassigned</option><option value="You">You</option><option value="Priya">Priya</option><option value="Ishan">Ishan</option>
        </select><ChevronDown size={14} /></label>
      </div>

      <div className="detail-scroll">
        <article className="message-card">
          <div className="message-author"><span className="avatar">{ticket.customerAvatar}</span><div><strong>{ticket.customer}</strong><span>via {ticket.source} · {relativeTime(ticket.createdAt)}</span></div></div>
          <p>{ticket.body}</p>
        </article>

        <article className="ai-card">
          <div className="ai-card-heading"><span><Sparkles size={15} /> AI triage · {ticket.aiProvider || 'local'}</span><small>{Math.round(ticket.confidence * 100)}% confidence</small></div>
          <p>{ticket.summary}</p>
          <div className="ai-labels"><span><Tag size={12} /> {ticket.category}</span>{ticket.tags.map((tag) => <span key={tag}>{tag}</span>)}</div>
        </article>

        {ticket.externalIssue ? (
          <button className="issue-card" onClick={onEscalate}><span className="issue-icon"><GitBranch size={19} /></span><span><small>Linked engineering issue</small><strong>{ticket.externalIssue} · {ticket.title}</strong></span><ArrowUpRight size={16} /></button>
        ) : (
          <button className="escalate-button" disabled={busy} onClick={onEscalate}><GitBranch size={16} /> Escalate to engineering <ArrowUpRight size={14} /></button>
        )}

        <div className="timeline">
          <div className="section-heading"><span>Activity</span><span>{ticket.events.length} events</span></div>
          {[...ticket.events].reverse().map((event) => (
            <div className="timeline-event" key={event.id}>
              <span className={`timeline-dot event-${event.type}`} />
              <div><p>{event.body}</p><span>{event.actor} · {relativeTime(event.createdAt)}</span></div>
            </div>
          ))}
        </div>
      </div>

      <form className="reply-box" onSubmit={submitReply}>
        <div className="reply-toolbar"><button type="button" className="reply-mode"><MessageSquareText size={14} /> Public reply <ChevronDown size={12} /></button><span>AI suggestions on</span></div>
        <textarea value={reply} onChange={(event) => setReply(event.target.value)} placeholder={`Reply to ${ticket.customer}…`} aria-label="Reply message" />
        <div className="reply-actions"><span>⌘ + Enter to send</span><button disabled={busy || !reply.trim()}><Send size={14} /> Send reply</button></div>
      </form>
    </section>
  )
}

function NewRequestDialog({ open, onClose, onCreate, busy }: { open: boolean; onClose: () => void; onCreate: (input: { body: string; source: string; customer: string }) => void; busy: boolean }) {
  const [customer, setCustomer] = useState('Demo Customer')
  const [source, setSource] = useState('web')
  const [body, setBody] = useState('')
  if (!open) return null
  const submit = (event: FormEvent) => {
    event.preventDefault()
    if (!body.trim()) return
    onCreate({ customer, source, body })
    setBody('')
  }
  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <form className="dialog" onSubmit={submit}>
        <div className="dialog-header"><div><span className="eyebrow">Ingestion simulator</span><h2>Inject a support request</h2></div><button type="button" className="icon-button" onClick={onClose}><X size={18} /></button></div>
        <p className="dialog-copy">This exercises the same API path a verified Slack or Discord webhook would use.</p>
        <div className="form-grid"><label>Customer<input value={customer} onChange={(event) => setCustomer(event.target.value)} /></label><label>Source<select value={source} onChange={(event) => setSource(event.target.value)}><option value="web">Web chat</option><option value="slack">Slack</option><option value="discord">Discord</option></select></label></div>
        <label>Message<textarea autoFocus value={body} onChange={(event) => setBody(event.target.value)} placeholder="Example: Production webhooks are returning 502s and order updates are blocked." /></label>
        <div className="dialog-footer"><span><Bot size={14} /> AI will classify this request</span><button disabled={busy || !body.trim()}>{busy ? <RefreshCw className="spin" size={15} /> : <Plus size={15} />} Create request</button></div>
      </form>
    </div>
  )
}

export function App() {
  const [tickets, setTickets] = useState<Ticket[]>([])
  const [selectedID, setSelectedID] = useState('')
  const [filter, setFilter] = useState<QueueFilter>('all')
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [error, setError] = useState('')

  const load = async () => {
    try {
      const next = await api.listTickets()
      setTickets(next)
      setSelectedID((current) => current || next[0]?.id || '')
      setError('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not reach the API')
    } finally { setLoading(false) }
  }
  useEffect(() => { void load() }, [])

  const filtered = useMemo(() => tickets.filter((ticket) => {
    const matchesSearch = `${ticket.title} ${ticket.customer} ${ticket.sequence}`.toLowerCase().includes(search.toLowerCase())
    if (!matchesSearch) return false
    if (filter === 'mine') return ticket.assignee === 'You' && ticket.status !== 'resolved'
    if (filter === 'unassigned') return !ticket.assignee && ticket.status !== 'resolved'
    if (filter === 'at_risk') return ['at_risk', 'breached'].includes(ticket.slaRisk)
    if (filter === 'resolved') return ticket.status === 'resolved'
    return ticket.status !== 'resolved'
  }), [tickets, filter, search])

  useEffect(() => {
    if (filtered.length && !filtered.some((ticket) => ticket.id === selectedID)) setSelectedID(filtered[0].id)
  }, [filtered, selectedID])
  const selected = tickets.find((ticket) => ticket.id === selectedID)
  const replaceTicket = (next: Ticket) => setTickets((current) => current.map((ticket) => ticket.id === next.id ? { ...next, slaRisk: ticket.slaRisk } : ticket))
  const perform = async (action: () => Promise<Ticket>) => {
    setBusy(true)
    try { replaceTicket(await action()); setError('') } catch (reason) { setError(reason instanceof Error ? reason.message : 'Action failed') } finally { setBusy(false) }
  }
  const create = async (input: { body: string; source: string; customer: string }) => {
    setBusy(true)
    try {
      const ticket = await api.createTicket(input)
      const enriched = { ...ticket, slaRisk: 'healthy' as SLARisk }
      setTickets((current) => [enriched, ...current]); setSelectedID(ticket.id); setFilter('all'); setDialogOpen(false); setError('')
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Could not create ticket') } finally { setBusy(false) }
  }

  const openCount = tickets.filter((ticket) => ticket.status !== 'resolved').length
  const riskCount = tickets.filter((ticket) => ['at_risk', 'breached'].includes(ticket.slaRisk)).length
  return (
    <main className="app-shell">
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark"><Radio size={19} /></span><span>Relay<em>Desk</em></span></div>
        <button className="new-request" onClick={() => setDialogOpen(true)}><Plus size={16} /> New request <kbd>N</kbd></button>
        <QueueNav tickets={tickets} active={filter} onChange={setFilter} />
        <div className="sidebar-rule" />
        <div className="channels"><p className="eyebrow sidebar-label">Live sources</p><div><SourceMark source="slack" /><span>Slack</span><i /></div><div><SourceMark source="web" /><span>Web chat</span><i /></div><div><SourceMark source="discord" /><span>Discord</span><i /></div></div>
        <div className="sidebar-spacer" />
        <div className="system-status"><ShieldCheck size={15} /><span><strong>All systems nominal</strong><small>3 sources connected</small></span></div>
        <button className="user-card"><span className="avatar avatar-user">YS</span><span><strong>Yusii</strong><small>Support engineer</small></span><Settings size={15} /></button>
      </aside>

      <section className="workspace">
        <header className="workspace-header">
          <div><span className="eyebrow">RelayDesk / Live queue</span><h1>Support operations.</h1></div>
          <div className="header-actions"><a className="chat-link" href="/chat" target="_blank" rel="noreferrer"><MessageSquareText size={14} /> Open web chat</a><button className="sync-button" onClick={load} disabled={loading}><RefreshCw size={14} className={loading ? 'spin' : ''} /> Synced now</button><button className="icon-button"><Activity size={18} /></button></div>
        </header>

        <section className="stats-grid">
          <StatCard label="Open requests" value={openCount} note="Across 3 sources" tone="ink" icon={Inbox} />
          <StatCard label="SLA attention" value={riskCount} note="Needs action now" tone="ember" icon={AlertTriangle} />
          <StatCard label="Median response" value="8m" note="↓ 18% this week" tone="sage" icon={Clock3} />
          <StatCard label="AI auto-triage" value="94%" note="27 of 29 requests" tone="blue" icon={Sparkles} />
        </section>

        {error && <div className="error-banner"><AlertTriangle size={15} /><span>{error}. Start the Go API on port 8080.</span><button onClick={load}>Retry</button></div>}

        <section className="triage-board">
          <div className="ticket-list">
            <div className="list-toolbar"><div><h2>{filter.replace('_', ' ')}</h2><span>{filtered.length} requests</span></div><label className="search"><Search size={15} /><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search requests" /></label></div>
            <div className="list-sort"><span>Sorted by <strong>recent activity</strong></span><button>View <Layers3 size={14} /></button></div>
            <div className="ticket-list-scroll">
              {loading ? <div className="empty-state"><RefreshCw className="spin" /><p>Loading the queue…</p></div> : filtered.length ? filtered.map((ticket, index) => <div className="stagger-item" style={{ animationDelay: `${index * 55}ms` }} key={ticket.id}><TicketRow ticket={ticket} selected={ticket.id === selectedID} onSelect={() => setSelectedID(ticket.id)} /></div>) : <div className="empty-state"><Check size={24} /><p>No requests in this queue.</p></div>}
            </div>
          </div>
          {selected ? <TicketDetail ticket={selected} busy={busy} onUpdate={(input) => perform(() => api.updateTicket(selected.id, input))} onReply={(body) => perform(() => api.reply(selected.id, body))} onEscalate={() => perform(() => api.escalate(selected.id))} /> : <div className="no-selection"><LifeBuoy size={30} /><h2>Select a request</h2><p>Choose a conversation from the queue to inspect its context and activity.</p></div>}
        </section>
      </section>
      <NewRequestDialog open={dialogOpen} onClose={() => setDialogOpen(false)} onCreate={create} busy={busy} />
    </main>
  )
}
