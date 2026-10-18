package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fingo/internal/document"
)

// journalRows is how many lines the manual journal form renders.
//
// Fixed rather than grown by a button, because a button is JavaScript and this
// form has to work without it. Eight is more than any entry the v1 UI is for;
// the limit is stated on the page rather than left to be discovered.
const journalRows = 8

// documentsData is the document list.
type documentsData struct {
	Filter string
	Rows   []DocumentRow
}

// newDocumentData is the create form.
type newDocumentData struct {
	Kind  string
	Today string

	VATRates []VATRateRow
	Accounts []AccountRow
	Cash     []AccountRow

	// Lines is the row numbers the journal form renders.
	Lines []int

	// Values is what was submitted, so a refused form is handed back filled in
	// rather than emptied. An operator who mistyped one figure should not have
	// to type the other six again.
	Values url.Values

	// Against is the document a payment is settling, when the pay form sent the
	// operator here.
	Against string
	// Label names that document for the page.
	Label string
}

// accountsData is the chart of accounts screen.
type accountsData struct {
	Rows []AccountRow
}

// newAccountData is the new-account form.
type newAccountData struct {
	Accounts []AccountRow
	Values   url.Values
}

// fail renders an error the books returned, unchanged.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, content, title string, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, document.ErrUnknownDocument) {
		status = http.StatusNotFound
	}

	s.tmpl.render(w, r, status, content, page{
		Title: title,
		Nav:   title,
		Error: err.Error(),
	})
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	model, err := s.books.Dashboard(r.Context())
	if err != nil {
		s.fail(w, r, "dashboard", "dashboard", err)

		return
	}

	s.tmpl.render(w, r, http.StatusOK, "dashboard", page{Title: "Dashboard", Nav: "dashboard", Data: model})
}

func (s *Server) documents(w http.ResponseWriter, r *http.Request) {
	s.listDocuments(w, r, formText(r.URL.Query().Get("type")), http.StatusOK, "")
}

// listDocuments renders the list, optionally with a note about what just
// happened.
//
// A write answers with the list rather than a redirect. That is what makes it
// replayable: internal/idempotency records a 2xx answer and gives it back on a
// repeat, so a refresh after posting shows the same page instead of posting
// again — and a 303 would not be recorded at all.
func (s *Server) listDocuments(w http.ResponseWriter, r *http.Request, kind string, status int, flash string) {
	rows, err := s.books.Documents(r.Context(), DocumentFilter{Type: kind})
	if err != nil {
		s.fail(w, r, "documents", "documents", err)

		return
	}

	s.tmpl.render(w, r, status, "documents", page{
		Title: "Documents",
		Nav:   "documents",
		Flash: flash,
		Data:  documentsData{Filter: kind, Rows: rows},
	})
}

func (s *Server) documentNew(w http.ResponseWriter, r *http.Request) {
	s.renderNewDocument(w, r, http.StatusOK, formText(r.URL.Query().Get("type")), "", nil, "", "")
}

// kindPayment is the form's name for a receipt.
//
// The form asks what the operator is doing — recording a payment — and the
// document it writes is a receipt: money received. The two names are not the
// same and both are right.
const kindPayment = "payment"

func (s *Server) renderNewDocument(w http.ResponseWriter, r *http.Request, status int, kind, errMsg string, values url.Values, against, label string) {
	switch kind {
	case string(document.TypeInvoice), kindPayment, string(document.TypeJournal):
	default:
		kind = string(document.TypeInvoice)
	}

	settings, err := s.books.Settings(r.Context())
	if err != nil {
		s.fail(w, r, "document_new", "documents", err)

		return
	}

	accounts, err := s.books.Accounts(r.Context())
	if err != nil {
		s.fail(w, r, "document_new", "documents", err)

		return
	}

	lines := make([]int, 0, journalRows)
	for i := 1; i <= journalRows; i++ {
		lines = append(lines, i)
	}

	s.tmpl.render(w, r, status, "document_new", page{
		Title: "New document",
		Nav:   "documents",
		Error: errMsg,
		Data: newDocumentData{
			Kind:     kind,
			Today:    settings.Today,
			VATRates: settings.VATRates,
			Accounts: accounts,
			Cash:     cashAccounts(accounts),
			Lines:    lines,
			Values:   values,
			Against:  against,
			Label:    label,
		},
	})
}

