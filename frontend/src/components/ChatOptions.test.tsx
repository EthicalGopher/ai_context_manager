import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { apiMock, resetApi } from '../test/mockApi'

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<any>('../lib/api')
  return { ...actual, api: apiMock, onEvent: () => () => {} }
})

import { ChatOptions } from './ChatOptions'

beforeEach(() => { resetApi(); apiMock.GetSettings.mockResolvedValue({ agentModels: {} }); apiMock.ListAgents.mockResolvedValue([]) })

const agents = [{ id: 'claude', name: 'Claude Code' }, { id: 'agy', name: 'AGY' }]

describe('ChatOptions', () => {
  it('keeps agent, model and edit permission in one menu that opens and closes', async () => {
    const onAgent = vi.fn(), onEdits = vi.fn()
    render(<ChatOptions agent="agy" agents={agents} edits={false} onAgent={onAgent} onEdits={onEdits} onError={() => {}} />)
    expect(screen.queryByRole('dialog')).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: /AGY/ }))
    expect(screen.getByRole('dialog', { name: 'Chat options' })).toBeInTheDocument()
    await userEvent.selectOptions(screen.getByLabelText('Agent for this chat'), 'claude')
    expect(onAgent).toHaveBeenCalledWith('claude')
    await userEvent.click(screen.getByLabelText('Allow file edits'))
    expect(onEdits).toHaveBeenCalledWith(true)
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('shows the plain shell without a model section', async () => {
    render(<ChatOptions agent="shell" agents={agents} edits={false} onAgent={() => {}} onEdits={() => {}} onError={() => {}} />)
    await userEvent.click(screen.getByRole('button', { name: /Plain shell/ }))
    expect(screen.queryByText('Model')).toBeNull()
  })
})
