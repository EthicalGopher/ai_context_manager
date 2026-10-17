import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { apiMock, resetApi } from '../test/mockApi'

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<any>('../lib/api')
  return { ...actual, api: apiMock, onEvent: () => () => {} }
})
vi.mock('../../wailsjs/runtime/runtime', () => ({ OnFileDrop: vi.fn(), OnFileDropOff: vi.fn() }))
vi.mock('../components/TerminalPane', () => ({ TerminalPane: (p: any) => <div data-testid="terminal-pane" data-autostart={String(!!p.autoStart)} /> }))

import Chat, { TERMINAL_WARMUP } from './Chat'
import { useStore } from '../store'

const ws = { id: 1, path: '/p', name: 'shop', agent: '', settings: '{}', lastOpened: '' }
const conv = { id: 5, workspaceId: 1, title: 'Fix login', agent: 'claude', sessionId: 0, messages: 2, updatedAt: new Date().toISOString() }
const msg = (o: any) => ({ id: 1, role: 'user', source: 'ui', body: 'hi', createdAt: '', media: [], ...o })

const renderAt = (path: string) => render(<MemoryRouter initialEntries={[path]}><Routes><Route path="/" element={<Chat />} /><Route path="/c/:id" element={<Chat />} /></Routes></MemoryRouter>)

beforeEach(() => {
  resetApi(); localStorage.clear(); localStorage.setItem('aicw-target', 'chat'); localStorage.setItem('aicw-term-open', '0'); TERMINAL_WARMUP.ms = 0
  useStore.setState({ open: { workspace: ws, sessions: [], resumable: [], action: 'new', isNew: false, requestedNew: false, reason: '' } as any, convs: [conv] })
  apiMock.ListAgents.mockResolvedValue([{ profile: { id: 'claude', displayName: 'Claude Code' }, status: { state: 'Detected' } }])
  apiMock.Conversations.mockResolvedValue([conv])
  apiMock.TermStatus.mockResolvedValue(null)
  apiMock.ChatStatus.mockResolvedValue({ running: false, queued: 0 })
  apiMock.GetSettings.mockResolvedValue({ chatEdits: false, autoTerminal: true })
  apiMock.SetChatEdits.mockResolvedValue(undefined)
  apiMock.StopChat.mockResolvedValue(undefined)
  apiMock.VoiceStatus.mockResolvedValue({ ready: false, hint: '' })
})

describe('Save to page', () => {
  it('saves a message (with attachments) to a new or existing page after choosing a title', async () => {
    apiMock.Messages.mockResolvedValue([msg({ id: 1, role: 'assistant', source: 'agent', body: 'Final pig design.' })])
    apiMock.ListPages.mockResolvedValue([{ title: 'Character' }])
    apiMock.SaveMessageToPage.mockResolvedValue({ title: 'Character' })
    renderAt('/c/5')
    await userEvent.click(await screen.findByRole('button', { name: /save to page/i }))
    await userEvent.type(await screen.findByLabelText('Page title'), 'Character')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(apiMock.SaveMessageToPage).toHaveBeenCalledWith(1, 'Character'))
    expect(await screen.findByText(/Saved to the page/)).toBeInTheDocument()
  })
})

