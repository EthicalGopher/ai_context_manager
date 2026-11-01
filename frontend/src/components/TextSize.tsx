import { FiMinus, FiPlus } from 'react-icons/fi'
import { useStore } from '../store'

/** Makes all text bigger or smaller (also Ctrl + / Ctrl - / Ctrl 0 and Ctrl + scroll). */
export function TextSize() {
  const { textScale, stepText } = useStore()
  return (
    <div role="group" aria-label="Text size" className="inline-flex shrink-0 items-center rounded-lg border border-line bg-bg">
      <button type="button" aria-label="Smaller text" title="Smaller text (Ctrl -)" onClick={() => stepText(-1)} className="px-2 py-1.5 text-mute hover:text-fg"><FiMinus aria-hidden /></button>
      <button type="button" aria-label="Reset text size" title="Reset text size (Ctrl 0)" onClick={() => stepText(0)} className="min-w-[3rem] px-1 text-xs tabular-nums text-mute hover:text-fg">{Math.round(textScale * 100)}%</button>
      <button type="button" aria-label="Bigger text" title="Bigger text (Ctrl +)" onClick={() => stepText(1)} className="px-2 py-1.5 text-mute hover:text-fg"><FiPlus aria-hidden /></button>
    </div>
  )
}
