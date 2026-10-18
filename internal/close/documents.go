package close

import (
	"context"
	"fmt"

	"fingo/internal/document"
)

// RepositorySource answers the close's question about open documents from the
// AR/AP document book.
//
// It is the simple implementation: the book is read and its documents are
// filtered here. A client's own database is small enough that this is the whole
// of it, and a store that can answer the question with a query of its own should
// implement Documents directly rather than go through this.
type RepositorySource struct {
	documents document.Repository
}

// NewRepositorySource adapts a document repository to the port a close asks
// through.
func NewRepositorySource(documents document.Repository) *RepositorySource {
	return &RepositorySource{documents: documents}
}

// OpenInPeriod returns the period's documents that are still owed.
//
// A document is open while anything on it is outstanding. One that has been
// settled in full is not owed, and one that has been voided is owed to nobody —
// so neither holds a period open, and only an invoice somebody is still waiting
// to be paid for does.
func (s *RepositorySource) OpenInPeriod(ctx context.Context, periodID string) ([]document.Document, error) {
	all, err := s.documents.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}

	open := make([]document.Document, 0, len(all))
	for _, d := range all {
		if d.PeriodID != periodID {
			continue
		}
		if d.Status == document.StatusVoid || d.Settled() {
			continue
		}

		open = append(open, d)
	}

	return open, nil
}
