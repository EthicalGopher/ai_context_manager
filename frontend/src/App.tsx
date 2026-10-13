import { useEffect, useMemo, useState } from 'react'
import { HashRouter, NavLink, Route, Routes, useLocation, useNavigate, useParams } from 'react-router-dom'
import { FiPlus, FiSearch, FiArrowUp, FiArrowDown, FiSettings, FiSun, FiMoon, FiZap, FiFolder, FiChevronUp, FiMoreHorizontal, FiTrash2 } from 'react-icons/fi'
import { api, errMsg, onEvent, type OpenResult } from './lib/api'
import { useStore, type Conv } from './store'
import { Confirm, ErrorBox, Seg } from './components/ui'
import Chat from './pages/Chat'
import Overview from './pages/Overview'
import Workspaces from './pages/Workspaces'
import FreshStart from './pages/FreshStart'
import ContextPage from './pages/Context'
import Skills from './pages/Skills'
import Agents from './pages/Agents'
import Sessions from './pages/Sessions'
import Settings from './pages/Settings'
import Media from './pages/Media'
import PagesView, { usePages } from './pages/Pages'

const more = [
  { to: '/overview', label: 'Project overview' },
  { to: '/workspaces', label: 'Workspaces' },
  { to: '/fresh-start', label: 'Fresh Start wizard' },
  { to: '/context', label: 'Context package' },
  { to: '/media', label: 'Media library' },
  { to: '/agents', label: 'AI agents' },
  { to: '/sessions', label: 'Session history' },
]

const group = (iso: string) => {
  const d = new Date(iso), now = new Date()
  const days = Math.floor((new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime() - new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()) / 86400000)
  return days <= 0 ? 'Today' : days === 1 ? 'Yesterday' : days <= 7 ? 'Previous 7 days' : 'Older'
}

