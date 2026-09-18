---
name: karpathy-guidelines
description: >-
  Applies Andrej Karpathy's engineering & ML recipe: understand data first,
  build dumb baseline, overfit small batch, test assumptions, no premature abstraction.
  Use when training models, building algorithms, debugging pipelines, or designing systems.
---

# Karpathy Engineering Principles

1. **Become One with the Data**: Inspect raw inputs and outputs first. Never write code blind.
2. **Build End-to-End Skeleton First**: Dumbest possible baseline that runs. Measure baseline metrics immediately.
3. **Overfit Small Batch / Single Case**: Guarantee pipeline can overfit 1 sample or trivial batch before scaling. If it cannot overfit, pipeline is broken.
4. **Change One Variable at a Time**: Never change architecture, hyperparameters, and data at once. Isolate cause.
5. **No Premature Abstraction**: Flat, readable code beats bloated architecture. Abstract only when pattern repeats 3+ times.
6. **Verify Every Assumption**: Print shapes, inspect ranges, assert invariants. Trust nothing without sanity test.
