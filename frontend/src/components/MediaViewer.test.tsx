import { describe, expect, it } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { MediaViewer } from './MediaViewer'
import { buildPageExport } from '../lib/exportPage'

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

describe('buildPageExport', () => {
  const page = { title: 'Demo <1>', summary: 'Sum', updatedAt: '2026-11-01T00:00:00Z', body: 'Hello\n\n![Clip](media:5)\n\n![Pic](media:4)\n\n[Song](media:6)\n\n<script>alert(1)</script>' }
  const catalog = new Map([[4, { id: 4, name: 'p.png', kind: 'image', size: 1 }], [5, { id: 5, name: 'c.mp4', kind: 'video', size: 1 }], [6, { id: 6, name: 's.mp3', kind: 'audio', size: 1 }]])
  it('references media by id for the backend to embed, and escapes the page text', () => {
    const x = buildPageExport(page, catalog)
    expect(x.body).toContain('class="mv" data-kind="video" data-src="/media/5" data-caption="Clip"')
    expect(x.body).toContain('data-kind="image" data-src="/media/4"')
    expect(x.body).toContain('<audio controls="" preload="metadata" src="/media/6">')
    expect(x.body).toContain('Demo &lt;1&gt;')
    expect(x.body).not.toContain('<script>alert')
    expect(x.js).toContain('aicwMountViewer')
    expect(x.js.trim().endsWith('window.aicwMountAll(document);')).toBe(true)
  })
})
