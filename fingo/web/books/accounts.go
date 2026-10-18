package books

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"fingo/internal/domain/gl"
	"fingo/web"
)

// Accounts returns the chart of accounts, by code.
func (b *Books) Accounts(_ context.Context) ([]web.AccountRow, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.accountsLocked(), nil
}

func (b *Books) accountsLocked() []web.AccountRow {
	codes := append([]string(nil), b.codes...)
	sort.Strings(codes)

	out := make([]web.AccountRow, 0, len(codes))

	for _, code := range codes {
		account, known := b.chart.ByCode(code)
		if !known {
			continue
		}

		out = append(out, web.AccountRow{
			Code:     account.Code,
			Name:     account.Name,
			Parent:   account.ParentCode,
			Type:     string(account.Type),
			Nature:   string(account.Nature),
			Active:   account.IsActive,
			Postable: b.chart.IsPostable(code),
			Depth:    depthOf(b.chart, code),
		})
	}

	return out
}

// depthOf reports how far under a heading an account sits, for indenting it.
//
// Bounded rather than merely walking up: a chart is validated for cycles when it
// is loaded, but an account added through the form has not been, and a loop here
// would hang the page rather than report anything.
func depthOf(chart *gl.ChartOfAccounts, code string) int {
	const deepest = 16

	depth := 0

	for cur := code; depth < deepest; depth++ {
		account, known := chart.ByCode(cur)
		if !known || account.ParentCode == "" {
			return depth
		}

		cur = account.ParentCode
	}

	return deepest
}

// CreateAccount adds an account to the chart.
//
// The chart's own rules do the refusing — a code that is not numeric, a missing
// name, an unknown type — and their message is what the operator sees. The two
// checks made here are the ones the chart cannot make on its own: whether the
// code is already taken, and whether the parent it names exists.
func (b *Books) CreateAccount(_ context.Context, in web.AccountInput) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	code := strings.TrimSpace(in.Code)
	parent := strings.TrimSpace(in.ParentCode)

	if _, exists := b.chart.ByCode(code); exists {
		return fmt.Errorf("account %s already exists", code)
	}
	if parent != "" {
		if _, known := b.chart.ByCode(parent); !known {
			return fmt.Errorf("account %s does not exist, so %s cannot sit under it", parent, code)
		}
	}

	account := gl.Account{
		Code:       code,
		Name:       strings.TrimSpace(in.Name),
		ParentCode: parent,
		Type:       gl.AccountType(strings.TrimSpace(in.Type)),
		Nature:     gl.AccountNature(strings.TrimSpace(in.Nature)),
	}

	if err := b.chart.Add(account); err != nil {
		return err
	}

	b.codes = append(b.codes, account.Code)

	return nil
}
