import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { apiMock, resetApi } from '../test/mockApi'

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<any>('../lib/api')
  return { ...actual, api: apiMock, onEvent: () => () => {} }
})
vi.mock('@xterm/xterm', () => ({ Terminal: class { cols = 80; rows = 24; loadAddon() {} open() {} write() {} focus() {} dispose() {} onData() { return { dispose() {} } } } }))
vi.mock('@xterm/addon-fit', () => ({ FitAddon: class { fit() {} } }))
vi.mock('@xterm/xterm/css/xterm.css', () => ({}))
globalThis.ResizeObserver = class { observe() {} disconnect() {} } as any

import { TerminalPane } from './TerminalPane'

const agents = [{ id: 'claude', name: 'Claude Code', state: 'Detected' }]
const conv = (o: any = {}) => ({ id: 5, workspaceId: 1, agent: 'claude', sessionId: 0, ...o })

beforeEach(() => { resetApi(); apiMock.TermStatus.mockResolvedValue(null); apiMock.TermBuffer.mockResolvedValue(''); apiMock.TermResize.mockResolvedValue(undefined) })

describe('TerminalPane', () => {
  it('starts the chat agent by itself when autoStart is on (new session)', async () => {
    apiMock.TermStart.mockResolvedValue({ id: 't1', running: true })
    render(<TerminalPane conv={conv()} agents={agents} autoStart onAgent={() => {}} onStateChange={() => {}} onClose={() => {}} />)
    await waitFor(() => expect(apiMock.TermStart).toHaveBeenCalledTimes(1))
    expect(apiMock.TermStart.mock.calls[0][0]).toMatchObject({ conversationId: 5, agent: 'claude', mode: 'new' })
    expect(await screen.findByTestId('xterm-host')).toBeInTheDocument()
  })

  it('resumes a chat that already has a session', async () => {
    apiMock.TermStart.mockResolvedValue({ id: 't1', running: true })
    render(<TerminalPane conv={conv({ sessionId: 9 })} agents={agents} autoStart onAgent={() => {}} onStateChange={() => {}} onClose={() => {}} />)
    await waitFor(() => expect(apiMock.TermStart).toHaveBeenCalled())
    expect(apiMock.TermStart.mock.calls[0][0]).toMatchObject({ mode: 'resume' })
  })

  it('does not start anything without autoStart, and shows the start choices', async () => {
    render(<TerminalPane conv={conv()} agents={agents} onAgent={() => {}} onStateChange={() => {}} onClose={() => {}} />)
    expect(await screen.findByRole('button', { name: /Start Claude Code/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Open shell' })).toBeInTheDocument()
    expect(apiMock.TermStart).not.toHaveBeenCalled()
  })

  it('does not retry in a loop when the agent cannot start, and explains why', async () => {
    apiMock.TermStart.mockResolvedValue({ error: 'Claude Code is not installed: none found', fallback: 'Install it.' })
    render(<TerminalPane conv={conv()} agents={agents} autoStart onAgent={() => {}} onStateChange={() => {}} onClose={() => {}} />)
    expect(await screen.findByRole('alert')).toHaveTextContent('not installed')
    await new Promise((r) => setTimeout(r, 150))
    expect(apiMock.TermStart).toHaveBeenCalledTimes(1)
  })

  it('attaches to an already-running terminal instead of starting another', async () => {
    apiMock.TermStatus.mockResolvedValue({ id: 't7' })
    render(<TerminalPane conv={conv()} agents={agents} autoStart onAgent={() => {}} onStateChange={() => {}} onClose={() => {}} />)
    expect(await screen.findByTestId('xterm-host')).toBeInTheDocument()
    expect(apiMock.TermStart).not.toHaveBeenCalled()
  })
})
