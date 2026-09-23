import { FormEvent, useEffect, useMemo, useRef, useState } from 'react'
import { ArrowLeft, Bot, CheckCheck, Circle, Clock3, LifeBuoy, Radio, Send, ShieldCheck, Sparkles } from 'lucide-react'
import { api } from './api'
import type { Ticket } from './types'

interface ChatMessage { id: string; body: string; actor: 'customer' | 'agent'; time: string }

function formatTime(value: string) {
  return new Intl.DateTimeFormat('en', { hour: 'numeric', minute: '2-digit' }).format(new Date(value))
}

export function ChatApp() {
  const [name, setName] = useState('')
  const [draft, setDraft] = useState('')
  const [ticket, setTicket] = useState<Ticket | null>(null)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const endRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const savedName = window.localStorage.getItem('relaydesk_customer_name')
    const savedTicketID = window.localStorage.getItem('relaydesk_ticket_id')
    if (savedName) setName(savedName)
    if (savedTicketID) {
      api.getTicket(savedTicketID).then(setTicket).catch(() => window.localStorage.removeItem('relaydesk_ticket_id'))
    }
  }, [])

  useEffect(() => {
    if (!ticket) return
    const interval = window.setInterval(async () => {
      try { setTicket(await api.getTicket(ticket.id)) } catch { /* keep visible state during a transient poll failure */ }
    }, 2000)
    return () => window.clearInterval(interval)
  }, [ticket?.id])

  const messages = useMemo<ChatMessage[]>(() => {
    if (!ticket) return []
    const timeline: ChatMessage[] = [{ id: `${ticket.id}-initial`, body: ticket.body, actor: 'customer', time: ticket.createdAt }]
    for (const event of ticket.events) {
      if (event.type === 'customer_message') timeline.push({ id: event.id, body: event.body, actor: 'customer', time: event.createdAt })
      if (event.type === 'reply') timeline.push({ id: event.id, body: event.body, actor: 'agent', time: event.createdAt })
    }
    return timeline
  }, [ticket])

  useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [messages.length])

  const send = async (event: FormEvent) => {
    event.preventDefault()
    const body = draft.trim()
    if (!body || sending) return
    setSending(true); setError('')
    try {
      const next = ticket
        ? await api.customerMessage(ticket.id, body)
        : await api.createTicket({ body, customer: name.trim() || 'Website visitor', source: 'web' })
      window.localStorage.setItem('relaydesk_ticket_id', next.id)
      window.localStorage.setItem('relaydesk_customer_name', next.customer)
      setTicket(next); setDraft('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Message could not be delivered')
    } finally { setSending(false) }
  }

  return (
    <main className="chat-page">
      <header className="chat-site-header">
        <a className="chat-brand" href="/"><span><Radio size={18} /></span>Relay<em>Desk</em></a>
        <a className="console-link" href="/"><ArrowLeft size={14} /> Agent console</a>
      </header>

      <section className="chat-stage">
        <div className="chat-story">
          <span className="chat-story-label">Human support, accelerated</span>
          <h1>Tell us what’s<br/><i>getting in your way.</i></h1>
          <p>RelayDesk routes your message to the right person, watches response deadlines, and keeps the conversation in one place.</p>
          <div className="trust-row"><span><ShieldCheck size={15} /> Private by default</span><span><Clock3 size={15} /> Typical reply in 8 min</span></div>
          <div className="signal-card"><span className="signal-pulse"><Circle size={7} fill="currentColor" /></span><div><strong>Support is online</strong><small>3 specialists available</small></div><span>IST · LIVE</span></div>
        </div>

        <section className="chat-window" aria-label="Customer support chat">
          <header className="chat-window-header">
            <div className="support-avatar"><LifeBuoy size={20} /></div>
            <div><strong>RelayDesk support</strong><span><i /> Online · replies usually within minutes</span></div>
            {ticket && <span className="chat-ticket-id">#{ticket.sequence}</span>}
          </header>

          <div className="chat-messages">
            <div className="chat-day">Today</div>
            <div className="chat-line agent">
              <span className="mini-avatar"><Bot size={14} /></span>
              <div><p>Hi! Share what you’re working on and where you got stuck. A support specialist will see the full context.</p><time>Now</time></div>
            </div>

            {messages.map((message) => (
              <div className={`chat-line ${message.actor}`} key={message.id}>
                {message.actor === 'agent' && <span className="mini-avatar"><LifeBuoy size={13} /></span>}
                <div><p>{message.body}</p><time>{formatTime(message.time)} {message.actor === 'customer' && <CheckCheck size={12} />}</time></div>
              </div>
            ))}

            {ticket && messages.every((message) => message.actor !== 'agent') && (
              <div className="triage-receipt"><Sparkles size={14} /><div><strong>Request triaged</strong><span>{ticket.category} · {ticket.priority} priority · {Math.round(ticket.confidence * 100)}% confidence</span></div></div>
            )}
            <div ref={endRef} />
          </div>

          <form className="chat-composer" onSubmit={send}>
            {!ticket && <label>Your name<input value={name} onChange={(event) => setName(event.target.value)} placeholder="How should we address you?" /></label>}
            {error && <p className="chat-error">{error}</p>}
            <div className="composer-row"><textarea aria-label="Chat message" value={draft} onChange={(event) => setDraft(event.target.value)} placeholder={ticket ? 'Add to the conversation…' : 'Describe the issue you’re facing…'} onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); event.currentTarget.form?.requestSubmit() } }} /><button aria-label="Send message" disabled={sending || !draft.trim()}><Send size={17} /></button></div>
            <span className="composer-note">By chatting, you agree to share this conversation with the support team.</span>
          </form>
        </section>
      </section>
    </main>
  )
}
