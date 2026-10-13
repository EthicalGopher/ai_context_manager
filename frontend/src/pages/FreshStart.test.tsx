import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { apiMock, resetApi } from '../test/mockApi'

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<any>('../lib/api')
  return { ...actual, api: apiMock, onEvent: () => () => {} }
})

import FreshStart from './FreshStart'
import { useStore } from '../store'

const scan = {
  root: '/p', state: 'ai-configured', warnings: [], truncated: false, duplicates: [], tree: ['AGENTS.md'],
  git: { isRepo: true, branch: 'main', dirty: 2 },
  items: [
    { path: 'AGENTS.md', category: 'instruction', size: 100, modTime: '2026-01-01T00:00:00Z', secret: false, injection: false, tooLarge: false, action: 'keep' },
    { path: 'docs/leak.md', category: 'documentation', size: 50, modTime: '2026-01-01T00:00:00Z', secret: true, injection: false, tooLarge: false, action: 'exclude', warning: 'content looks like it contains credentials; excluded from context' },
  ],
}

beforeEach(() => {
  resetApi()
  useStore.setState({ open: { workspace: { id: 1, path: '/p', name: 'p', agent: 'claude', settings: '{}', lastOpened: '' }, sessions: [], resumable: [], action: 'new', isNew: false, requestedNew: true, reason: '' } as any })
  apiMock.Scan.mockResolvedValue(scan)
  apiMock.WorkspaceWarnings.mockResolvedValue(['This directory is outside your home directory.'])
  apiMock.ListAgents.mockResolvedValue([])
  apiMock.GetWorkspaceState.mockResolvedValue({ task: '', knownIssues: '', nextSteps: '', excluded: [], mode: 'clean' })
  apiMock.GetSettings.mockResolvedValue({ defaultAgent: 'claude' })
  apiMock.SaveWorkspaceState.mockResolvedValue(undefined)
  apiMock.BuildContext.mockResolvedValue({ markdown: '# Project Context Package', tokens: 120, budget: 8000, included: [], excluded: [{ path: 'docs/leak.md', reason: 'possible secret' }] })
})

const next = () => userEvent.click(screen.getByRole('button', { name: 'Next' }))

describe('Fresh Start wizard', () => {
  it('step 1 shows path, git status and non-blocking warnings', async () => {
    render(<MemoryRouter><FreshStart /></MemoryRouter>)
    expect(screen.getByText('/p')).toBeInTheDocument()
    expect(await screen.findByText(/outside your home directory/)).toBeInTheDocument()
    expect(await screen.findByText(/branch/)).toBeInTheDocument()
  })

  it('review step locks secret files out and persists user exclusions', async () => {
    render(<MemoryRouter><FreshStart /></MemoryRouter>)
    await next(); await next()
    const leak = await screen.findByLabelText('Include docs/leak.md')
    expect(leak).toBeDisabled()
    expect(leak).not.toBeChecked()
    expect(screen.getByText(/excluded from context/)).toBeInTheDocument()
    await userEvent.click(screen.getByLabelText('Include AGENTS.md'))
    expect(apiMock.SaveWorkspaceState).toHaveBeenCalled()
    expect(apiMock.SaveWorkspaceState.mock.calls.at(-1)![0]).toBe(1)
    expect((apiMock.SaveWorkspaceState.mock.calls.at(-1)![1] as any).excluded).toEqual(['AGENTS.md'])
  })

  it('bootstrap mode previews a diff and requires confirmation before writing', async () => {
    apiMock.InstructionDraft.mockResolvedValue('# AGENTS.md draft')
    apiMock.PreviewWrite.mockResolvedValue({ path: 'AGENTS.md', exists: true, changed: true, diff: '--- a/AGENTS.md\n+++ b/AGENTS.md\n-old\n+new', baseHash: 'h1' })
    apiMock.ApplyWrite.mockResolvedValue({})
    render(<MemoryRouter><FreshStart /></MemoryRouter>)
    await next(); await next(); await next()
    await userEvent.click(screen.getByLabelText(/D\. Bootstrap/))
    await next()
    await screen.findByLabelText('Draft content')
    await userEvent.click(screen.getByRole('button', { name: 'Preview diff' }))
    expect(await screen.findByRole('dialog', { name: 'Replace AGENTS.md?' })).toBeInTheDocument()
    expect(apiMock.ApplyWrite).not.toHaveBeenCalled()
    await userEvent.click(screen.getByRole('button', { name: /back up & replace/i }))
    await waitFor(() => expect(apiMock.ApplyWrite).toHaveBeenCalledWith(1, 'AGENTS.md', '# AGENTS.md draft', 'h1', true))
  })

  it('clean mode offers no file-writing controls', async () => {
    render(<MemoryRouter><FreshStart /></MemoryRouter>)
    await next(); await next(); await next(); await next()
    await screen.findByText(/Context package preview/)
    expect(screen.queryByText(/Save project summary/)).toBeNull()
    expect(screen.queryByLabelText('Draft content')).toBeNull()
  })

  it('shows the empty state when no workspace is selected', () => {
    useStore.setState({ open: null })
    render(<MemoryRouter><FreshStart /></MemoryRouter>)
    expect(screen.getByText('No workspace selected')).toBeInTheDocument()
  })

  it('surfaces scan errors', async () => {
    apiMock.Scan.mockRejectedValue(new Error('workspace directory is missing: /p'))
    render(<MemoryRouter><FreshStart /></MemoryRouter>)
    expect(await screen.findByRole('alert')).toHaveTextContent(/directory is missing/)
  })
})
