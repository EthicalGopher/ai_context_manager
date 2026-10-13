import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { apiMock, resetApi } from '../test/mockApi'

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<any>('../lib/api')
  return { ...actual, api: apiMock, onEvent: () => () => {} }
})
vi.mock('../../wailsjs/runtime/runtime', () => ({ OnFileDrop: vi.fn(), OnFileDropOff: vi.fn() }))

import Media from './Media'
import { useStore } from '../store'

const item = (o: any = {}) => ({ id: 1, name: 'bug.png', kind: 'image', mime: 'image/png', size: 2048, note: '', inContext: false, addedAt: '2026-01-01T00:00:00Z', ...o })

beforeEach(() => {
  resetApi()
  useStore.setState({ open: { workspace: { id: 1, path: '/p', name: 'p', agent: '', settings: '{}', lastOpened: '' }, sessions: [], resumable: [], action: 'new', isNew: false, requestedNew: false, reason: '' } as any })
})
const wrap = () => render(<MemoryRouter><Media /></MemoryRouter>)

describe('Media page', () => {
  it('shows an empty state', async () => {
    apiMock.ListMedia.mockResolvedValue([])
    wrap()
    expect(await screen.findByText('No media yet')).toBeInTheDocument()
  })

  it('renders image and video tiles and filters by type', async () => {
    apiMock.ListMedia.mockResolvedValue([item(), item({ id: 2, name: 'demo.mp4', kind: 'video', mime: 'video/mp4' })])
    wrap()
    expect(await screen.findByAltText('bug.png')).toHaveAttribute('src', '/media/1')
    await userEvent.click(screen.getByRole('tab', { name: /video/i }))
    expect(screen.queryByAltText('bug.png')).toBeNull()
    expect(screen.getByLabelText('Open demo.mp4')).toBeInTheDocument()
  })

  it('imports picked files and reports per-file errors', async () => {
    apiMock.ListMedia.mockResolvedValue([])
    apiMock.PickMedia.mockResolvedValue(['/a/x.png', '/a/.env'])
    apiMock.ImportMedia.mockResolvedValue({ added: [item()], errors: ['.env: looks like a secrets file; refusing to store it'] })
    wrap()
    await screen.findByText('No media yet')
    await userEvent.click(screen.getAllByRole('button', { name: /add files/i })[0])
    await waitFor(() => expect(apiMock.ImportMedia).toHaveBeenCalledWith(1, ['/a/x.png', '/a/.env']))
    expect(await screen.findByRole('alert')).toHaveTextContent(/secrets file/)
  })

  it('requires confirmation before deleting and says the original is kept', async () => {
    apiMock.ListMedia.mockResolvedValue([item()])
    apiMock.DeleteMedia.mockResolvedValue(undefined)
    wrap()
    await userEvent.click(await screen.findByLabelText('Open bug.png'))
    await userEvent.click(screen.getByRole('button', { name: /delete/i }))
    expect(await screen.findByText(/original file you imported is not touched/)).toBeInTheDocument()
    expect(apiMock.DeleteMedia).not.toHaveBeenCalled()
    await userEvent.click(screen.getAllByRole('button', { name: 'Delete' }).at(-1)!)
    await waitFor(() => expect(apiMock.DeleteMedia).toHaveBeenCalledWith(1, true))
  })

  it('toggles inclusion in context', async () => {
    apiMock.ListMedia.mockResolvedValue([item()])
    apiMock.UpdateMedia.mockResolvedValue(undefined)
    wrap()
    await userEvent.click(await screen.findByLabelText('Include bug.png in context'))
    expect(apiMock.UpdateMedia).toHaveBeenCalledWith(1, '', true)
  })
})