describe('Chat replies', () => {
  it('shows a working indicator with Stop while the agent is replying, then the reply', async () => {
    apiMock.Messages.mockResolvedValue([msg({ id: 1 })])
    apiMock.ChatStatus.mockResolvedValue({ running: true, queued: 0 })
    renderAt('/c/5')
    expect(await screen.findByText(/Agent is working/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Stop' }))
    expect(apiMock.StopChat).toHaveBeenCalledWith(5)
    await waitFor(() => expect(screen.queryByText(/Agent is working/)).toBeNull())
  })

  it('renders system notes (errors, stopped) distinctly from chat bubbles', async () => {
    apiMock.Messages.mockResolvedValue([msg({ id: 1 }), msg({ id: 2, role: 'system', source: 'ui', body: 'Claude Code failed (exit status 1).' })])
    renderAt('/c/5')
    expect(await screen.findByRole('note')).toHaveTextContent('Claude Code failed')
  })

  it('file edits are opt-in and the switch persists', async () => {
    apiMock.Messages.mockResolvedValue([msg({ id: 1 })])
    renderAt('/c/5')
    await userEvent.click(await screen.findByRole('button', { name: /Claude|Agent|Plain shell|claude/i }))
    const box = await screen.findByLabelText('Allow file edits')
    expect(box).not.toBeChecked()
    await userEvent.click(box)
    expect(apiMock.SetChatEdits).toHaveBeenCalledWith(true)
  })

  it('shows the working indicator right after sending when the backend reports a running turn', async () => {
    apiMock.Messages.mockResolvedValue([msg({ id: 1 })])
    apiMock.SendMessage.mockResolvedValue({ sent: false, running: true, note: '' })
    renderAt('/c/5')
    await screen.findByText('hi')
    await userEvent.type(screen.getByLabelText('Message'), 'do it{Enter}')
    expect(await screen.findByText(/Agent is working/)).toBeInTheDocument()
  })
})

describe('Chat page', () => {
  it('shows the welcome screen when no project is open', async () => {
    useStore.setState({ open: null })
    apiMock.WorkspaceList.mockResolvedValue([{ workspace: { id: 1, name: 'shop', path: '/p' } }])
    renderAt('/')
    expect(await screen.findByRole('button', { name: /open project folder/i })).toBeInTheDocument()
    expect(await screen.findByText('shop')).toBeInTheDocument()
  })

  it('empty chat shows the prompt, and the first message creates the conversation then sends it', async () => {
    apiMock.NewConversation.mockResolvedValue({ id: 9 })
    apiMock.SendMessage.mockResolvedValue({ sent: false, note: 'Saved. No agent is running in this chat yet' })
    apiMock.Messages.mockResolvedValue([msg({ id: 1, body: 'hello there' })])
    renderAt('/')
    expect(await screen.findByText('What are we working on?')).toBeInTheDocument()
    await userEvent.type(screen.getByLabelText('Message'), 'hello there{Enter}')
    await waitFor(() => expect(apiMock.NewConversation).toHaveBeenCalledWith(1, '', ''))
    await waitFor(() => expect(apiMock.SendMessage).toHaveBeenCalledWith({ conversationId: 9, text: 'hello there', mediaIds: [], mode: 'chat' }))
    expect(await screen.findByText('hello there')).toBeInTheDocument()
  })

  it('renders user bubbles, agent messages with attachments, and the delivery note', async () => {
    apiMock.Messages.mockResolvedValue([
      msg({ id: 1, body: 'what changed?' }),
      msg({ id: 2, role: 'assistant', source: 'agent', body: '**Done.** See screenshot', media: [{ id: 4, name: 'after.png', kind: 'image', size: 10 }, { id: 5, name: 'demo.mp4', kind: 'video', size: 20 }] }),
    ])
    renderAt('/c/5')
    expect(await screen.findByText('what changed?')).toBeInTheDocument()
    expect(await screen.findByText('Done.')).toBeInTheDocument()
    expect(screen.getByText('Agent (via AICW tools)')).toBeInTheDocument()
    expect(screen.getByAltText('after.png')).toHaveAttribute('src', '/media/4')
    expect(document.querySelector('video')).toHaveAttribute('src', '/media/5')
  })

  it('tells the user when a message was saved but no agent received it', async () => {
    apiMock.Messages.mockResolvedValue([msg({ id: 1 })])
    apiMock.SendMessage.mockResolvedValue({ sent: false, note: 'Saved. No agent is running in this chat yet, so nothing was delivered.' })
    renderAt('/c/5')
    await screen.findByText('hi')
    await userEvent.type(screen.getByLabelText('Message'), 'again{Enter}')
    expect(await screen.findByRole('status')).toHaveTextContent('nothing was delivered')
  })

  it('confirms delivery when the terminal is running', async () => {
    apiMock.Messages.mockResolvedValue([msg({ id: 1 })])
    apiMock.TermStatus.mockResolvedValue({ id: 't1' })
    apiMock.SendMessage.mockResolvedValue({ sent: true, note: '' })
    renderAt('/c/5')
    await screen.findByText('hi')
    await userEvent.type(screen.getByLabelText('Message'), 'go{Enter}')
    expect(await screen.findByText('Delivered to the agent in the terminal.')).toBeInTheDocument()
  })

  it('toggles the terminal drawer and remembers the choice', async () => {
    apiMock.Messages.mockResolvedValue([msg({ id: 1 })])
    renderAt('/c/5')
    await screen.findByText('hi')
    expect(screen.queryByTestId('terminal-pane')).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: /terminal/i }))
    expect(await screen.findByTestId('terminal-pane')).toBeInTheDocument()
    expect(localStorage.getItem('aicw-term-open')).toBe('1')
  })

  it('opens the terminal directly by default and passes autoStart from settings', async () => {
    localStorage.removeItem('aicw-term-open')
    apiMock.Messages.mockResolvedValue([msg({ id: 1 })])
    renderAt('/c/5')
    expect(await screen.findByTestId('terminal-pane')).toHaveAttribute('data-autostart', 'true')
  })

  it('in Terminal mode it starts the agent first, then delivers the message to the terminal', async () => {
    localStorage.setItem('aicw-target', 'terminal')
    apiMock.Messages.mockResolvedValue([msg({ id: 1 })])
    apiMock.TermStatus.mockResolvedValue(null)
    apiMock.TermStart.mockResolvedValue({ id: 't1', running: true })
    apiMock.SendMessage.mockResolvedValue({ sent: true, note: '' })
    renderAt('/c/5')
    await screen.findByText('hi')
    await userEvent.type(screen.getByLabelText('Message'), 'build it{Enter}')
    await waitFor(() => expect(apiMock.SendMessage).toHaveBeenCalledWith({ conversationId: 5, text: 'build it', mediaIds: [], mode: 'terminal' }))
    expect(apiMock.TermStart).toHaveBeenCalledWith(expect.objectContaining({ conversationId: 5 }))
    expect(apiMock.TermStart.mock.invocationCallOrder[0]).toBeLessThan(apiMock.SendMessage.mock.invocationCallOrder[0])
  })

  it('falls back to a chat reply and shows why when the agent cannot start', async () => {
    localStorage.setItem('aicw-target', 'terminal')
    apiMock.Messages.mockResolvedValue([msg({ id: 1 })])
    apiMock.TermStart.mockResolvedValue({ error: 'Claude Code is not installed: none found', fallback: 'Install it.' })
    apiMock.SendMessage.mockResolvedValue({ sent: false, running: false, note: 'x' })
    renderAt('/c/5')
    await screen.findByText('hi')
    await userEvent.type(screen.getByLabelText('Message'), 'hello{Enter}')
    await waitFor(() => expect(apiMock.SendMessage).toHaveBeenCalledWith(expect.objectContaining({ mode: 'chat' })))
    expect(await screen.findByRole('alert')).toHaveTextContent('not installed')
  })

  it('the Terminal / Chat reply switch changes where messages go and is remembered', async () => {
    apiMock.Messages.mockResolvedValue([msg({ id: 1 })])
    renderAt('/c/5')
    await screen.findByText('hi')
    await userEvent.click(screen.getByRole('radio', { name: 'Terminal' }))
    expect(localStorage.getItem('aicw-target')).toBe('terminal')
    await userEvent.click(screen.getByRole('radio', { name: 'Chat reply' }))
    expect(localStorage.getItem('aicw-target')).toBe('chat')
  })

  it('shows a send failure and keeps the draft', async () => {
    apiMock.Messages.mockResolvedValue([msg({ id: 1 })])
    apiMock.SendMessage.mockRejectedValue(new Error('message is too long (64 KB max)'))
    renderAt('/c/5')
    await screen.findByText('hi')
    await userEvent.type(screen.getByLabelText('Message'), 'big')
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('too long')
    expect(screen.getByLabelText('Message')).toHaveValue('big')
  })
})
