import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { StatusTag } from './StatusTag'

describe('StatusTag', () => {
  it('renders the status value', () => {
    render(<StatusTag value="UP" />)
    expect(screen.getByText('UP')).toBeInTheDocument()
  })
})
