import { api } from '../api/client'
import { FileBrowser } from './FileBrowser'

interface Props {
  sourceId: string
  initialPath?: string
  onSelect: (path: string) => void
  onClose: () => void
}

export function RemoteBrowser({ sourceId, initialPath = '/', onSelect, onClose }: Props) {
  return (
    <FileBrowser
      title="Browse Remote"
      initialPath={initialPath}
      queryKey={['browse', 'remote', sourceId]}
      fetcher={(path) => api.sources.browse(sourceId, path)}
      onSelect={onSelect}
      onClose={onClose}
    />
  )
}
