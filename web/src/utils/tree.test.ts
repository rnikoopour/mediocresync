import { describe, it, expect } from 'vitest'
import { sortNodes } from './tree'

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
