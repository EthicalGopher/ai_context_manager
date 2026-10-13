import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { apiMock, resetApi } from '../test/mockApi'

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<any>('../lib/api')
  return { ...actual, api: apiMock, onEvent: () => () => {} }
})

import { LaunchDialog } from './LaunchDialog'
import { ResumePanel } from './ResumePanel'
import { Confirm, DiffView } from './ui'

const plan = {
  spec: { dir: '/p', display: "/usr/bin/claude --resume abc", handoff: 'prompt-arg', notes: ['Claude Code can read and modify files'] },
  disclosure: 'context is sent to that tool\'s provider', tokens: 321,
}

beforeEach(() => resetApi())

describe('LaunchDialog', () => {
  it('shows a loading state, then the dry-run plan, and does not launch until confirmed', async () => {
    apiMock.Launch.mockResolvedValueOnce(plan)
    render(<LaunchDialog params={{ workspaceId: 1, mode: 'resume', sessionId: 5 }} onClose={() => {}} onLaunched={() => {}} />)
    expect(screen.getByRole('status')).toHaveTextContent(/preparing/i)
    expect(await screen.findByText(/--resume abc/)).toBeInTheDocument()
    expect(screen.getByText(/sent to that tool/)).toBeInTheDocument()
    expect(apiMock.Launch).toHaveBeenCalledTimes(1)
    expect(apiMock.Launch.mock.calls[0][0]).toMatchObject({ dryRun: true, mode: 'resume', sessionId: 5 })
  })

  it('launches for real only after clicking the button', async () => {
    apiMock.Launch.mockResolvedValueOnce(plan).mockResolvedValueOnce({ ...plan, launched: true, sessionId: 9 })
    const onLaunched = vi.fn()
    render(<LaunchDialog params={{ workspaceId: 1, mode: 'new' }} onClose={() => {}} onLaunched={onLaunched} />)
    await screen.findByText(/--resume abc/)
    await userEvent.click(screen.getByRole('button', { name: /launch in terminal/i }))
    await waitFor(() => expect(onLaunched).toHaveBeenCalled())
    expect(apiMock.Launch.mock.calls[1][0]).toMatchObject({ dryRun: false })
    expect(screen.getByText(/Session recorded/)).toBeInTheDocument()
  })

  it('shows an error and fallback, and disables launch when the CLI is missing', async () => {
    apiMock.Launch.mockResolvedValueOnce({ error: 'Claude Code is not installed: none found', fallback: 'Install the CLI or export the context', spec: null })
    render(<LaunchDialog params={{ workspaceId: 1, mode: 'new' }} onClose={() => {}} onLaunched={() => {}} />)
    expect(await screen.findByText(/is not installed/)).toBeInTheDocument()
    expect(screen.getByText(/export the context/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /launch in terminal/i })).toBeDisabled()
  })

  it('offers an explicit override when the session looks already running', async () => {
    apiMock.Launch.mockResolvedValueOnce({ error: 'This session appears to be running already (launcher PID 7).', spec: null })
    render(<LaunchDialog params={{ workspaceId: 1, mode: 'resume', sessionId: 1 }} onClose={() => {}} onLaunched={() => {}} />)
    expect(await screen.findByRole('button', { name: /launch anyway/i })).toBeInTheDocument()
  })

  it('surfaces backend rejection', async () => {
    apiMock.Launch.mockRejectedValueOnce(new Error('session does not belong to this workspace'))
    render(<LaunchDialog params={{ workspaceId: 1, mode: 'resume', sessionId: 1 }} onClose={() => {}} onLaunched={() => {}} />)
    expect(await screen.findByRole('alert')).toHaveTextContent(/does not belong/)
  })
})