function Sidebar() {
  const { open, setOpen, convs, setConvs, theme, setTheme, setError } = useStore()
  const navigate = useNavigate()
  const { id } = useParams()
  const loc = useLocation()
  const mode: 'chats' | 'pages' = loc.pathname.startsWith('/p') ? 'pages' : 'chats'
  const { pages, reload: reloadPages } = usePages()
  const [q, setQ] = useState('')
  const [menu, setMenu] = useState(false)
  const [picker, setPicker] = useState(false)
  const [del, setDel] = useState<Conv | null>(null)
  const [list, setList] = useState<any[]>([])
  const ws = open?.workspace

  useEffect(() => {
    if (!ws) { setConvs([]); return }
    let live = true
    const load = () => api.Conversations(ws.id).then((c: any) => live && setConvs(c ?? [])).catch(() => {})
    load()
    const t = setInterval(() => { if (!document.hidden) load() }, 4000) // picks up chats the agent starts itself
    return () => { live = false; clearInterval(t) }
  }, [ws?.id])

  const filtered = useMemo(() => convs.filter((c) => c.title.toLowerCase().includes(q.toLowerCase())), [convs, q])
  const groups = useMemo(() => {
    const g: Record<string, Conv[]> = {}
    filtered.forEach((c) => { (g[group(c.updatedAt)] ??= []).push(c) })
    return ['Today', 'Yesterday', 'Previous 7 days', 'Older'].filter((k) => g[k]).map((k) => [k, g[k]] as const)
  }, [filtered])

  const openWs = async (p: string) => { try { setOpen((await api.OpenWorkspace(p, false)) as OpenResult); setPicker(false); navigate('/') } catch (e) { setError(errMsg(e)) } }
  const newChat = () => navigate('/')
  const newPage = async () => {
    const title = window.prompt('Page title') ?? ''
    if (!title.trim() || !ws) return
    try { const p: any = await api.SavePage(ws.id, title.trim(), '', ''); await reloadPages(); navigate(`/p/${p.id}`) } catch (e) { setError(errMsg(e)) }
  }
  const move = async (i: number, d: number) => {
    const ids = pages.map((p) => p.id); const j = i + d
    if (!ws || j < 0 || j >= ids.length) return
    ;[ids[i], ids[j]] = [ids[j], ids[i]]
    try { await api.ReorderPages(ws.id, ids); reloadPages() } catch (e) { setError(errMsg(e)) }
  }
  const shownPages = pages.filter((p) => p.title.toLowerCase().includes(q.toLowerCase()))

  return (
    <aside aria-label="Chats" className="flex w-72 shrink-0 flex-col border-r border-line bg-panel">
      <div className="p-3">
        <div className="mb-2 flex justify-center">
          <Seg<'chats' | 'pages'> label="Sidebar mode" value={mode} onChange={(m) => navigate(m === 'pages' ? '/p' : '/')} options={[{ id: 'chats', label: 'Chats' }, { id: 'pages', label: 'Pages' }]} />
        </div>
        {mode === 'chats'
          ? <button onClick={newChat} className="flex w-full items-center gap-2 rounded-xl px-3 py-2.5 font-bold hover:bg-raised"><FiPlus aria-hidden /> New chat</button>
          : <button onClick={newPage} className="flex w-full items-center gap-2 rounded-xl px-3 py-2.5 font-bold hover:bg-raised"><FiPlus aria-hidden /> New page</button>}
        <label className="mt-1 flex items-center gap-2 rounded-xl px-3 py-2 text-mute focus-within:bg-raised hover:bg-raised"><FiSearch aria-hidden />
          <input value={q} onChange={(e) => setQ(e.target.value)} placeholder={mode === 'chats' ? 'Search chats' : 'Search pages'} aria-label={mode === 'chats' ? 'Search chats' : 'Search pages'} className="min-w-0 flex-1 border-0 bg-transparent p-0 text-fg shadow-none placeholder:text-mute" /></label>
      </div>
      {mode === 'pages' ? (
        <nav aria-label="Pages" className="min-h-0 flex-1 overflow-auto px-2">
          {ws && <div className="px-3 pb-2 pt-1"><div className="text-xs font-bold uppercase tracking-widest text-mute">{ws.name}</div></div>}
          {ws && pages.length === 0 && <p className="px-3 py-2 text-sm text-mute">No pages yet. Create one, save a chat message to a page, or let your AI write them.</p>}
          <ul>{shownPages.map((p, i) => (
            <li key={p.id} className="group relative">
              <NavLink to={`/p/${p.id}`} className={({ isActive }) => `flex items-center gap-3 truncate rounded-xl px-3 py-2.5 pr-14 ${isActive || (loc.pathname === '/p' && i === 0) ? 'bg-raised text-fg' : 'text-fg/85 hover:bg-raised/70'}`}>
                <span className="text-[11px] font-bold text-mute">{String(pages.indexOf(p) + 1).padStart(2, '0')}</span><span className="truncate">{p.title}</span></NavLink>
              <span className="absolute right-1 top-1/2 hidden -translate-y-1/2 gap-0.5 group-hover:flex">
                <button aria-label={`Move ${p.title} up`} className="rounded p-1 text-mute hover:text-fg" onClick={() => move(pages.indexOf(p), -1)}><FiArrowUp aria-hidden size={13} /></button>
                <button aria-label={`Move ${p.title} down`} className="rounded p-1 text-mute hover:text-fg" onClick={() => move(pages.indexOf(p), 1)}><FiArrowDown aria-hidden size={13} /></button></span>
            </li>))}</ul>
        </nav>
      ) : (
      <nav aria-label="Conversations" className="min-h-0 flex-1 overflow-auto px-2">
        {!ws && <p className="px-3 py-2 text-sm text-mute">Open a project to see its chats.</p>}
        {ws && convs.length === 0 && <p className="px-3 py-2 text-sm text-mute">No chats yet. Start one above.</p>}
        {groups.map(([name, items]) => (
          <div key={name} className="mb-3">
            <h3 className="px-3 py-1 text-xs font-bold text-mute">{name}</h3>
            <ul>{items.map((c) => (
              <li key={c.id} className="group relative">
                <NavLink to={`/c/${c.id}`} className={({ isActive }) => `block truncate rounded-xl px-3 py-2 pr-9 ${isActive || String(c.id) === id ? 'bg-raised text-fg' : 'text-fg/85 hover:bg-raised/70'}`}>{c.title}</NavLink>
                <button aria-label={`Delete ${c.title}`} className="absolute right-2 top-1/2 hidden -translate-y-1/2 rounded p-1 text-mute hover:text-bad group-hover:block" onClick={() => setDel(c)}><FiTrash2 aria-hidden size={14} /></button>
              </li>))}</ul>
          </div>))}
      </nav>)}
      <div className="relative border-t border-line p-2">
        {menu && (
          <div className="absolute bottom-full left-2 right-2 mb-1 rounded-xl border border-line bg-panel p-1 shadow-2xl" role="menu">
            {more.map((m) => <NavLink key={m.to} role="menuitem" to={m.to} onClick={() => setMenu(false)} className="block rounded-lg px-3 py-2 text-mute hover:bg-raised hover:text-fg">{m.label}</NavLink>)}
          </div>)}
        {picker && (
          <div className="absolute bottom-full left-2 right-2 mb-1 max-h-72 overflow-auto rounded-xl border border-line bg-panel p-1 shadow-2xl" role="listbox" aria-label="Workspaces">
            <button className="flex w-full items-center gap-2 rounded-lg px-3 py-2 font-bold hover:bg-raised" onClick={async () => { try { const p = await api.PickDirectory(); if (p) openWs(p) } catch (e) { setError(errMsg(e)) } }}><FiFolder aria-hidden /> Open folder…</button>
            {list.map((w) => <button key={w.workspace.id} role="option" aria-selected={w.workspace.id === ws?.id} className="block w-full truncate rounded-lg px-3 py-2 text-left text-mute hover:bg-raised hover:text-fg" title={w.workspace.path} onClick={() => openWs(w.workspace.path)}>{w.workspace.name}</button>)}
          </div>)}
        <button className="flex w-full items-center gap-2 rounded-xl px-3 py-2 text-left hover:bg-raised" aria-expanded={picker} onClick={async () => { setMenu(false); setPicker(!picker); if (!picker) setList(((await api.WorkspaceList()) as any) ?? []) }}>
          <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-accent text-sm font-extrabold text-white" aria-hidden>{(ws?.name ?? '?').slice(0, 1).toUpperCase()}</span>
          <span className="min-w-0 flex-1"><span className="block truncate text-sm font-bold">{ws?.name ?? 'No project'}</span><span className="block truncate text-xs text-mute">{ws?.path ?? 'Click to open a folder'}</span></span><FiChevronUp aria-hidden className="text-mute" />
        </button>
        <div className="mt-1 flex items-center gap-1">
          <NavLink to="/skills" className={({ isActive }) => `flex flex-1 items-center gap-2 rounded-xl px-3 py-2 text-sm ${isActive ? 'bg-raised' : 'text-mute hover:bg-raised hover:text-fg'}`}><FiZap aria-hidden /> Skills & memory</NavLink>
          <NavLink to="/settings" aria-label="Settings" title="Settings" className={({ isActive }) => `rounded-xl p-2.5 ${isActive ? 'bg-raised' : 'text-mute hover:bg-raised hover:text-fg'}`}><FiSettings aria-hidden /></NavLink>
          <button aria-label="Toggle theme" title="Toggle theme" className="rounded-xl p-2.5 text-mute hover:bg-raised hover:text-fg" onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}>{theme === 'dark' ? <FiMoon aria-hidden /> : <FiSun aria-hidden />}</button>
          <button aria-label="More pages" title="More" aria-expanded={menu} className="rounded-xl p-2.5 text-mute hover:bg-raised hover:text-fg" onClick={() => { setPicker(false); setMenu(!menu) }}><FiMoreHorizontal aria-hidden /></button>
        </div>
      </div>
      {del && <Confirm title="Delete this chat?" danger confirmLabel="Delete" onCancel={() => setDel(null)} onConfirm={async () => {
        try { await api.DeleteConversation(del.id, true); setConvs(convs.filter((c) => c.id !== del.id)); if (String(del.id) === id) navigate('/') } catch (e) { setError(errMsg(e)) } finally { setDel(null) }
      }}><p className="text-sm">Deletes “{del.title}”, its messages, and stops its terminal if one is running. Files in your project and the media library are not deleted.</p></Confirm>}
    </aside>
  )
}

