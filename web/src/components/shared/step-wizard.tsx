import { type ReactNode } from 'react'
import { CheckCircle } from 'lucide-react'
import { cn } from '../../lib/utils'

interface StepItem {
  id: string
  label: string
  icon?: ReactNode
}

interface StepWizardProps {
  steps: StepItem[]
  activeStep: string
  onStepChange: (stepId: string) => void
  completedSteps?: string[]
  children: ReactNode
}

export function StepWizard({
  steps,
  activeStep,
  onStepChange,
  completedSteps = [],
  children,
}: StepWizardProps) {
  const completedSet = new Set(completedSteps)

  return (
    <div className="flex flex-col gap-[18px]">
      <div className="flex items-center overflow-x-auto rounded-[var(--card-radius)] border border-[var(--color-border-default)] bg-[var(--color-surface-card)] px-[14px] py-3">
        {steps.map((step, index) => {
          const isCompleted = completedSet.has(step.id)
          const isActive = step.id === activeStep

          return (
            <div key={step.id} className="flex min-w-0 items-center">
              <button
                type="button"
                onClick={() => onStepChange(step.id)}
                className="flex min-w-0 cursor-pointer items-center gap-2.5 border-none bg-transparent p-0"
              >
                <span
                  className={cn(
                    'inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full border text-xs font-bold',
                    isActive
                      ? 'border-[var(--color-brand-gold)] bg-[rgba(255,215,0,0.14)]'
                      : 'border-[var(--color-border-default)] bg-transparent',
                    isCompleted || isActive
                      ? 'text-[var(--color-brand-gold)]'
                      : 'text-[var(--color-text-secondary)]'
                  )}
                >
                  {isCompleted ? <CheckCircle size={15} /> : step.icon ?? index + 1}
                </span>
                <span
                  className={cn(
                    'whitespace-nowrap text-[13px]',
                    isActive
                      ? 'font-bold text-[var(--color-brand-gold)]'
                      : 'font-semibold text-[var(--color-text-secondary)]'
                  )}
                >
                  {step.label}
                </span>
              </button>

              {index < steps.length - 1 && (
                <span
                  className={cn(
                    'mx-3 h-px w-11 shrink-0',
                    completedSet.has(step.id)
                      ? 'bg-[var(--color-brand-gold)]'
                      : 'bg-[var(--color-border-default)]'
                  )}
                />
              )}
            </div>
          )
        })}
      </div>

      <div>{children}</div>
    </div>
  )
}
