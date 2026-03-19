import { api } from '../api/client'
import { FileBrowser } from './FileBrowser'

interface Props {
  connectionId: string
  initialPath?: string
  onSelect: (path: string) => void
  onClose: () => void
}

export function RemoteBrowser({ connectionId, initialPath = '/', onSelect, onClose }: Props) {
  return (
    <FileBrowser
      title="Browse Remote"
      initialPath={initialPath}
      queryKey={['browse', 'remote', connectionId]}
      fetcher={(path) => api.connections.browse(connectionId, path)}
      onSelect={onSelect}
      onClose={onClose}
    />
  )
}
