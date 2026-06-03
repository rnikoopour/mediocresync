import { describe, it, expect } from 'vitest'
import { sortNodes, buildPathTree } from './tree'

describe('sortNodes', () => {
  it('places folders before files', () => {
    const nodes = [
      { type: 'file',   name: 'a.txt' },
      { type: 'folder', name: 'z-dir' },
    ]
    const result = sortNodes(nodes)
    expect(result[0].type).toBe('folder')
    expect(result[1].type).toBe('file')
  })

  it('sorts folders alphabetically among themselves', () => {
    const nodes = [
      { type: 'folder', name: 'z-dir' },
      { type: 'folder', name: 'a-dir' },
      { type: 'folder', name: 'm-dir' },
    ]
    const result = sortNodes(nodes).map((n) => n.name)
    expect(result).toEqual(['a-dir', 'm-dir', 'z-dir'])
  })

  it('sorts files alphabetically among themselves', () => {
    const nodes = [
      { type: 'file', name: 'z.txt' },
      { type: 'file', name: 'a.txt' },
      { type: 'file', name: 'm.txt' },
    ]
    const result = sortNodes(nodes).map((n) => n.name)
    expect(result).toEqual(['a.txt', 'm.txt', 'z.txt'])
  })

  it('does not mutate the input array', () => {
    const nodes = [
      { type: 'file',   name: 'b.txt' },
      { type: 'folder', name: 'a-dir' },
    ]
    const original = [...nodes]
    sortNodes(nodes)
    expect(nodes).toEqual(original)
  })
})

describe('buildPathTree', () => {
  const leaf = (item: { remote_path: string }, name: string) => ({ type: 'file' as const, name, path: item.remote_path })

  it('builds a flat list of files with no nested folders', () => {
    const items = [
      { remote_path: '/root/a.txt' },
      { remote_path: '/root/b.txt' },
    ]
    const result = buildPathTree(items, '/root', leaf)
    expect(result).toHaveLength(2)
    expect(result.map((n) => n.name)).toEqual(['a.txt', 'b.txt'])
    expect(result.every((n) => n.type === 'file')).toBe(true)
  })

  it('nests files into subfolders', () => {
    const items = [
      { remote_path: '/root/dir/file.txt' },
    ]
    const result = buildPathTree(items, '/root', leaf)
    expect(result).toHaveLength(1)
    const folder = result[0] as { type: 'folder'; name: string; children: unknown[] }
    expect(folder.type).toBe('folder')
    expect(folder.name).toBe('dir')
    expect(folder.children).toHaveLength(1)
  })

  it('sorts folders before files at each level', () => {
    const items = [
      { remote_path: '/root/z.txt' },
      { remote_path: '/root/a-dir/file.txt' },
    ]
    const result = buildPathTree(items, '/root', leaf)
    expect(result[0].type).toBe('folder')
    expect(result[1].type).toBe('file')
  })

  it('handles items whose path does not start with the base prefix', () => {
    const items = [{ remote_path: 'other/file.txt' }]
    const result = buildPathTree(items, '/root', leaf)
    expect(result).toHaveLength(1)
    const folder = result[0] as { type: 'folder'; name: string; children: unknown[] }
    expect(folder.name).toBe('other')
  })

  it('skips items whose path resolves to empty after stripping the base', () => {
    const items = [{ remote_path: '/root/' }]
    const result = buildPathTree(items, '/root', leaf)
    expect(result).toHaveLength(0)
  })

  it('merges files sharing the same parent folder', () => {
    const items = [
      { remote_path: '/root/dir/a.txt' },
      { remote_path: '/root/dir/b.txt' },
    ]
    const result = buildPathTree(items, '/root', leaf)
    expect(result).toHaveLength(1)
    const folder = result[0] as { type: 'folder'; name: string; children: unknown[] }
    expect(folder.children).toHaveLength(2)
  })
})