func (s *Server) documentView(w http.ResponseWriter, r *http.Request) {
	s.showDocument(w, r, r.PathValue("id"), http.StatusOK, "")
}

func (s *Server) showDocument(w http.ResponseWriter, r *http.Request, id string, status int, flash string) {
	model, err := s.books.Document(r.Context(), id)
	if err != nil {
		s.fail(w, r, "document_view", "documents", err)

		return
	}

	s.tmpl.render(w, r, status, "document_view", page{
		Title: "Document " + model.ID,
		Nav:   "documents",
		Flash: flash,
		Data:  model,
	})
}

func (s *Server) documentPay(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	model, err := s.books.Document(r.Context(), id)
	if err != nil {
		s.fail(w, r, "document_view", "documents", err)

		return
	}

	// The payment form is the same form, pre-filled with what is outstanding.
	// A second template for it would be a second place to keep the fields in
	// step with the handler that reads them.
	values := url.Values{}
	values.Set("customer", model.Partner)
	values.Set("date", model.Date)
	values.Set("amount", model.Outstanding.Text)

	s.renderNewDocument(w, r, http.StatusOK, kindPayment, "", values, model.ID,
		model.Partner+" — "+model.Outstanding.Text+" outstanding")
}

func (s *Server) createInvoice(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderNewDocument(w, r, http.StatusUnprocessableEntity, string(document.TypeInvoice), err.Error(), nil, "", "")

		return
	}

	in, err := invoiceInput(r)
	if err == nil {
		if _, err = s.books.CreateInvoice(r.Context(), in); err == nil {
			s.listDocuments(w, r, "", http.StatusOK, "The invoice was recorded and posted.")

			return
		}
	}

	s.renderNewDocument(w, r, http.StatusUnprocessableEntity, string(document.TypeInvoice), err.Error(), r.PostForm, "", "")
}

func (s *Server) createPayment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderNewDocument(w, r, http.StatusUnprocessableEntity, kindPayment, err.Error(), nil, "", "")

		return
	}

	against := formText(r.PostFormValue("against"))

	in, err := paymentInput(r)
	if err == nil {
		if _, err = s.books.CreatePayment(r.Context(), in); err == nil {
			if against != "" {
				s.showDocument(w, r, against, http.StatusOK, "The payment was recorded and applied.")

				return
			}

			s.listDocuments(w, r, "", http.StatusOK, "The payment was recorded and posted.")

			return
		}
	}

	s.renderNewDocument(w, r, http.StatusUnprocessableEntity, kindPayment, err.Error(), r.PostForm, against, "")
}

func (s *Server) createJournal(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderNewDocument(w, r, http.StatusUnprocessableEntity, string(document.TypeJournal), err.Error(), nil, "", "")

		return
	}

	in, err := journalInput(r)
	if err == nil {
		if _, err = s.books.CreateJournal(r.Context(), in); err == nil {
			s.listDocuments(w, r, "", http.StatusOK, "The journal entry was recorded and posted.")

			return
		}
	}

	s.renderNewDocument(w, r, http.StatusUnprocessableEntity, string(document.TypeJournal), err.Error(), r.PostForm, "", "")
}

func (s *Server) accounts(w http.ResponseWriter, r *http.Request) {
	rows, err := s.books.Accounts(r.Context())
	if err != nil {
		s.fail(w, r, "accounts", "accounts", err)

		return
	}

	s.tmpl.render(w, r, http.StatusOK, "accounts", page{
		Title: "Chart of accounts",
		Nav:   "accounts",
		Data:  accountsData{Rows: rows},
	})
}

func (s *Server) accountNew(w http.ResponseWriter, r *http.Request) {
	s.renderNewAccount(w, r, http.StatusOK, "", nil)
}

