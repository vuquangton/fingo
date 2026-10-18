package web

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"fingo/internal/books"
)

// ExportHandover streams one client's books as a zip of CSV files.
//
// It is the handover artefact: a successor accountant receives the client's rows,
// not a backup of the practice's database. Row-level security does the filtering,
// so the export cannot contain another client by construction.
func (a *Auth) ExportHandover(c *gin.Context) {
	scope, _, ok := a.requireScope(c)
	if !ok {
		return
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition",
		fmt.Sprintf(`attachment; filename="fingo-handover-%s.zip"`, scope.ID()[:8]))
	c.Status(http.StatusOK)

	if err := books.Open(a.store, scope).Export(c.Request.Context(), c.Writer); err != nil {
		// The headers are already sent, so the only honest report is the log and a
		// broken download the operator will notice.
		_ = c.Error(err)
	}
}
