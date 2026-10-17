import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { apiMock, resetApi } from '../test/mockApi'

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<any>('../lib/api')
  return { ...actual, api: apiMock, onEvent: () => () => {} }
})
vi.mock('../../wailsjs/runtime/runtime', () => ({ OnFileDrop: vi.fn(), OnFileDropOff: vi.fn() }))

import { Composer } from './Composer'

beforeEach(() => resetApi())

describe('Composer', () => {
  it('sends on Enter, keeps Shift+Enter as a newline, and clears afterwards', async () => {
    const onSend = vi.fn().mockResolvedValue(undefined)
    render(<Composer wsId={1} onSend={onSend} />)
    const box = screen.getByLabelText('Message')
    await userEvent.type(box, 'line one{Shift>}{Enter}{/Shift}line two')
    expect(box).toHaveValue('line one\nline two')
    await userEvent.keyboard('{Enter}')
    await waitFor(() => expect(onSend).toHaveBeenCalledWith('line one\nline two', []))
    expect(box).toHaveValue('')
  })

  it('disables Send for empty input and restores the draft if sending fails', async () => {
    const onSend = vi.fn().mockRejectedValue(new Error('boom'))
    render(<Composer wsId={1} onSend={onSend} />)
    expect(screen.getByRole('button', { name: 'Send' })).toBeDisabled()
    await userEvent.type(screen.getByLabelText('Message'), 'keep me')
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('boom')
    expect(screen.getByLabelText('Message')).toHaveValue('keep me')
  })

  it('attaches picked files, shows a removable chip, and sends their ids', async () => {
    apiMock.PickMedia.mockResolvedValue(['/a/shot.png'])
    apiMock.ImportMedia.mockResolvedValue({ added: [{ id: 7, name: 'shot.png', kind: 'image' }], errors: [] })
    const onSend = vi.fn().mockResolvedValue(undefined)
    render(<Composer wsId={1} onSend={onSend} />)
    await userEvent.click(screen.getByRole('button', { name: 'Attach files' }))
    expect(await screen.findByAltText('shot.png')).toHaveAttribute('src', '/media/7')
    await userEvent.type(screen.getByLabelText('Message'), 'see this')
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))
    await waitFor(() => expect(onSend).toHaveBeenCalledWith('see this', [{ id: 7, name: 'shot.png', kind: 'image' }]))
  })

  it('can send attachments alone and remove one before sending', async () => {
    apiMock.PickMedia.mockResolvedValue(['/a/x.mp4'])
    apiMock.ImportMedia.mockResolvedValue({ added: [{ id: 3, name: 'x.mp4', kind: 'video' }], errors: [] })
    render(<Composer wsId={1} onSend={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: 'Attach files' }))
    await screen.findByText('x.mp4')
    expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled()
    await userEvent.click(screen.getByRole('button', { name: 'Remove x.mp4' }))
    expect(screen.queryByText('x.mp4')).toBeNull()
  })

  it('dictation: record, stop, and the transcript lands in the box for review', async () => {
    apiMock.VoiceStatus.mockResolvedValue({ ready: true })
    apiMock.VoiceStart.mockResolvedValue(undefined)
    apiMock.VoiceStop.mockResolvedValue('Turn the heading blue')
    const onSend = vi.fn()
    render(<Composer wsId={1} onSend={onSend} />)
    await userEvent.click(screen.getByRole('button', { name: 'Dictate with your voice' }))
    const stop = await screen.findByRole('button', { name: 'Stop recording' })
    expect(screen.getByLabelText('Message')).toBeDisabled()
    await userEvent.click(stop)
    await waitFor(() => expect(screen.getByLabelText('Message')).toHaveValue('Turn the heading blue'))
    expect(onSend).not.toHaveBeenCalled() // never auto-sent
  })

  it('explains how to set up voice when no engine is ready', async () => {
    apiMock.VoiceStatus.mockResolvedValue({ ready: false, hint: 'No speech-to-text engine yet.' })
    const help = vi.fn()
    render(<Composer wsId={1} onSend={vi.fn()} onVoiceHelp={help} />)
    await userEvent.click(screen.getByRole('button', { name: 'Dictate with your voice' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('No speech-to-text engine yet.')
    expect(apiMock.VoiceStart).not.toHaveBeenCalled()
    expect(help).toHaveBeenCalled()
  })

  it('shows a transcription error without losing typed text', async () => {
    apiMock.VoiceStatus.mockResolvedValue({ ready: true })
    apiMock.VoiceStart.mockResolvedValue(undefined)
    apiMock.VoiceStop.mockRejectedValue(new Error('no speech was recognised'))
    render(<Composer wsId={1} onSend={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: 'Dictate with your voice' }))
    await userEvent.click(await screen.findByRole('button', { name: 'Stop recording' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('no speech was recognised')
    expect(screen.getByLabelText('Message')).toBeEnabled()
  })

  it('voice message: keeps the recording as an audio attachment and adds the transcript', async () => {
    apiMock.VoiceStatus.mockResolvedValue({ recorder: 'pw-record', ready: false })
    apiMock.VoiceStart.mockResolvedValue(undefined)
    apiMock.VoiceStopAttach.mockResolvedValue({ media: { id: 11, name: 'voice-1.wav' }, transcript: 'Add a locations page', warning: '' })
    const onSend = vi.fn().mockResolvedValue(undefined)
    render(<Composer wsId={1} onSend={onSend} />)
    await userEvent.click(screen.getByRole('button', { name: 'Record a voice message' }))
    // while a voice message records, dictation is locked out so the two cannot mix
    expect(screen.getByRole('button', { name: 'Dictate with your voice' })).toBeDisabled()
    await userEvent.click(await screen.findByRole('button', { name: 'Stop voice message' }))
    await waitFor(() => expect(screen.getByLabelText('Message')).toHaveValue('Add a locations page'))
    expect(screen.getByText('voice-1.wav')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Send' }))
    await waitFor(() => expect(onSend).toHaveBeenCalledWith('Add a locations page', [{ id: 11, name: 'voice-1.wav', kind: 'audio' }]))
  })

  it('voice message still attaches the audio when no transcript is possible, and says so', async () => {
    apiMock.VoiceStatus.mockResolvedValue({ recorder: 'pw-record' })
    apiMock.VoiceStart.mockResolvedValue(undefined)
    apiMock.VoiceStopAttach.mockResolvedValue({ media: { id: 12, name: 'voice-2.wav' }, transcript: '', warning: 'No speech-to-text engine is set up, so only the audio is attached.' })
    render(<Composer wsId={1} onSend={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: 'Record a voice message' }))
    await userEvent.click(await screen.findByRole('button', { name: 'Stop voice message' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('only the audio is attached')
    expect(screen.getByText('voice-2.wav')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Send' })).toBeEnabled()
  })

  it('voice message explains when there is no recorder', async () => {
    apiMock.VoiceStatus.mockResolvedValue({ recorder: '', hint: 'No audio recorder found.' })
    render(<Composer wsId={1} onSend={vi.fn()} />)
    await userEvent.click(screen.getByRole('button', { name: 'Record a voice message' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('No audio recorder found.')
    expect(apiMock.VoiceStart).not.toHaveBeenCalled()
  })

  it('offers the fixed / commands, completes with Enter, then sends the command text', async () => {
    apiMock.SlashCommands.mockResolvedValue([
      { name: 'grill-me', description: 'Interview me', usage: '[topic]' },
      { name: 'write-pages', description: 'Save to pages' },
    ])
    const onSend = vi.fn().mockResolvedValue(undefined)
    render(<Composer wsId={1} onSend={onSend} />)
    const box = screen.getByLabelText('Message')
    await userEvent.type(box, '/gr')
    expect(await screen.findByRole('option', { name: /grill-me/ })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: /write-pages/ })).toBeNull()
    await userEvent.keyboard('{Enter}')
    expect(box).toHaveValue('/grill-me ')
    expect(onSend).not.toHaveBeenCalled()
    await userEvent.type(box, 'login{Enter}')
    await waitFor(() => expect(onSend).toHaveBeenCalledWith('/grill-me login', []))
  })
})
