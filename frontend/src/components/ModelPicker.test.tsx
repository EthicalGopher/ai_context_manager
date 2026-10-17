import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { apiMock, resetApi } from '../test/mockApi'

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<any>('../lib/api')
  return { ...actual, api: apiMock, onEvent: () => () => {} }
})

import { ModelPicker } from './ModelPicker'

const claude = { profile: { id: 'claude', modelArgs: ['--model', '{model}'] } }
const freebuff = { profile: { id: 'freebuff' } }
const list = {
  models: [
    { id: 'sonnet', label: 'Sonnet (latest)', group: 'Claude' },
    { id: 'opus', label: 'Opus (latest)', group: 'Claude' },
    { id: 'ollama:llama3.2:latest', label: 'llama3.2:latest', group: 'Local · Ollama' },
  ],
  notes: [],
}

beforeEach(() => {
  resetApi()
  apiMock.GetSettings.mockResolvedValue({ agentModels: {}, theme: 'dark' })
  apiMock.ListAgents.mockResolvedValue([claude, freebuff])
  apiMock.ListModels.mockResolvedValue(list)
  apiMock.SaveSettings.mockResolvedValue(undefined)
})

describe('ModelPicker', () => {
  it('lists the models in a dropdown, local ones included, with nothing pre-selected', async () => {
    render(<ModelPicker agent="claude" />)
    const box = (await screen.findByLabelText('Model for this agent')) as HTMLSelectElement
    await waitFor(() => expect(screen.getByRole('option', { name: /llama3\.2/ })).toBeInTheDocument())
    expect(box.value).toBe('')
    expect(screen.getByRole('option', { name: 'Choose a model…' })).toBeInTheDocument()
    expect(screen.queryByText(/default model/i)).toBeNull()
    expect(screen.getByRole('group', { name: 'Local · Ollama' })).toBeInTheDocument()
  })

  it('saves the chosen model for that agent', async () => {
    render(<ModelPicker agent="claude" />)
    await screen.findByRole('option', { name: 'Opus (latest) (opus)' })
    await userEvent.selectOptions(screen.getByLabelText('Model for this agent'), 'ollama:llama3.2:latest')
    await waitFor(() => expect(apiMock.SaveSettings).toHaveBeenCalledWith({ agentModels: { claude: 'ollama:llama3.2:latest' }, theme: 'dark' }))
  })

  it('lets you type a name that is not in the list', async () => {
    render(<ModelPicker agent="claude" />)
    await screen.findByRole('option', { name: /Sonnet/ })
    await userEvent.selectOptions(screen.getByLabelText('Model for this agent'), '__custom__')
    await userEvent.type(screen.getByPlaceholderText('Type a model name'), 'claude-opus-4-1{Enter}')
    await waitFor(() => expect(apiMock.SaveSettings).toHaveBeenCalledWith({ agentModels: { claude: 'claude-opus-4-1' }, theme: 'dark' }))
  })

  it('falls back to a text box with the reason when the CLI offers no list', async () => {
    apiMock.ListModels.mockResolvedValue({ models: [], notes: ['This agent does not publish a model list. Type a model name.'] })
    render(<ModelPicker agent="claude" />)
    expect(await screen.findByPlaceholderText('Type a model name')).toBeInTheDocument()
    expect(screen.getByText(/does not publish a model list/)).toBeInTheDocument()
  })

  it('is hidden for agents without a model option and for the plain shell', async () => {
    const { container, rerender } = render(<ModelPicker agent="freebuff" />)
    await waitFor(() => expect(apiMock.ListAgents).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
    rerender(<ModelPicker agent="shell" />)
    expect(container).toBeEmptyDOMElement()
  })

  it('reports a refusal from the backend', async () => {
    apiMock.SaveSettings.mockRejectedValue(new Error('invalid model name'))
    const onError = vi.fn()
    render(<ModelPicker agent="claude" onError={onError} />)
    await screen.findByRole('option', { name: /Sonnet/ })
    await userEvent.selectOptions(screen.getByLabelText('Model for this agent'), 'sonnet')
    await waitFor(() => expect(onError).toHaveBeenCalledWith('invalid model name'))
  })
})
