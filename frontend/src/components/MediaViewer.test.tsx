import { describe, expect, it } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { MediaViewer } from './MediaViewer'

const layer = (c: HTMLElement) => c.querySelector('.mv-layer') as HTMLElement

describe('MediaViewer', () => {
  it('builds a video player with controls and zooms with the buttons and keys', () => {
    const { container } = render(<MediaViewer kind="video" src="/media/5" caption="Demo" />)
    expect(container.querySelector('video')).toHaveAttribute('src', '/media/5')
    expect(screen.getByRole('button', { name: /Play/ })).toBeInTheDocument()
    expect(screen.getByLabelText('Playback speed')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reset zoom' })).toHaveTextContent('100%')
    fireEvent.click(screen.getByRole('button', { name: /Zoom in/ }))
    expect(screen.getByRole('button', { name: 'Reset zoom' })).toHaveTextContent('125%')
    expect(layer(container).style.transform).toContain('scale(1.25)')
    fireEvent.keyDown(container.querySelector('.mv') as HTMLElement, { key: '+' })
    expect(screen.getByRole('button', { name: 'Reset zoom' })).toHaveTextContent('156%')
    fireEvent.click(screen.getByRole('button', { name: 'Reset zoom' }))
    expect(layer(container).style.transform).toContain('scale(1)')
  })

  it('zooms images with ctrl+wheel but lets a plain wheel scroll the page', () => {
    const { container } = render(<MediaViewer kind="image" src="/media/4" caption="Ref" />)
    const stage = container.querySelector('.mv-stage') as HTMLElement
    const plain = new WheelEvent('wheel', { deltaY: -100, cancelable: true, bubbles: true })
    stage.dispatchEvent(plain)
    expect(plain.defaultPrevented).toBe(false)
    const zoom = new WheelEvent('wheel', { deltaY: -100, ctrlKey: true, cancelable: true, bubbles: true })
    stage.dispatchEvent(zoom)
    expect(zoom.defaultPrevented).toBe(true)
    expect(layer(container).style.transform).toContain('scale(2.')
  })

  it('does not remount the video when the parent re-renders', () => {
    const { container, rerender } = render(<MediaViewer kind="video" src="/media/5" />)
    const v = container.querySelector('video')
    rerender(<MediaViewer kind="video" src="/media/5" />)
    expect(container.querySelector('video')).toBe(v)
  })
})

describe('text size', () => {
  it('steps the root font size and remembers it', async () => {
    const { useStore } = await import('../store')
    useStore.getState().stepText(0)
    expect(document.documentElement.style.fontSize).toBe('16px')
    useStore.getState().stepText(1)
    expect(document.documentElement.style.fontSize).toBe('17.6px')
    expect(localStorage.getItem('aicw-text-scale')).toBe('1.1')
    useStore.getState().stepText(0)
  })
})
