/// <reference types="vite/client" />

declare global {
  interface Window {
    aicwMountViewer: (el: HTMLElement) => { destroy: () => void; media: HTMLElement }
    aicwMountAll: (scope?: ParentNode) => void
  }
}
export {}