func (s *Server) renderNewAccount(w http.ResponseWriter, r *http.Request, status int, errMsg string, values url.Values) {
	rows, err := s.books.Accounts(r.Context())
	if err != nil {
		s.fail(w, r, "account_new", "accounts", err)

		return
	}

	s.tmpl.render(w, r, status, "account_new", page{
		Title: "New account",
		Nav:   "accounts",
		Error: errMsg,
		Data:  newAccountData{Accounts: rows, Values: values},
	})
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderNewAccount(w, r, http.StatusUnprocessableEntity, err.Error(), nil)

		return
	}

	err := s.books.CreateAccount(r.Context(), AccountInput{
		Code:       formText(r.PostFormValue("code")),
		Name:       formText(r.PostFormValue("name")),
		ParentCode: formText(r.PostFormValue("parent_code")),
		Type:       formText(r.PostFormValue("type")),
		Nature:     formText(r.PostFormValue("nature")),
	})
	if err != nil {
		s.renderNewAccount(w, r, http.StatusUnprocessableEntity, err.Error(), r.PostForm)

		return
	}

	s.accountList(w, r, http.StatusOK, "Account added to the chart.")
}

func (s *Server) accountList(w http.ResponseWriter, r *http.Request, status int, flash string) {
	rows, err := s.books.Accounts(r.Context())
	if err != nil {
		s.fail(w, r, "accounts", "accounts", err)

		return
	}

	s.tmpl.render(w, r, status, "accounts", page{
		Title: "Chart of accounts",
		Nav:   "accounts",
		Flash: flash,
		Data:  accountsData{Rows: rows},
	})
}

func (s *Server) trialBalance(w http.ResponseWriter, r *http.Request) {
	model, err := s.books.TrialBalance(r.Context())
	if err != nil {
		s.fail(w, r, "trial_balance", "reports", err)

		return
	}

	s.tmpl.render(w, r, http.StatusOK, "trial_balance", page{
		Title: "Trial balance",
		Nav:   "reports",
		Data:  model,
	})
}

func (s *Server) profitAndLoss(w http.ResponseWriter, r *http.Request) {
	model, err := s.books.ProfitAndLoss(r.Context())
	if err != nil {
		s.fail(w, r, "profit_and_loss", "reports", err)

		return
	}

	s.tmpl.render(w, r, http.StatusOK, "profit_and_loss", page{
		Title: "Profit and loss",
		Nav:   "reports",
		Data:  model,
	})
}

func (s *Server) balanceSheet(w http.ResponseWriter, r *http.Request) {
	model, err := s.books.BalanceSheet(r.Context())
	if err != nil {
		s.fail(w, r, "balance_sheet", "reports", err)

		return
	}

	s.tmpl.render(w, r, http.StatusOK, "balance_sheet", page{
		Title: "Balance sheet",
		Nav:   "reports",
		Data:  model,
	})
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	s.showSettings(w, r, http.StatusOK, "", "")
}

func (s *Server) showSettings(w http.ResponseWriter, r *http.Request, status int, errMsg, flash string) {
	model, err := s.books.Settings(r.Context())
	if err != nil {
		s.fail(w, r, "settings", "settings", err)

		return
	}

	s.tmpl.render(w, r, status, "settings", page{
		Title: "Settings",
		Nav:   "settings",
		Error: errMsg,
		Flash: flash,
		Data:  model,
	})
}

func (s *Server) createPeriod(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.showSettings(w, r, http.StatusUnprocessableEntity, err.Error(), "")

		return
	}

	from, err := formDate("start date", r.PostFormValue("from"))
	if err != nil {
		s.showSettings(w, r, http.StatusUnprocessableEntity, err.Error(), "")

		return
	}

	to, err := formDate("end date", r.PostFormValue("to"))
	if err != nil {
		s.showSettings(w, r, http.StatusUnprocessableEntity, err.Error(), "")

		return
	}

	err = s.books.CreatePeriod(r.Context(), PeriodInput{
		ID:   formText(r.PostFormValue("id")),
		From: from,
		To:   to,
	})
	if err != nil {
		s.showSettings(w, r, http.StatusUnprocessableEntity, err.Error(), "")

		return
	}

	s.showSettings(w, r, http.StatusOK, "", "The accounting period was added.")
}

