import { useEffect, useState } from 'react'
import { api, errMsg, onEvent } from '../lib/api'
import { useStore } from '../store'
import { Button, Card, Confirm, ErrorBox, PageHeader } from '../components/ui'

export default function Settings() {
  const [s, setS] = useState<any>(null)
  const [agents, setAgents] = useState<any[]>([])
  const [terms, setTerms] = useState<string[]>([])
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  const [doctor, setDoctor] = useState('')
  const [voice, setVoice] = useState<any>(null)
  const [setup, setSetup] = useState<string[] | null>(null)
  const [confirmSetup, setConfirmSetup] = useState(false)
  const [setupErr, setSetupErr] = useState('')
  const loadVoice = () => api.VoiceStatus().then(setVoice).catch(() => {})
  const { theme, setTheme } = useStore()
  useEffect(() => {
    api.GetSettings().then(setS).catch((e) => setError(errMsg(e)))
    api.ListAgents().then((a) => setAgents(a as any)).catch(() => {})
    api.ListTerminals().then((t) => setTerms(t as any)).catch(() => {})
    loadVoice()
    const a = onEvent('voice:setup', (l: string) => setSetup((x) => [...(x ?? []), l]))
    const b = onEvent('voice:setup:done', (m: string) => { setSetupErr(m); loadVoice() })
    return () => { a(); b() }
  }, [])
  if (!s) return <><PageHeader title="Settings" /><ErrorBox error={error} /></>
  const set = (k: string, v: any) => { setSaved(false); setS({ ...s, [k]: v }) }
  const lines = (v: string) => v.split('\n').map((x) => x.trim()).filter(Boolean)
  const row = (label: string, el: React.ReactNode, hint?: string) => (
    <div className="grid items-start gap-2 border-t border-line py-3 first:border-t-0 md:grid-cols-[14rem_1fr]"><div><div className="text-sm font-medium">{label}</div>{hint && <div className="text-xs text-mute">{hint}</div>}</div><div>{el}</div></div>)
  const inp = 'rounded-md border border-line bg-bg px-2 py-1.5'
  return (
    <>
      <PageHeader title="Settings" actions={<Button variant="primary" onClick={async () => { try { await api.SaveSettings(s); setSaved(true); setError('') } catch (e) { setError(errMsg(e)) } }}>Save settings</Button>} />
      <ErrorBox error={error} onDismiss={() => setError('')} />
      {saved && <p role="status" className="mb-3 text-sm text-ok">Settings saved.</p>}
      <Card>
        {row('Default AI agent', <select aria-label="Default AI agent" className={inp} value={s.defaultAgent} onChange={(e) => set('defaultAgent', e.target.value)}>{agents.map((a) => <option key={a.profile.id} value={a.profile.id}>{a.profile.displayName}</option>)}</select>)}
        {row('Default Fresh Start mode', <select aria-label="Default mode" className={inp} value={s.defaultMode} onChange={(e) => set('defaultMode', e.target.value)}>{['clean', 'refresh', 'archive', 'bootstrap', 'custom'].map((m) => <option key={m}>{m}</option>)}</select>)}
        {row('Context budget (tokens)', <input aria-label="Context budget" type="number" className={`${inp} w-32`} value={s.budget} onChange={(e) => set('budget', Number(e.target.value))} />, 'Approximate (~4 bytes per token).')}
        {row('File-size limit (KB)', <input aria-label="File size limit" type="number" className={`${inp} w-32`} value={s.maxFileKB} onChange={(e) => set('maxFileKB', Number(e.target.value))} />)}
        {row('Ignored directories', <textarea aria-label="Ignored directories" rows={4} className={`${inp} mono w-full text-xs`} value={s.ignoreDirs.join('\n')} onChange={(e) => set('ignoreDirs', lines(e.target.value))} />, 'One per line. .gitignore is also respected.')}
        {row('Ignored file globs', <textarea aria-label="Ignored globs" rows={2} className={`${inp} mono w-full text-xs`} value={s.ignoreGlobs.join('\n')} onChange={(e) => set('ignoreGlobs', lines(e.target.value))} />)}
        {row('Context storage location', <input aria-label="Context storage location" className={`${inp} w-full`} placeholder="(default: app data directory)" value={s.contextDir} onChange={(e) => set('contextDir', e.target.value)} />, 'Absolute path. Launch context files go here, not into your project.')}
        {row('Terminal', <select aria-label="Terminal" className={inp} value={s.terminal} onChange={(e) => set('terminal', e.target.value)}><option value="">(first available)</option>{terms.map((t) => <option key={t}>{t}</option>)}</select>)}
        {row('Open the agent terminal automatically', <label className="flex items-center gap-2"><input type="checkbox" aria-label="Open terminal automatically" checked={s.autoTerminal !== false} onChange={(e) => set('autoTerminal', e.target.checked)} /> Show the terminal in every chat and start its agent right away</label>, 'Starting a CLI does not use your account until you send it a message.')}
        {row('Confirm file writes', <label className="flex items-center gap-2"><input type="checkbox" checked disabled /> Always on — writes, moves and restores require confirmation.</label>)}
        {row('Theme', <select aria-label="Theme" className={inp} value={theme} onChange={(e) => { setTheme(e.target.value as any); set('theme', e.target.value) }}><option>dark</option><option>light</option></select>)}
      </Card>
      <Card title="Voice dictation" className="mt-4">
        <p className="mb-3 text-sm text-mute">The mic button in the chat records with your system audio tools and converts speech to text locally. Audio never leaves your computer and the recording is deleted right after transcription. You review the text before sending.</p>
        <dl className="mb-3 grid grid-cols-[8rem_1fr] gap-y-1 text-sm">
          <dt className="text-mute">Recorder</dt><dd>{voice?.recorder || <span className="text-warn">not found</span>}</dd>
          <dt className="text-mute">Speech engine</dt><dd>{voice?.engine || <span className="text-warn">not set up</span>}</dd>
          <dt className="text-mute">Status</dt><dd>{voice?.ready ? <span className="text-ok">Ready</span> : <span className="text-warn">{voice?.hint}</span>}</dd>
        </dl>
        {row('Model', <select aria-label="Voice model" className={inp} value={s.voiceModel || 'base'} onChange={(e) => set('voiceModel', e.target.value)}>{['tiny', 'base', 'small', 'medium'].map((m) => <option key={m}>{m}</option>)}</select>, 'Bigger is more accurate and slower. base ≈ 150 MB.')}
        {row('Language', <input aria-label="Voice language" className={`${inp} w-32`} placeholder="auto or en" value={s.voiceLanguage ?? ''} onChange={(e) => set('voiceLanguage', e.target.value)} />)}
        {row('Custom command', <input aria-label="Custom speech command" className={`${inp} w-full font-mono text-xs`} placeholder="whisper-cli -m /path/model.bin -f {audio} -nt" value={s.voiceCommand ?? ''} onChange={(e) => set('voiceCommand', e.target.value)} />, 'Optional. Must print the transcript to stdout. Overrides the built-in engine. {audio} is replaced by the recording path.')}
        <div className="mt-3 flex flex-wrap gap-2"><Button onClick={() => setConfirmSetup(true)}>Set up offline voice (faster-whisper)</Button><Button variant="ghost" onClick={loadVoice}>Re-check</Button></div>
        {setupErr && <p role="alert" className="mt-2 text-sm text-bad">{setupErr}</p>}
        {setup && <pre aria-label="Setup log" className="mt-3 max-h-48 overflow-auto rounded-lg border border-line bg-bg p-2 text-xs">{setup.join('\n')}</pre>}
      </Card>
      {confirmSetup && <Confirm title="Install offline voice?" confirmLabel="Install" onCancel={() => setConfirmSetup(false)} onConfirm={async () => { setConfirmSetup(false); setSetup([]); setSetupErr(''); try { await api.VoiceSetup(true) } catch (e) { setError(errMsg(e)) } }}>
        <p className="text-sm">This creates a private Python environment inside AICW's data folder and downloads <b>faster-whisper</b> (tens of MB) from the Python package index. The speech model (about 150 MB for “base”) downloads the first time you dictate. Nothing outside AICW's folder is changed. Requires python3 and a network connection.</p></Confirm>}
      <Card title="Diagnostics" className="mt-4" actions={<Button onClick={async () => setDoctor(await api.Doctor())}>Run aicw doctor</Button>}>
        {doctor ? <pre className="max-h-72 overflow-auto text-xs">{doctor}</pre> : <p className="text-sm text-mute">Checks installation, PATH, data directory, database and agent CLIs.</p>}
      </Card>
    </>
  )
}
