import { Link } from 'react-router-dom'
import { Empty } from './ui'

export function NoWorkspace() {
  return <Empty title="No workspace selected" hint="Run `aicw .` in a project directory, or pick one on the Workspaces page." action={<Link className="rounded-md bg-accent px-3 py-1.5 font-semibold text-bg" to="/workspaces">Choose workspace</Link>} />
}
