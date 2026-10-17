import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { apiMock, resetApi } from '../test/mockApi'

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<any>('../lib/api')
  return { ...actual, api: apiMock, onEvent: () => () => {} }
})

import { ModelPicker } from './ModelPicker'

const claude = { profile: { id: 'claude', modelArgs: ['--model', '{model}'], modelHints: ['sonnet', 'opus'] } }
const freebuff = { profile: { id: 'freebuff' } }

beforeEach(() => resetApi())

describe('ModelPicker', () => {
  it('shows the saved model and saves a change for that agent only', async () => {
    apiMock.GetSettings.mockResolvedValue({ agentModels: { claude: 'sonnet' }, theme: 'dark' })
    apiMock.ListAgents.mockResolvedValue([claude, freebuff])
    apiMock.SaveSettings.mockResolvedValue(undefined)
    render(<ModelPicker agent="claude" />)
    const box = await screen.findByLabelText('Model for this agent')
    expect(box).toHaveValue('sonnet')
    await userEvent.clear(box)
    await userEvent.type(box, 'opus{Enter}')
    await waitFor(() => expect(apiMock.SaveSettings).toHaveBeenCalledWith({ agentModels: { claude: 'opus' }, theme: 'dark' }))
  })

  it('is hidden for agents without a model option and for the plain shell', async () => {
    apiMock.GetSettings.mockResolvedValue({ agentModels: {} })
    apiMock.ListAgents.mockResolvedValue([claude, freebuff])
    const { container, rerender } = render(<ModelPicker agent="freebuff" />)
    await waitFor(() => expect(apiMock.ListAgents).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
    rerender(<ModelPicker agent="shell" />)
    expect(container).toBeEmptyDOMElement()
  })

  it('puts the backend refusal back into the box and reports it', async () => {
    apiMock.GetSettings.mockResolvedValue({ agentModels: {} })
    apiMock.ListAgents.mockResolvedValue([claude])
    apiMock.SaveSettings.mockRejectedValue(new Error('invalid model name'))
    const onError = vi.fn()
    render(<ModelPicker agent="claude" onError={onError} />)
    const box = await screen.findByLabelText('Model for this agent')
    await userEvent.type(box, '--evil{Enter}')
    await waitFor(() => expect(onError).toHaveBeenCalledWith('invalid model name'))
    expect(box).toHaveValue('')
  })
})
