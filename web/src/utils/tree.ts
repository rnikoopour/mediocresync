// Folders first, then files, each group alpha-sorted by name.
export function sortNodes<T extends { type: string; name: string }>(nodes: T[]): T[] {
  return [...nodes].sort((a, b) => {
    if (a.type !== b.type) return a.type === 'folder' ? -1 : 1
    return a.name.localeCompare(b.name)
  })
}

type PathFolder<L> = { type: 'folder'; name: string; children: Array<L | PathFolder<L>> }

// Builds a nested folder/file tree from a flat list of items with remote_path,
// stripping the remotePath prefix and sorting folders before files at each level.
export function buildPathTree<T extends { remote_path: string }, L extends { type: 'file'; name: string }>(
  items: T[],
  remotePath: string,
  makeLeaf: (item: T, name: string) => L,
): Array<L | PathFolder<L>> {
  const base = remotePath.replace(/\/+$/, '')
  const root: PathFolder<L> = { type: 'folder', name: '', children: [] }

  for (const item of items) {
    const rel = item.remote_path.startsWith(base + '/')
      ? item.remote_path.slice(base.length + 1)
      : item.remote_path
    const segments = rel.split('/').filter(Boolean)
    if (segments.length === 0) continue

    let cur = root
    for (let i = 0; i < segments.length - 1; i++) {
      const seg = segments[i]
      let child = cur.children.find((c): c is PathFolder<L> => c.type === 'folder' && c.name === seg)
      if (!child) {
        child = { type: 'folder', name: seg, children: [] }
        cur.children.push(child)
      }
      cur = child
    }
    cur.children.push(makeLeaf(item, segments[segments.length - 1]))
  }

  function sortFolder(folder: PathFolder<L>) {
    folder.children = sortNodes(folder.children)
    folder.children.forEach((c) => { if (c.type === 'folder') sortFolder(c as PathFolder<L>) })
  }
  sortFolder(root)

  return root.children
}