func (s *Server) createVATRate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.showSettings(w, r, http.StatusUnprocessableEntity, err.Error(), "")

		return
	}

	from, err := formDate("effective from", r.PostFormValue("from"))
	if err != nil {
		s.showSettings(w, r, http.StatusUnprocessableEntity, err.Error(), "")

		return
	}

	// Basis points, never a float: 10% is 1000. The form asks for the percentage
	// a person knows and the handler does the one conversion.
	basisPoints, err := formAmount("rate", r.PostFormValue("rate"))
	if err != nil {
		s.showSettings(w, r, http.StatusUnprocessableEntity, err.Error(), "")

		return
	}

	var to time.Time
	if raw := formText(r.PostFormValue("to")); raw != "" {
		to, err = formDate("effective to", raw)
		if err != nil {
			s.showSettings(w, r, http.StatusUnprocessableEntity, err.Error(), "")

			return
		}
	}

	in := VATRateInput{
		ID:              formText(r.PostFormValue("id")),
		RateBasisPoints: int(basisPoints) * 100,
		From:            from,
		To:              to,
	}

	if err := s.books.CreateVATRate(r.Context(), in); err != nil {
		s.showSettings(w, r, http.StatusUnprocessableEntity, err.Error(), "")

		return
	}

	s.showSettings(w, r, http.StatusOK, "", "The VAT rate was added.")
}

func invoiceInput(r *http.Request) (InvoiceInput, error) {
	date, err := formDate("date", r.PostFormValue("date"))
	if err != nil {
		return InvoiceInput{}, err
	}

	net, err := formAmount("value before VAT", r.PostFormValue("net_amount"))
	if err != nil {
		return InvoiceInput{}, err
	}

	return InvoiceInput{
		Customer:    formText(r.PostFormValue("customer")),
		Date:        date,
		Description: formText(r.PostFormValue("description")),
		NetAmount:   net,
		VATRateID:   formText(r.PostFormValue("vat_rate_id")),
	}, nil
}

func paymentInput(r *http.Request) (PaymentInput, error) {
	date, err := formDate("date", r.PostFormValue("date"))
	if err != nil {
		return PaymentInput{}, err
	}

	amount, err := formAmount("amount", r.PostFormValue("amount"))
	if err != nil {
		return PaymentInput{}, err
	}

	return PaymentInput{
		Customer:    formText(r.PostFormValue("customer")),
		Date:        date,
		Amount:      amount,
		AccountCode: formText(r.PostFormValue("account_code")),
		Against:     formText(r.PostFormValue("against")),
	}, nil
}

func journalInput(r *http.Request) (JournalInput, error) {
	date, err := formDate("date", r.PostFormValue("date"))
	if err != nil {
		return JournalInput{}, err
	}

	out := JournalInput{
		Date:        date,
		Description: formText(r.PostFormValue("description")),
	}

	for i := 1; i <= journalRows; i++ {
		row := strconv.Itoa(i)

		amount, err := formAmount("line "+row+" amount", r.PostFormValue("amount_"+row))
		if err != nil {
			return JournalInput{}, err
		}

		out.Lines = append(out.Lines, JournalLineInput{
			AccountCode: formText(r.PostFormValue("account_" + row)),
			Side:        formText(r.PostFormValue("side_" + row)),
			Amount:      amount,
		})
	}

	return out, nil
}

// cashAccounts narrows the chart to the accounts money moves through.
func cashAccounts(rows []AccountRow) []AccountRow {
	var out []AccountRow

	for _, row := range rows {
		if !row.Active || !row.Postable {
			continue
		}
		if strings.HasPrefix(row.Code, "111") || strings.HasPrefix(row.Code, "112") {
			out = append(out, row)
		}
	}

	return out
}