function Shell() {
  const { setOpen, error, setError, theme, notice, setNotice } = useStore()
  const navigate = useNavigate()
  const [ready, setReady] = useState(false)
  const go = async (page?: string, wsId?: number) => {
    if (page === 'terminal' || page === 'chat') {
      try { if (page === 'terminal') localStorage.setItem('aicw-term-open', '1') } catch { /* storage unavailable */ }
      const id = wsId ?? useStore.getState().open?.workspace.id
      const list: any[] = id ? ((await api.Conversations(id)) as any) ?? [] : []
      navigate(list.length ? `/c/${list[0].id}` : '/')
      return
    }
    navigate(page === 'pages' ? '/p' : page ? '/' + page : '/')
  }

  useEffect(() => {
    document.documentElement.classList.toggle('light', theme === 'light')
    api.Bootstrap().then((b: any) => {
      if (b.initial) {
        setOpen(b.initial as OpenResult)
        if (b.initial.page) go(b.initial.page, b.initial.workspace.id)
        else if (b.initial.requestedNew) navigate('/')
      }
      if (b.initialError) setError(b.initialError)
    }).catch((e) => setError(errMsg(e))).finally(() => setReady(true))
    const offOpen = onEvent('workspace:open', (r: OpenResult) => {
      setOpen(r)
      setNotice(`Opened ${r.workspace.path} in the running window.`)
      if (r.page) go(r.page, r.workspace.id); else navigate('/')
    })
    const offErr = onEvent('app:error', (m: string) => setError(m))
    return () => { offOpen(); offErr() }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const page = (el: React.ReactElement) => <div className="h-full overflow-auto p-6">{el}</div>
  return (
    <div className="flex h-full">
      <a href="#main" className="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:bg-accent focus:p-2 focus:text-white">Skip to content</a>
      <Routes>
        <Route path="/c/:id" element={<Sidebar />} />
        <Route path="*" element={<Sidebar />} />
      </Routes>
      <main id="main" className="relative min-h-0 min-w-0 flex-1">
        {(error || notice) && (
          <div className="absolute inset-x-0 top-0 z-30 mx-auto max-w-3xl p-3">
            <ErrorBox error={error} onDismiss={() => setError('')} />
            {notice && <div role="status" className="flex justify-between rounded-xl border border-accent/40 bg-panel px-3 py-2 text-sm shadow-lg">{notice}<button className="underline" onClick={() => setNotice('')}>Dismiss</button></div>}
          </div>)}
        {ready && (
          <Routes>
            <Route path="/" element={<Chat />} />
            <Route path="/c/:id" element={<Chat />} />
            <Route path="/p" element={<PagesView />} />
            <Route path="/p/:id" element={<PagesView />} />
            <Route path="/overview" element={page(<Overview />)} />
            <Route path="/workspaces" element={page(<Workspaces />)} />
            <Route path="/fresh-start" element={page(<FreshStart />)} />
            <Route path="/context" element={page(<ContextPage />)} />
            <Route path="/media" element={page(<Media />)} />
            <Route path="/skills" element={page(<Skills />)} />
            <Route path="/agents" element={page(<Agents />)} />
            <Route path="/sessions" element={page(<Sessions />)} />
            <Route path="/settings" element={page(<Settings />)} />
          </Routes>
        )}
      </main>
    </div>
  )
}

export default function App() {
  return <HashRouter><Shell /></HashRouter>
}