describe('ResumePanel (resume-first)', () => {
  const ws: any = { id: 1, path: '/p', name: 'p', agent: 'claude', settings: '{}', lastOpened: '' }
  const ses = (over: any = {}): any => ({ id: 1, workspaceId: 1, agent: 'claude', externalId: 'abc', status: 'paused', title: 'Fix login', handoff: '', pid: 0, createdAt: '', lastActive: new Date().toISOString(), ...over })

  it('offers resume for a saved session and starts nothing by itself', async () => {
    apiMock.Launch.mockResolvedValue(plan)
    const open: any = { workspace: ws, sessions: [ses()], resumable: [ses()], action: 'resume', isNew: false, requestedNew: false, reason: 'one saved session found' }
    render(<MemoryRouter><ResumePanel open={open} onChanged={() => {}} /></MemoryRouter>)
    expect(screen.getByText('Saved session found')).toBeInTheDocument()
    expect(screen.getByText('Fix login')).toBeInTheDocument()
    expect(apiMock.Launch).not.toHaveBeenCalled()
    await userEvent.click(screen.getByRole('button', { name: 'Resume' }))
    await waitFor(() => expect(apiMock.Launch).toHaveBeenCalled())
    expect(apiMock.Launch.mock.calls.every((c) => c[0].dryRun === true)).toBe(true)
  })

  it('shows a picker with every candidate when several sessions exist', () => {
    const list = [ses({ id: 1, title: 'A' }), ses({ id: 2, title: 'B', agent: 'kilo' })]
    const open: any = { workspace: ws, sessions: list, resumable: list, action: 'choose', isNew: false, requestedNew: false, reason: '2 saved sessions found; choose which to resume' }
    render(<MemoryRouter><ResumePanel open={open} onChanged={() => {}} /></MemoryRouter>)
    expect(screen.getByText('Choose a session to resume')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Resume' })).toHaveLength(2)
  })

  it('discloses when only AICW context (not the CLI conversation) can be restored', () => {
    const s = ses({ externalId: '' })
    const open: any = { workspace: ws, sessions: [s], resumable: [s], action: 'resume', isNew: false, requestedNew: false, reason: 'one saved session found' }
    render(<MemoryRouter><ResumePanel open={open} onChanged={() => {}} /></MemoryRouter>)
    expect(screen.getByText(/only AICW's own context can be restored/)).toBeInTheDocument()
  })

  it('points to Fresh Start when nothing is resumable', () => {
    const open: any = { workspace: ws, sessions: [], resumable: [], action: 'new', isNew: true, requestedNew: false, reason: 'no resumable session; confirm to start a new one' }
    render(<MemoryRouter><ResumePanel open={open} onChanged={() => {}} /></MemoryRouter>)
    expect(screen.getByText('No saved session')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Resume' })).toBeNull()
  })
})

describe('Confirm / DiffView', () => {
  it('Escape cancels without confirming', () => {
    const onConfirm = vi.fn(), onCancel = vi.fn()
    render(<Confirm title="Replace AGENTS.md?" onConfirm={onConfirm} onCancel={onCancel}><p>body</p></Confirm>)
    expect(screen.getByRole('dialog', { name: 'Replace AGENTS.md?' })).toBeInTheDocument()
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onCancel).toHaveBeenCalled()
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('renders added and removed lines distinctly and handles empty diffs', () => {
    const { container, rerender } = render(<DiffView diff={'--- a/x\n+++ b/x\n-old\n+new\n same'} />)
    expect(container.querySelectorAll('.diff-add')).toHaveLength(1)
    expect(container.querySelectorAll('.diff-del')).toHaveLength(1)
    rerender(<DiffView diff="" />)
    expect(screen.getByText('No changes.')).toBeInTheDocument()
  })
})

describe('Modal focus', () => {
  it('keeps focus in a text field while typing (re-renders must not steal focus)', async () => {
    const { useState } = await import('react')
    function Host() {
      const [v, setV] = useState('')
      return <Confirm title="t" onConfirm={() => {}} onCancel={() => {}}><input aria-label="field" autoFocus value={v} onChange={(e) => setV(e.target.value)} /></Confirm>
    }
    render(<Host />)
    await userEvent.type(screen.getByLabelText('field'), 'hello world')
    expect(screen.getByLabelText('field')).toHaveValue('hello world')
  })
})
