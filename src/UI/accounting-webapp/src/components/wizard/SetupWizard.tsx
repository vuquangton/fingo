"use client";

import React, { useState } from "react";
import { WizardLayout } from "./WizardLayout";

export function SetupWizard() {
  const [step, setStep] = useState(1);

  const nextStep = () => setStep((s) => Math.min(s + 1, 8));
  const prevStep = () => setStep((s) => Math.max(s - 1, 1));

  return (
    <>
      {step === 1 && (
        <WizardLayout
          currentStep={1}
          totalSteps={8}
          title="Chào m?ng"
          onNext={nextStep}
          nextLabel="B?t d?u"
        >
          <div className="space-y-6">
            <h1 className="text-display text-primary">K? toán nhanh, chính xác, dúng h?n.</h1>
            <p className="text-text-secondary">B?n là ai?</p>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              {["Ch? doanh nghi?p", "K? toán tru?ng", "K? toán viên"].map((role) => (
                <div key={role} className="border border-border p-4 rounded-md hover:border-primary cursor-pointer transition-colors">
                  <h3 className="font-medium text-text-primary">{role}</h3>
                </div>
              ))}
            </div>
          </div>
        </WizardLayout>
      )}
      
      {step === 2 && (
        <WizardLayout
          currentStep={2}
          totalSteps={8}
          title="Công ty"
          onBack={prevStep}
          onNext={nextStep}
        >
          <div className="space-y-4">
            <label className="block text-sm font-medium text-text-primary">Mã s? thu? (MST)</label>
            <div className="flex gap-2">
              <input type="text" className="flex-1 border border-border p-2 rounded-md focus:outline-none focus:ring-2 focus:ring-border-focus" placeholder="Nh?p 10 ho?c 13 ch? s?" />
              <button className="px-4 py-2 bg-secondary text-surface rounded-md hover:opacity-90">Tìm ki?m</button>
            </div>
          </div>
        </WizardLayout>
      )}
      
      {step > 2 && (
        <WizardLayout
          currentStep={step}
          totalSteps={8}
          title={`Bu?c ${step}`}
          onBack={prevStep}
          onNext={step < 8 ? nextStep : undefined}
          nextLabel={step === 7 ? "Hoàn t?t" : "Ti?p theo"}
        >
          <div className="py-12 text-center text-text-secondary">
            N?i dung bu?c {step} dang du?c xây d?ng...
          </div>
        </WizardLayout>
      )}
    </>
  );
}

