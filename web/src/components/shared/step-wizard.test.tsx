import { describe, it, expect, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { StepWizard } from './step-wizard'

const steps = [
  { id: 'template', label: 'Template' },
  { id: 'tools', label: 'Tools' },
  { id: 'review', label: 'Review' },
]

describe('StepWizard', () => {
  it('renders steps and child content without crashing', () => {
    const { container } = render(
      <StepWizard steps={steps} activeStep="template" onStepChange={vi.fn()}>
        <div>Wizard body</div>
      </StepWizard>
    )

    expect(container).toBeTruthy()
    expect(screen.getByRole('button', { name: /Template/ })).not.toBeNull()
    expect(screen.getByRole('button', { name: /Tools/ })).not.toBeNull()
    expect(screen.getByRole('button', { name: /Review/ })).not.toBeNull()
    expect(screen.getByText('Wizard body')).not.toBeNull()
  })

  it('calls onStepChange when a step is clicked', () => {
    const onStepChange = vi.fn()

    render(
      <StepWizard steps={steps} activeStep="template" onStepChange={onStepChange}>
        <div>Wizard body</div>
      </StepWizard>
    )

    fireEvent.click(screen.getByRole('button', { name: /Review/ }))
    expect(onStepChange).toHaveBeenCalledWith('review')
  })

  it('shows completed step indicator icon for completed steps', () => {
    render(
      <StepWizard
        steps={steps}
        activeStep="tools"
        completedSteps={['template']}
        onStepChange={vi.fn()}
      >
        <div>Wizard body</div>
      </StepWizard>
    )

    const completedButton = screen.getByRole('button', { name: /Template/ })
    expect(completedButton.querySelector('svg')).not.toBeNull()
  })
})
