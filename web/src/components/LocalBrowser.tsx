import { api } from '../api/client'
import { FileBrowser } from './FileBrowser'

interface Props {
  initialPath?: string
  onSelect: (path: string) => void
  onClose: () => void
}

export function LocalBrowser({ initialPath = '/', onSelect, onClose }: Props) {
  return (
    <FileBrowser
      title="Browse Local"
      initialPath={initialPath}
      queryKey={['browse', 'local']}
      fetcher={(path) => api.local.browse(path)}
      onSelect={onSelect}
      onClose={onClose}
    />
  )
}
