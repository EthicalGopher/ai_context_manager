import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { apiMock, resetApi } from '../test/mockApi'

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<any>('../lib/api')
  return { ...actual, api: apiMock, onEvent: () => () => {} }
})

import { LocalModels } from './LocalModels'

const status = (over: any = {}) => ({
  engineInstalled: false, engineVersion: '', running: '', dir: '/data/llm', models: [],
  catalog: [{ id: 'qwen3-4b', name: 'Qwen3 4B', file: 'Qwen3-4B-Q4_K_M.gguf', size: 2497280256, note: 'General purpose' }], ...over,
})

beforeEach(() => resetApi())

describe('LocalModels', () => {
  it('asks before downloading the engine, then starts it', async () => {
    apiMock.LocalModels.mockResolvedValue(status())
    apiMock.LocalEngineInstall.mockResolvedValue(undefined)
    render(<LocalModels />)
    await userEvent.click(await screen.findByRole('button', { name: 'Download engine' }))
    expect(apiMock.LocalEngineInstall).not.toHaveBeenCalled()
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Download' }))
    await waitFor(() => expect(apiMock.LocalEngineInstall).toHaveBeenCalledWith(true))
  })

  it('shows sizes, confirms a model download, and offers delete once downloaded', async () => {
    apiMock.LocalModels.mockResolvedValue(status({ engineInstalled: true, engineVersion: 'b1' }))
    apiMock.LocalModelDownload.mockResolvedValue(undefined)
    const { unmount } = render(<LocalModels />)
    expect(await screen.findByText(/2\.5 GB/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Download' }))
    await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Download' }))
    await waitFor(() => expect(apiMock.LocalModelDownload).toHaveBeenCalledWith('qwen3-4b', true))
    unmount()
    apiMock.LocalModels.mockResolvedValue(status({ engineInstalled: true, models: [{ file: 'Qwen3-4B-Q4_K_M.gguf', name: 'Qwen3 4B', size: 2497280256 }] }))
    render(<LocalModels />)
    expect(await screen.findByText('Downloaded')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument()
  })
})
