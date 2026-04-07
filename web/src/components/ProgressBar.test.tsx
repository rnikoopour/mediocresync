import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { ProgressBar } from './ProgressBar'

describe('ProgressBar', () => {
  it('renders the percentage label for default variant', () => {
    render(<ProgressBar percent={42} />)
    expect(screen.getByText('42%')).toBeInTheDocument()
  })

  it('renders a custom label when provided', () => {
    render(<ProgressBar percent={50} label="Half" />)
    expect(screen.getByText('Half')).toBeInTheDocument()
  })

  it('renders a dash and no percentage for loading variant', () => {
    render(<ProgressBar percent={0} variant="loading" />)
    expect(screen.getByText('—')).toBeInTheDocument()
    expect(screen.queryByText('%')).not.toBeInTheDocument()
  })
})
