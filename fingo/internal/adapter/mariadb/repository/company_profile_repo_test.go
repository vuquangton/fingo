package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"fingo/internal/adapter/mariadb/repository"
	"fingo/internal/domain/system"
)

func setupTestDB(t *testing.T) *sql.DB {
	dsn := "dev:123456@tcp(127.0.0.1:3306)/fingo?parseTime=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("skipping test: cannot connect to local MariaDB: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("skipping test: MariaDB ping failed: %v", err)
	}
	return db
}

func TestCompanyProfileRepo_SaveAndGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewCompanyProfileRepo(db)
	ctx := context.Background()

	profile, err := system.NewProductionCompanyProfile(system.CreateCompanyProfileParams{
		ID:                  "test-cp-01",
		TaxCode:             "0101243150",
		LegalName:           "CÔNG TY CỔ PHẦN FIN GO TEST",
		TradeName:           "FinGo Test",
		Address:             "123 Phố Duy Tân, Cầu Giấy, Hà Nội",
		ProvinceCity:        "Hà Nội",
		LegalRepresentative: "Nguyễn Văn Giám Đốc",
		ChiefAccountant:     "Trần Thị Kế Toán",
		TaxAuthorityCode:    "10500",
		TaxAuthorityName:    "Chi cục Thuế Cầu Giấy",
		StateBudgetChapter:  "754",
		Regime:              system.RegimeCircular133,
		VATMethod:           system.VATMethodDeduction,
		CostingMethod:       system.CostingMovingWeighted,
		BusinessType:        system.BusinessTrading,
	})
	if err != nil {
		t.Fatalf("failed creating domain profile: %v", err)
	}

	profile.RegisteredBanks = []system.BankAccountRegistration{
		{
			AccountNumber: "19030011223344",
			BankName:      "Techcombank",
			BankBranch:    "Thăng Long",
			IsTaxPayment:  true,
		},
	}

	// Act: Save
	if err := repo.SaveProfile(ctx, profile); err != nil {
		t.Fatalf("SaveProfile failed: %v", err)
	}

	// Act: Get
	fetched, err := repo.GetProfile(ctx)
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}

	// Assert
	if fetched.TaxCode != "0101243150" {
		t.Errorf("expected TaxCode 0101243150, got %s", fetched.TaxCode)
	}
	if fetched.LegalName != "CÔNG TY CỔ PHẦN FIN GO TEST" {
		t.Errorf("expected LegalName match, got %s", fetched.LegalName)
	}
	if len(fetched.RegisteredBanks) != 1 || fetched.RegisteredBanks[0].BankName != "Techcombank" {
		t.Errorf("expected 1 registered bank (Techcombank), got %+v", fetched.RegisteredBanks)
	}

	// Act: Update Lock Date
	lockDate := time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC)
	if err := repo.SetLockDate(ctx, fetched.ID, lockDate); err != nil {
		t.Fatalf("SetLockDate failed: %v", err)
	}

	updated, err := repo.GetProfile(ctx)
	if err != nil {
		t.Fatalf("GetProfile after lock date failed: %v", err)
	}
	if updated.LockDate.IsZero() {
		t.Errorf("expected non-zero lock date")
	}
}
