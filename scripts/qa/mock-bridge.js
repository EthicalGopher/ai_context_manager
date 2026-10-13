// Visual-QA only: a fake Wails bridge with sample data so the real frontend can be
// screenshotted in headless Chrome. Not shipped in the app.
(function () {
  const now = new Date().toISOString()
  const ws = { id: 1, path: '/home/dev/my-dream-game', name: 'My Dream Game', agent: 'claude', settings: '{}', lastOpened: now }
  const media = [
    { id: 1, name: 'pig-reference.png', kind: 'image', mime: 'image/png', size: 53400, note: '', inContext: true, addedAt: now },
    { id: 2, name: 'level-1.png', kind: 'image', mime: 'image/png', size: 22000, note: '', inContext: false, addedAt: now },
    { id: 3, name: 'walk-cycle.mp4', kind: 'video', mime: 'video/mp4', size: 21800, note: '', inContext: false, addedAt: now },
  ]
  const body = `## Meet Mr. Mak.\n\nKeep the pink nose and enormous ears. Everything else starts with a clear silhouette.\n\n![Selected visual reference · GPT Image 2 edit](media:1)\n\n### Selected direction\n\n## The little explorer\n\nA white piglet with a grey beanie, a tiny backpack and just enough confidence to try the next jump.\n\n> Read at a distance: the beanie, the ears and the backpack must stay readable at thumbnail size.\n\n![Walk cycle test](media:3)\n\n- Pink nose and big ears stay\n- Grey beanie, brown backpack\n- Calm, curious posture\n\n![Level one blockout](media:2)\n`
  const pages = [
    { id: 1, workspaceId: 1, title: 'Intro screen', summary: 'A small adventure.', body: '## Intro\n\nHello.', sort: 1, updatedAt: now, updatedBy: 'user' },
    { id: 2, workspaceId: 1, title: 'Character', summary: 'Keep the pink nose and enormous ears.', body, sort: 2, updatedAt: now, updatedBy: 'agent' },
    { id: 3, workspaceId: 1, title: 'About', summary: '', body: '', sort: 3, updatedAt: now, updatedBy: 'agent' },
    { id: 4, workspaceId: 1, title: 'Development', summary: '', body: 'Notes.', sort: 4, updatedAt: now, updatedBy: 'user' },
  ]
  const convs = [
    { id: 1, workspaceId: 1, title: 'Design the hero character', agent: 'claude', sessionId: 1, messages: 3, updatedAt: now },
    { id: 2, workspaceId: 1, title: 'Release checklist (started by the agent)', agent: 'claude', sessionId: 0, messages: 1, updatedAt: now },
  ]
  const msgs = [
    { id: 1, conversationId: 1, role: 'user', source: 'ui', body: 'Here is my sketch. Make a low-poly white piglet with a grey beanie and a tiny backpack, then save the final design as a page.', createdAt: now, media: [media[1]] },
    { id: 2, conversationId: 1, role: 'assistant', source: 'agent', body: 'I read the **project memory** first, then drafted the character.\n\n- Low-poly, white body, pink nose\n- Grey beanie, brown backpack\n- Saved it to the **Character** page with the reference image\n\n```\naicw tool write_page ...\n```', createdAt: now, media: [media[0], media[2]] },
    { id: 3, conversationId: 1, role: 'user', source: 'ui', body: 'Great. Add a locations page too.', createdAt: now, media: [] },
  ]
  const skills = [
    { id: 1, workspaceId: 1, name: 'read-context-first', description: 'Always load project memory before working', body: 'At the start of every session call read_context and read every note.', alwaysOn: true, updatedBy: 'user', updatedAt: now },
    { id: 2, workspaceId: 1, name: 'keep-memory-updated', description: 'Record progress and decisions in notes', body: 'After each task update project-memory.', alwaysOn: true, updatedBy: 'agent', updatedAt: now },
  ]
  const agents = ['claude:Claude Code', 'codex:OpenAI Codex CLI', 'kilo:Kilo', 'agy:AGY'].map((x) => { const [id, displayName] = x.split(':'); return { profile: { id, displayName, verified: id !== 'codex', handoff: 'prompt-arg', capabilities: {} }, status: { state: id === 'codex' ? 'Not installed' : 'Detected', path: '/usr/bin/' + id, version: '1.0' } } })
  const dir = [
    { name: 'assets', rel: 'assets', dir: true, size: 0, secret: false }, { name: 'docs', rel: 'docs', dir: true, size: 0, secret: false }, { name: 'src', rel: 'src', dir: true, size: 0, secret: false },
    { name: '.env', rel: '.env', dir: false, size: 40, secret: true }, { name: 'AGENTS.md', rel: 'AGENTS.md', dir: false, size: 1200, secret: false }, { name: 'README.md', rel: 'README.md', dir: false, size: 2100, secret: false },
  ]
  const data = {
    Bootstrap: { version: 'qa', dataDir: '/tmp', initial: { workspace: ws, sessions: [], resumable: [], action: 'new', isNew: false, requestedNew: false, reason: '' }, initialError: '', settings: { theme: 'dark' } },
    Conversations: convs, Messages: (id, after) => msgs.filter((m) => m.conversationId === id && m.id > after), ListAgents: agents, TermStatus: null, ChatStatus: { running: false, queued: 0 },
    GetSettings: { defaultAgent: 'claude', chatEdits: false, budget: 8000, maxFileKB: 512, ignoreDirs: [], ignoreGlobs: [], agentPaths: {}, pinned: [], voiceModel: 'base' },
    ListPages: pages, ListMedia: media, ListDir: dir, WorkspaceList: [{ workspace: ws, sessions: 2, resumable: 1, pinned: false }], ListAgentSkills: skills,
    InstructionsStatus: [{ file: 'AGENTS.md', exists: true, hasBlock: true, current: true }, { file: 'CLAUDE.md', exists: false, hasBlock: false, current: false }, { file: 'GEMINI.md', exists: false, hasBlock: false, current: false }],
    StarterSkills: [], ListNotes: [{ name: 'project-memory', body: '# Memory\n- Low-poly style', updatedBy: 'agent', updatedAt: now }], NoteVersions: [], VoiceStatus: { recorder: 'pw-record', engine: 'faster-whisper (base, offline)', ready: true, hint: '' },
    ListTerminals: ['konsole'], RecentWorkspaces: [ws], SkillsOverview: null, ReadFile: '# README\nHello',
  }
  if (location.hash.includes('term')) { // show the xterm view with sample output
    try { localStorage.setItem('aicw-term-open', '1') } catch (e) {}
    data.TermStatus = { id: 't1' }
    const demo = '\x1b[1;36maicw\x1b[0m $ claude\r\n\x1b[32m●\x1b[0m Read project context via \x1b[1mread_context\x1b[0m\r\n\x1b[33m●\x1b[0m write_page \"Character\" \x1b[2m(2 images)\x1b[0m\r\n\x1b[32m✔\x1b[0m Saved. Open Pages to see it.\r\n\r\n> '
    data.TermBuffer = btoa(unescape(encodeURIComponent(demo)))
  } else { try { localStorage.removeItem('aicw-term-open') } catch (e) {} }
  const svc = new Proxy({}, { get: (_t, name) => (...a) => { const v = data[name]; return Promise.resolve(typeof v === 'function' ? v(...a) : v === undefined ? [] : JSON.parse(JSON.stringify(v))) } })
  document.addEventListener('DOMContentLoaded', () => { const st = document.createElement('style'); st.textContent = '*{animation:none!important;transition:none!important}'; document.head.appendChild(st) })
  window.go = { service: { Service: svc } }
  window.runtime = new Proxy({}, { get: () => () => () => {} })
})()
