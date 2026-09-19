import React from "react";

interface WizardLayoutProps {
  currentStep: number;
  totalSteps: number;
  title: string;
  onBack?: () => void;
  onNext?: () => void;
  isNextDisabled?: boolean;
  nextLabel?: string;
  children: React.ReactNode;
}

export function WizardLayout({
  currentStep,
  totalSteps,
  title,
  onBack,
  onNext,
  isNextDisabled,
  nextLabel = "Ti?p theo",
  children,
}: WizardLayoutProps) {
  return (
    <div className="min-h-screen bg-surface-alt flex flex-col items-center py-12 px-4 sm:px-6 lg:px-8 font-sans">
      <div className="w-full max-w-3xl">
        <div className="bg-surface shadow-md rounded-lg overflow-hidden transition-shadow hover:shadow-lg duration-200">
          
          {/* Header */}
          <div className="px-6 py-4 border-b border-border flex items-center justify-between">
            <div className="flex items-center gap-4">
              {onBack && (
                <button
                  onClick={onBack}
                  className="text-text-secondary hover:text-text-primary transition-colors cursor-pointer"
                >
                  &larr; Quay l?i
                </button>
              )}
              <h2 className="text-heading-sm font-semibold text-text-primary">
                [{currentStep}/{totalSteps}] {title}
              </h2>
            </div>
          </div>

          {/* Body */}
          <div className="p-6 text-body">
            {children}
          </div>

          {/* Footer */}
          <div className="px-6 py-4 bg-surface-alt border-t border-border flex items-center justify-between">
            <div>
               <button className="text-text-secondary hover:text-text-primary text-body-sm transition-colors cursor-pointer">
                  Luu & làm sau
               </button>
            </div>
            <div className="flex gap-4">
              {onBack && (
                <button
                  onClick={onBack}
                  className="px-4 py-2 border border-border text-text-primary rounded-md hover:bg-surface-alt transition-colors cursor-pointer"
                >
                  Quay l?i
                </button>
              )}
              {onNext && (
                <button
                  onClick={onNext}
                  disabled={isNextDisabled}
                  className={`px-4 py-2 rounded-md transition-colors cursor-pointer text-surface ${
                    isNextDisabled
                      ? "bg-text-disabled cursor-not-allowed"
                      : "bg-primary hover:bg-primary-hover active:bg-primary-active focus:ring-2 focus:ring-border-focus"
                  }`}
                >
                  {nextLabel} &rarr;
                </button>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

