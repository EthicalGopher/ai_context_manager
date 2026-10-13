import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { apiMock, resetApi } from '../test/mockApi'

vi.mock('../../wailsjs/runtime/runtime', () => ({ OnFileDrop: vi.fn(), OnFileDropOff: vi.fn() }))
vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<any>('../lib/api')
  return { ...actual, api: apiMock, onEvent: () => () => {} }
})

import PagesView from './Pages'
import { RichMarkdown, MediaCatalog } from '../components/RichMarkdown'
import { useStore } from '../store'

const ws = { id: 1, path: '/p', name: 'My Dream Game', agent: '', settings: '{}', lastOpened: '' }
const page = (o: any = {}) => ({ id: 1, workspaceId: 1, title: 'Character', summary: 'Keep the pink nose.', body: '## Meet Mr. Mak\n\nA white piglet.\n\n![Selected visual reference](media:4)\n\n![Demo](media:5)\n\n> Callout text', sort: 1, updatedAt: '2026-01-01T00:00:00Z', updatedBy: 'agent', ...o })
const media = [{ id: 4, name: 'pig.png', kind: 'image', size: 10 }, { id: 5, name: 'demo.mp4', kind: 'video', size: 20 }]

const renderAt = (path: string) => render(<MemoryRouter initialEntries={[path]}><Routes><Route path="/p" element={<PagesView />} /><Route path="/p/:id" element={<PagesView />} /></Routes></MemoryRouter>)

beforeEach(() => {
  resetApi()
  useStore.setState({ open: { workspace: ws, sessions: [], resumable: [], action: 'new', isNew: false, requestedNew: false, reason: '' } as any })
  apiMock.ListPages.mockResolvedValue([page()])
  apiMock.ListMedia.mockResolvedValue(media)
  apiMock.ListDir.mockResolvedValue([{ name: 'docs', rel: 'docs', dir: true, size: 0, secret: false }, { name: '.env', rel: '.env', dir: false, size: 5, secret: true }, { name: 'README.md', rel: 'README.md', dir: false, size: 40, secret: false }])
})

describe('Pages view', () => {
  it('renders title, lede, who wrote it, and embedded image, video and callout', async () => {
    renderAt('/p/1')
    expect(await screen.findByRole('heading', { level: 1, name: 'Character' })).toBeInTheDocument()
    expect(screen.getByText('Keep the pink nose.')).toBeInTheDocument()
    expect(screen.getByText('written by agent')).toBeInTheDocument()
    expect(await screen.findByAltText('Selected visual reference')).toHaveAttribute('src', '/media/4')
    expect(screen.getByText('Selected visual reference', { selector: 'figcaption' })).toBeInTheDocument()
    expect(document.querySelector('video')).toHaveAttribute('src', '/media/5')
    expect(screen.getByText('Callout text').closest('blockquote')).not.toBeNull()
  })

  it('shows an empty state that explains how pages get written', async () => {
    apiMock.ListPages.mockResolvedValue([])
    renderAt('/p')
    expect(await screen.findByText('No pages yet')).toBeInTheDocument()
    expect(screen.getByText(/write_page/)).toBeInTheDocument()
  })

  it('edits and saves a page, inserting a library item', async () => {
    apiMock.SavePage.mockResolvedValue(page())
    renderAt('/p/1')
    await userEvent.click(await screen.findByRole('button', { name: /edit/i }))
    const body = screen.getByLabelText('Page body')
    await userEvent.type(body, '\n\nmore text')
    await userEvent.click(screen.getByRole('button', { name: 'Media' }))
    await userEvent.click(screen.getAllByRole('button', { name: 'Insert into page' })[0])
    expect((screen.getByLabelText('Page body') as HTMLTextAreaElement).value).toContain('![pig.png](media:4)')
    await userEvent.click(screen.getByRole('button', { name: 'Save page' }))
    await waitFor(() => expect(apiMock.SavePage).toHaveBeenCalled())
    expect(apiMock.SavePage.mock.calls[0][1]).toBe('Character')
    expect(apiMock.SavePage.mock.calls[0][3]).toContain('more text')
  })

  it('requires confirmation before deleting and reassures that media is kept', async () => {
    apiMock.DeletePage.mockResolvedValue(undefined)
    renderAt('/p/1')
    await userEvent.click(await screen.findByRole('button', { name: /delete/i }))
    expect(await screen.findByText(/Media files in the library are kept/)).toBeInTheDocument()
    expect(apiMock.DeletePage).not.toHaveBeenCalled()
    await userEvent.click(screen.getAllByRole('button', { name: 'Delete' }).at(-1)!)
    await waitFor(() => expect(apiMock.DeletePage).toHaveBeenCalledWith(1, true))
  })

  it('file browser lists the project, flags secrets and never offers to preview them', async () => {
    apiMock.ReadFile.mockResolvedValue('# readme text')
    renderAt('/p/1')
    expect(await screen.findByText('README.md')).toBeInTheDocument()
    expect(screen.getByText('secret')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /\.env/ }))
    expect(await screen.findByText(/will not display it/)).toBeInTheDocument()
    expect(apiMock.ReadFile).not.toHaveBeenCalled()
    await userEvent.click(screen.getByRole('button', { name: /README\.md/ }))
    expect(await screen.findByText('# readme text')).toBeInTheDocument()
  })

  it('history modal offers to restore an earlier version into the editor', async () => {
    apiMock.PageVersions.mockResolvedValue([{ id: 9, title: 'Character', summary: 's', body: 'older body', savedAt: '2026-01-01T00:00:00Z', savedBy: 'user' }])
    renderAt('/p/1')
    await userEvent.click(await screen.findByRole('button', { name: /history/i }))
    await userEvent.click(await screen.findByRole('button', { name: 'Restore into editor' }))
    expect((screen.getByLabelText('Page body') as HTMLTextAreaElement).value).toBe('older body')
  })
})

