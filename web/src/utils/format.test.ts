import { describe, it, expect } from 'vitest'
import { formatBytes, formatSpeed, formatDuration } from './format'

describe('formatBytes', () => {
  it.each([
    { bytes: 0,             expected: '0 B'      },
    { bytes: 500,           expected: '500 B'    },
    { bytes: 1_023,         expected: '1023 B'   },
    { bytes: 1_024,         expected: '1.0 KB'   },
    { bytes: 2_048,         expected: '2.0 KB'   },
    { bytes: 1_047_552,     expected: '1023.0 KB' },
    { bytes: 1_048_576,     expected: '1.0 MB'   },
    { bytes: 5_242_880,     expected: '5.0 MB'   },
    { bytes: 1_073_741_824, expected: '1.0 GB'   },
    { bytes: 2_147_483_648, expected: '2.0 GB'   },
  ])('$bytes bytes → $expected', ({ bytes, expected }) => {
    expect(formatBytes(bytes)).toBe(expected)
  })
})

describe('formatSpeed', () => {
  it.each([
    { bps: 1_024,     expected: '1.0 KB/s'  },
    { bps: 1_048_576, expected: '1.0 MB/s'  },
  ])('$bps bps → $expected', ({ bps, expected }) => {
    expect(formatSpeed(bps)).toBe(expected)
  })
})

describe('formatDuration', () => {
  it.each([
    { ms: 0,         expected: '0s'       },
    { ms: 45_000,    expected: '45s'      },
    { ms: 59_999,    expected: '59s'      },
    { ms: 60_000,    expected: '1m 0s'    },
    { ms: 90_000,    expected: '1m 30s'   },
    { ms: 3_599_000, expected: '59m 59s'  },
    { ms: 3_600_000, expected: '1h 0m 0s' },
    { ms: 3_661_000, expected: '1h 1m 1s' },
    { ms: 7_322_000, expected: '2h 2m 2s' },
  ])('$ms ms → $expected', ({ ms, expected }) => {
    expect(formatDuration(ms)).toBe(expected)
  })
})
