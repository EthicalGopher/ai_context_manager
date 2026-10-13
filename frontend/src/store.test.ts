import { describe, it, expect } from 'vitest'
import { useStore } from './store'

describe('store', () => {
  it('toggles the light theme class and survives unavailable storage', () => {
    const orig = Storage.prototype.setItem
    Storage.prototype.setItem = () => { throw new Error('blocked') }
    useStore.getState().setTheme('light')
    expect(document.documentElement.classList.contains('light')).toBe(true)
    useStore.getState().setTheme('dark')
    expect(document.documentElement.classList.contains('light')).toBe(false)
    Storage.prototype.setItem = orig
  })
})