describe('Add data to a page', () => {
  it('shows a composer under the page and appends text with attachments, then refreshes', async () => {
    apiMock.AddToPage.mockResolvedValue(page())
    apiMock.PickMedia.mockResolvedValue(['/a/x.png'])
    apiMock.ImportMedia.mockResolvedValue({ added: [{ id: 21, name: 'x.png', kind: 'image' }], errors: [] })
    renderAt('/p/1')
    const box = await screen.findByLabelText('Message')
    expect(box).toHaveAttribute('placeholder', expect.stringContaining('Character'))
    await userEvent.click(screen.getByRole('button', { name: 'Attach files' }))
    await screen.findByAltText('x.png')
    await userEvent.type(box, 'New paragraph{Enter}')
    await waitFor(() => expect(apiMock.AddToPage).toHaveBeenCalledWith(1, 'New paragraph', [21]))
    await waitFor(() => expect(apiMock.ListPages.mock.calls.length).toBeGreaterThan(1))
  })

  it('can add attachments with no text, and hides the composer while editing', async () => {
    apiMock.AddToPage.mockResolvedValue(page())
    apiMock.PickMedia.mockResolvedValue(['/a/clip.mp4'])
    apiMock.ImportMedia.mockResolvedValue({ added: [{ id: 22, name: 'clip.mp4', kind: 'video' }], errors: [] })
    renderAt('/p/1')
    await userEvent.click(await screen.findByRole('button', { name: 'Attach files' }))
    await screen.findByText('clip.mp4')
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))
    await waitFor(() => expect(apiMock.AddToPage).toHaveBeenCalledWith(1, '', [22]))
    await userEvent.click(screen.getByRole('button', { name: /edit/i }))
    expect(screen.queryByPlaceholderText(/Add text, images/)).toBeNull()
  })
})

describe('RichMarkdown', () => {
  it('renders audio, files, and keeps ordinary links inert', () => {
    const cat = new Map([[7, { id: 7, name: 'note.mp3', kind: 'audio', size: 1 }], [8, { id: 8, name: 'spec.pdf', kind: 'pdf', size: 1 }]])
    const { container } = render(<MediaCatalog.Provider value={cat}><RichMarkdown text={'![voice note](media:7)\n\n[spec](media:8)\n\n[site](https://evil.example)'} /></MediaCatalog.Provider>)
    expect(container.querySelector('audio')).toHaveAttribute('src', '/media/7')
    expect(screen.getByText('spec.pdf')).toBeInTheDocument()
    expect(container.querySelector('a')).toBeNull()
  })

  it('does not load arbitrary remote images', () => {
    const { container } = render(<RichMarkdown text={'![tracker](https://evil.example/p.png)'} />)
    expect(container.querySelector('img')).toBeNull()
    expect(within(container).getByText(/\[image: tracker\]/)).toBeInTheDocument()
  })
})
