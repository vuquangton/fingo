package catalog_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fingo/internal/domain/catalog"
)

func TestValidateTaxCode_Modulo11(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		taxCode  string
		expected bool
	}{
		{"Valid 10-digit MST (Viettel)", "0100109106", true},
		{"Valid 10-digit MST (PetroVietnam)", "0100681592", true},
		{"Valid 13-digit branch MST", "0100109106-001", true},
		{"Valid 13-digit branch without hyphen", "0100109106002", true},
		{"Valid 13-digit branch max suffix 999", "0100109106999", true},
		{"Invalid check digit", "0100109107", false},
		{"Invalid length (too short 9 digits)", "010010910", false},
		{"Invalid length (11 digits)", "01001091061", false},
		{"Invalid characters (alphabetic)", "010010910A", false},
		{"Invalid branch suffix 000", "0100109106-000", false},
		{"Invalid branch suffix non-numeric", "0100109106-ABC", false},
		{"Empty string is not valid MST", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, catalog.ValidateTaxCode(tc.taxCode))
		})
	}
}

func TestUnitOfMeasure_And_Conversion(t *testing.T) {
	t.Parallel()

	companyID := "comp-01"

	t.Run("Create valid UOM", func(t *testing.T) {
		uom, err := catalog.NewUnitOfMeasure("uom-1", companyID, "thung", "Thùng", "Thùng 24 lon")
		require.NoError(t, err)
		assert.Equal(t, "THUNG", uom.Code)
		assert.Equal(t, "Thùng", uom.Name)
		assert.True(t, uom.IsActive)
	})

	t.Run("Create UOM invalid code or empty name", func(t *testing.T) {
		_, err := catalog.NewUnitOfMeasure("uom-2", companyID, "", "Cái", "")
		require.ErrorIs(t, err, catalog.ErrInvalidCode)

		_, err = catalog.NewUnitOfMeasure("uom-3", companyID, "CAI", "", "")
		require.Error(t, err)
	})

	t.Run("UOMConversion calculation (Multiply: 1 Thùng = 24 Lon)", func(t *testing.T) {
		multiplier := decimal.RequireFromString("24")
		conv, err := catalog.NewUOMConversion(
			"conv-1", companyID, nil,
			"uom-thung", "uom-lon",
			multiplier, catalog.ConversionTypeMultiply,
		)
		require.NoError(t, err)

		// 10.5 Thùng -> 252 Lon
		qtyThung := decimal.RequireFromString("10.5")
		converted, err := conv.ConvertQuantity(qtyThung, 0)
		require.NoError(t, err)
		assert.Equal(t, "252", converted.String())
	})

	t.Run("UOMConversion calculation (Divide: 240 Lon -> 10 Thùng)", func(t *testing.T) {
		multiplier := decimal.RequireFromString("24")
		conv, err := catalog.NewUOMConversion(
			"conv-2", companyID, nil,
			"uom-lon", "uom-thung",
			multiplier, catalog.ConversionTypeDivide,
		)
		require.NoError(t, err)

		qtyLon := decimal.RequireFromString("240")
		converted, err := conv.ConvertQuantity(qtyLon, 2)
		require.NoError(t, err)
		assert.Equal(t, "10", converted.String())
	})

	t.Run("UOMConversion invalid multipliers and identical units", func(t *testing.T) {
		_, err := catalog.NewUOMConversion(
			"conv-err", companyID, nil,
			"uom-1", "uom-2",
			decimal.Zero, catalog.ConversionTypeMultiply,
		)
		require.ErrorIs(t, err, catalog.ErrInvalidConversionMultiplier)

		_, err = catalog.NewUOMConversion(
			"conv-err", companyID, nil,
			"uom-1", "uom-1",
			decimal.RequireFromString("10"), catalog.ConversionTypeMultiply,
		)
		require.Error(t, err)
	})

	t.Run("UOMConversion Invert method", func(t *testing.T) {
		multiplier := decimal.RequireFromString("24")
		conv, err := catalog.NewUOMConversion(
			"conv-1", companyID, nil,
			"uom-thung", "uom-lon",
			multiplier, catalog.ConversionTypeMultiply,
		)
		require.NoError(t, err)

		inverted := conv.Invert()
		assert.Equal(t, "uom-lon", inverted.FromUOMID)
		assert.Equal(t, "uom-thung", inverted.ToUOMID)
		assert.Equal(t, catalog.ConversionTypeDivide, inverted.ConversionType)

		inv2 := inverted.Invert()
		assert.Equal(t, catalog.ConversionTypeMultiply, inv2.ConversionType)
	})
}

func TestWarehouse_Validation(t *testing.T) {
	t.Parallel()

	companyID := "comp-01"

	t.Run("Valid warehouse with Group 15 inventory account", func(t *testing.T) {
		w, err := catalog.NewWarehouse(
			"wh-1", companyID, nil,
			"KHO_TONG", "Kho tổng", "Hà Nội",
			"acc-1561", "1561",
		)
		require.NoError(t, err)
		assert.Equal(t, "KHO_TONG", w.Code)
		assert.True(t, w.IsActive)
	})

	t.Run("Rejects incompatible warehouse asset account (INV-CAT-07)", func(t *testing.T) {
		_, err := catalog.NewWarehouse(
			"wh-2", companyID, nil,
			"KHO_FAIL", "Kho lỗi", "Hà Nội",
			"acc-1111", "1111", // Not group 15
		)
		require.ErrorIs(t, err, catalog.ErrIncompatibleWarehouseAccount)
	})

	t.Run("Rejects warehouse with missing account or empty code", func(t *testing.T) {
		_, err := catalog.NewWarehouse("wh-3", companyID, nil, "", "Kho", "", "acc-152", "152")
		require.ErrorIs(t, err, catalog.ErrInvalidCode)

		_, err = catalog.NewWarehouse("wh-4", companyID, nil, "KHO", "Kho", "", "", "")
		require.ErrorIs(t, err, catalog.ErrMissingAccount)
	})
}

func TestBankAccount_Validation(t *testing.T) {
	t.Parallel()

	companyID := "comp-01"

	t.Run("Valid VND BankAccount linked to 1121 (INV-CAT-06)", func(t *testing.T) {
		b, err := catalog.NewBankAccount(
			"ba-1", companyID, nil,
			"19036888888888", "Techcombank", "TCB", "Chi nhánh Ba Đình", "VND",
			"acc-11211", "11211",
		)
		require.NoError(t, err)
		assert.Equal(t, "VND", b.CurrencyCode)
		assert.Equal(t, "19036888888888", b.AccountNumber)
	})

	t.Run("Valid USD BankAccount linked to 1122 (INV-CAT-06)", func(t *testing.T) {
		b, err := catalog.NewBankAccount(
			"ba-2", companyID, nil,
			"0011001234567", "Vietcombank", "VCB", "Sở Giao Dịch", "USD",
			"acc-11221", "11221",
		)
		require.NoError(t, err)
		assert.Equal(t, "USD", b.CurrencyCode)
	})

	t.Run("Rejects currency and GL account mismatch (INV-CAT-06)", func(t *testing.T) {
		// VND with 1122 -> Error
		_, err := catalog.NewBankAccount(
			"ba-3", companyID, nil,
			"19036888888888", "Techcombank", "TCB", "", "VND",
			"acc-11221", "11221",
		)
		require.ErrorIs(t, err, catalog.ErrInvalidCurrencyGLAlignment)

		// USD with 1121 -> Error
		_, err = catalog.NewBankAccount(
			"ba-4", companyID, nil,
			"19036888888888", "Techcombank", "TCB", "", "USD",
			"acc-11211", "11211",
		)
		require.ErrorIs(t, err, catalog.ErrInvalidCurrencyGLAlignment)
	})
}

func TestCustomer_And_CreditLimit(t *testing.T) {
	t.Parallel()

	companyID := "comp-01"

	t.Run("Create valid customer with corporate tax code", func(t *testing.T) {
		cust, err := catalog.NewCustomer(
			"cust-1", companyID, "KH001", "CÔNG TY ALPHA", "0100109106",
			"Hà Nội", "0901234567", "alpha@test.vn", "Nguyễn Văn A",
			30, decimal.RequireFromString("100000000"), true,
			"acc-1311", "1311",
		)
		require.NoError(t, err)
		assert.Equal(t, "KH001", cust.Code)
		assert.True(t, cust.EnforceCreditLimit)
	})

	t.Run("Rejects customer with invalid tax code", func(t *testing.T) {
		_, err := catalog.NewCustomer(
			"cust-err", companyID, "KH002", "CÔNG TY BETA", "0100109107", // Invalid check digit
			"", "", "", "", 30, decimal.Zero, false, "acc-1311", "1311",
		)
		require.ErrorIs(t, err, catalog.ErrInvalidTaxCode)
	})

	t.Run("Credit limit barrier enforcement (INV-CAT-05)", func(t *testing.T) {
		cust, err := catalog.NewCustomer(
			"cust-3", companyID, "KH003", "CÔNG TY GAMMA", "",
			"", "", "", "",
			30, decimal.RequireFromString("100000000"), true, // Limit: 100M
			"acc-1311", "1311",
		)
		require.NoError(t, err)

		// Outstanding 40M + New 50M = 90M <= 100M -> PASS
		err = cust.CheckCreditLimit(decimal.RequireFromString("40000000"), decimal.RequireFromString("50000000"))
		require.NoError(t, err)

		// Outstanding 60M + New 50M = 110M > 100M -> REJECT
		err = cust.CheckCreditLimit(decimal.RequireFromString("60000000"), decimal.RequireFromString("50000000"))
		require.ErrorIs(t, err, catalog.ErrCreditLimitExceeded)

		// If enforcement disabled -> PASS
		cust.EnforceCreditLimit = false
		err = cust.CheckCreditLimit(decimal.RequireFromString("60000000"), decimal.RequireFromString("50000000"))
		require.NoError(t, err)
	})
}

func TestVendor_And_NonCashEligibility(t *testing.T) {
	t.Parallel()

	companyID := "comp-01"

	t.Run("Create valid vendor and verify non-cash bank info (Decree 181/2025)", func(t *testing.T) {
		v, err := catalog.NewVendor(
			"v-1", companyID, "NCC01", "TẬP ĐOÀN DẦU KHÍ", "0100681592",
			"Hà Nội", "0912345678", "pvn@test.vn", "Trần Văn B",
			"0011009999999", "Vietcombank", "Hà Nội",
			30, "acc-3311", "3311",
		)
		require.NoError(t, err)
		assert.True(t, v.HasValidBankAccountForNonCash())
	})

	t.Run("Vendor without bank details fails non-cash eligibility", func(t *testing.T) {
		v, err := catalog.NewVendor(
			"v-2", companyID, "NCC02", "HỘ KINH DOANH NAM AN", "",
			"", "", "", "",
			"", "", "", // No bank details
			15, "acc-3311", "3311",
		)
		require.NoError(t, err)
		assert.False(t, v.HasValidBankAccountForNonCash())
	})

	t.Run("Rejects vendor with invalid tax code", func(t *testing.T) {
		_, err := catalog.NewVendor(
			"v-err", companyID, "NCC03", "NHÀ CUNG CẤP LỖI", "123456",
			"", "", "", "", "", "", "", 30, "acc-3311", "3311",
		)
		require.ErrorIs(t, err, catalog.ErrInvalidTaxCode)
	})
}

func TestItem_Validation(t *testing.T) {
	t.Parallel()

	companyID := "comp-01"
	invAcc := "acc-1561"

	t.Run("Create valid merchandise item", func(t *testing.T) {
		item, err := catalog.NewItem(
			"item-1", companyID, "VT-THEP-D6", "Thép cuộn D6", "8934567890123",
			catalog.ItemTypeMerchandise, "uom-kg", nil, &invAcc,
			"acc-632", "acc-5111",
			decimal.RequireFromString("10"),
			decimal.RequireFromString("15000"),
			decimal.RequireFromString("16500"),
		)
		require.NoError(t, err)
		assert.Equal(t, "VT-THEP-D6", item.Code)
		assert.Equal(t, catalog.ItemTypeMerchandise, item.ItemType)
	})

	t.Run("Rejects non-service item without inventory account", func(t *testing.T) {
		_, err := catalog.NewItem(
			"item-err", companyID, "VT-FAIL", "Vật tư thiếu kho", "",
			catalog.ItemTypeMaterial, "uom-cai", nil, nil, // Missing inv acc
			"acc-632", "acc-5111",
			decimal.RequireFromString("10"), decimal.Zero, decimal.Zero,
		)
		require.ErrorIs(t, err, catalog.ErrMissingAccount)
	})

	t.Run("ValidateItemAccounts compatibility checks", func(t *testing.T) {
		// Material with 152 -> OK
		err := catalog.ValidateItemAccounts(catalog.ItemTypeMaterial, "1521", "632", "5111")
		require.NoError(t, err)

		// Material with 156 -> Error
		err = catalog.ValidateItemAccounts(catalog.ItemTypeMaterial, "1561", "632", "5111")
		require.ErrorIs(t, err, catalog.ErrIncompatibleItemTypeAccount)

		// Tool with 153 -> OK
		err = catalog.ValidateItemAccounts(catalog.ItemTypeTool, "1531", "632", "5111")
		require.NoError(t, err)

		// Finished Good with 155 -> OK
		err = catalog.ValidateItemAccounts(catalog.ItemTypeFinishedGood, "1551", "632", "5111")
		require.NoError(t, err)

		// Service without inv account -> OK
		err = catalog.ValidateItemAccounts(catalog.ItemTypeService, "", "632", "5113")
		require.NoError(t, err)

		// Invalid COGS account
		err = catalog.ValidateItemAccounts(catalog.ItemTypeMaterial, "1521", "642", "5111")
		require.ErrorIs(t, err, catalog.ErrIncompatibleItemTypeAccount)

		// Invalid Revenue account
		err = catalog.ValidateItemAccounts(catalog.ItemTypeMaterial, "1521", "632", "642")
		require.ErrorIs(t, err, catalog.ErrIncompatibleItemTypeAccount)
	})
}

func TestEmployee_Validation_And_PDPL_Masking(t *testing.T) {
	t.Parallel()

	companyID := "comp-01"

	t.Run("Create valid employee and verify PDPL masking (INV-CAT-08)", func(t *testing.T) {
		emp, err := catalog.NewEmployee(
			"emp-1", companyID, nil,
			"NV001", "Nguyễn Văn Tuấn", "Phòng Kế toán", "Kế toán viên",
			"001090012345", "8012345678", "0123456789",
			decimal.RequireFromString("15000000"), decimal.RequireFromString("1.0"),
			"19036888888888", "Techcombank",
			"acc-1411", "acc-3341",
		)
		require.NoError(t, err)
		assert.Equal(t, "NV001", emp.Code)
		assert.Equal(t, "001******345", emp.MaskCitizenID())
	})

	t.Run("Rejects invalid CCCD length", func(t *testing.T) {
		_, err := catalog.NewEmployee(
			"emp-err1", companyID, nil,
			"NV002", "Trần Văn Lỗi", "Phòng KD", "Nhân viên",
			"00109001234", // 11 digits
			"", "0123456789",
			decimal.Zero, decimal.NewFromInt(1), "", "",
			"acc-141", "acc-334",
		)
		require.ErrorIs(t, err, catalog.ErrInvalidCitizenID)

		// Non-digit CCCD
		assert.False(t, catalog.ValidateCitizenID("00109001234A"))
	})

	t.Run("Rejects invalid BHXH length", func(t *testing.T) {
		_, err := catalog.NewEmployee(
			"emp-err2", companyID, nil,
			"NV003", "Trần Văn Lỗi 2", "Phòng KD", "Nhân viên",
			"001090012345", "", "012345678", // 9 digits
			decimal.Zero, decimal.NewFromInt(1), "", "",
			"acc-141", "acc-334",
		)
		require.ErrorIs(t, err, catalog.ErrInvalidSocialInsuranceNo)

		// Non-digit BHXH
		assert.False(t, catalog.ValidateSocialInsuranceNo("012345678A"))
	})
}

func TestCatalog_Additional_Branch_Coverage(t *testing.T) {
	t.Parallel()

	companyID := "comp-01"

	t.Run("BankAccount validation edge cases", func(t *testing.T) {
		// Short account number
		_, err := catalog.NewBankAccount("b-x", companyID, nil, "12", "Bank", "B", "", "VND", "a-1", "1121")
		require.Error(t, err)

		// Empty bank name
		_, err = catalog.NewBankAccount("b-x", companyID, nil, "12345", "", "B", "", "VND", "a-1", "1121")
		require.Error(t, err)

		// Currency code != 3
		_, err = catalog.NewBankAccount("b-x", companyID, nil, "12345", "Bank", "B", "", "VN", "a-1", "1121")
		require.Error(t, err)

		// Missing GL account
		_, err = catalog.NewBankAccount("b-x", companyID, nil, "12345", "Bank", "B", "", "VND", "", "")
		require.ErrorIs(t, err, catalog.ErrMissingAccount)
	})

	t.Run("Customer validation edge cases", func(t *testing.T) {
		// Invalid code
		_, err := catalog.NewCustomer("c-x", companyID, "", "Name", "", "", "", "", "", 0, decimal.Zero, false, "a-1", "131")
		require.ErrorIs(t, err, catalog.ErrInvalidCode)

		// Empty name
		_, err = catalog.NewCustomer("c-x", companyID, "C01", "", "", "", "", "", "", 0, decimal.Zero, false, "a-1", "131")
		require.Error(t, err)

		// Missing AR account
		_, err = catalog.NewCustomer("c-x", companyID, "C01", "Name", "", "", "", "", "", 0, decimal.Zero, false, "", "")
		require.ErrorIs(t, err, catalog.ErrMissingAccount)

		// Negative credit limit and payment days defaults to 0
		cust, err := catalog.NewCustomer("c-x", companyID, "C01", "Name", "", "", "", "", "", -5, decimal.RequireFromString("-100"), false, "a-1", "131")
		require.NoError(t, err)
		assert.Equal(t, 0, cust.PaymentTermDays)
		assert.True(t, cust.CreditLimit.IsZero())
	})

	t.Run("Vendor validation edge cases", func(t *testing.T) {
		// Invalid code
		_, err := catalog.NewVendor("v-x", companyID, "", "Name", "", "", "", "", "", "", "", "", 0, "a-1", "331")
		require.ErrorIs(t, err, catalog.ErrInvalidCode)

		// Empty name
		_, err = catalog.NewVendor("v-x", companyID, "V01", "", "", "", "", "", "", "", "", "", 0, "a-1", "331")
		require.Error(t, err)

		// Missing AP account
		_, err = catalog.NewVendor("v-x", companyID, "V01", "Name", "", "", "", "", "", "", "", "", 0, "", "")
		require.ErrorIs(t, err, catalog.ErrMissingAccount)

		// Negative payment term days defaults to 0
		v, err := catalog.NewVendor("v-x", companyID, "V01", "Name", "", "", "", "", "", "", "", "", -5, "a-1", "331")
		require.NoError(t, err)
		assert.Equal(t, 0, v.PaymentTermDays)
	})

	t.Run("Item validation edge cases", func(t *testing.T) {
		inv := "a-156"
		// Invalid code
		_, err := catalog.NewItem("i-x", companyID, "", "Name", "", catalog.ItemTypeMerchandise, "uom-1", nil, &inv, "a-632", "a-511", decimal.Zero, decimal.Zero, decimal.Zero)
		require.ErrorIs(t, err, catalog.ErrInvalidCode)

		// Empty name
		_, err = catalog.NewItem("i-x", companyID, "I01", "", "", catalog.ItemTypeMerchandise, "uom-1", nil, &inv, "a-632", "a-511", decimal.Zero, decimal.Zero, decimal.Zero)
		require.Error(t, err)

		// Missing base UOM
		_, err = catalog.NewItem("i-x", companyID, "I01", "Name", "", catalog.ItemTypeMerchandise, "", nil, &inv, "a-632", "a-511", decimal.Zero, decimal.Zero, decimal.Zero)
		require.ErrorIs(t, err, catalog.ErrMissingBaseUOM)

		// Missing COGS account
		_, err = catalog.NewItem("i-x", companyID, "I01", "Name", "", catalog.ItemTypeMerchandise, "uom-1", nil, &inv, "", "a-511", decimal.Zero, decimal.Zero, decimal.Zero)
		require.ErrorIs(t, err, catalog.ErrMissingAccount)

		// Missing Revenue account
		_, err = catalog.NewItem("i-x", companyID, "I01", "Name", "", catalog.ItemTypeMerchandise, "uom-1", nil, &inv, "a-632", "", decimal.Zero, decimal.Zero, decimal.Zero)
		require.ErrorIs(t, err, catalog.ErrMissingAccount)

		// Negative prices default to 0
		item, err := catalog.NewItem("i-x", companyID, "I01", "Name", "", catalog.ItemTypeMerchandise, "uom-1", nil, &inv, "a-632", "a-511", decimal.Zero, decimal.RequireFromString("-10"), decimal.RequireFromString("-20"))
		require.NoError(t, err)
		assert.True(t, item.StandardCostPrice.IsZero())
		assert.True(t, item.StandardSalePrice.IsZero())

		// Incompatible COGS or Rev code
		err = catalog.ValidateItemAccounts(catalog.ItemTypeService, "", "1111", "5111")
		require.Error(t, err)
		err = catalog.ValidateItemAccounts(catalog.ItemTypeService, "", "632", "1111")
		require.Error(t, err)
	})

	t.Run("Employee validation edge cases", func(t *testing.T) {
		// Invalid code
		_, err := catalog.NewEmployee("e-x", companyID, nil, "", "Name", "D", "P", "001090012345", "", "0123456789", decimal.Zero, decimal.NewFromInt(1), "", "", "a-141", "a-334")
		require.ErrorIs(t, err, catalog.ErrInvalidCode)

		// Empty name
		_, err = catalog.NewEmployee("e-x", companyID, nil, "E01", "", "D", "P", "001090012345", "", "0123456789", decimal.Zero, decimal.NewFromInt(1), "", "", "a-141", "a-334")
		require.Error(t, err)

		// Invalid tax code length
		_, err = catalog.NewEmployee("e-x", companyID, nil, "E01", "Name", "D", "P", "001090012345", "123", "0123456789", decimal.Zero, decimal.NewFromInt(1), "", "", "a-141", "a-334")
		require.Error(t, err)

		// Missing advance or payroll account
		_, err = catalog.NewEmployee("e-x", companyID, nil, "E01", "Name", "D", "P", "001090012345", "", "0123456789", decimal.Zero, decimal.NewFromInt(1), "", "", "", "a-334")
		require.ErrorIs(t, err, catalog.ErrMissingAccount)
		_, err = catalog.NewEmployee("e-x", companyID, nil, "E01", "Name", "D", "P", "001090012345", "", "0123456789", decimal.Zero, decimal.NewFromInt(1), "", "", "a-141", "")
		require.ErrorIs(t, err, catalog.ErrMissingAccount)

		// Negative base salary and coefficient defaults
		emp, err := catalog.NewEmployee("e-x", companyID, nil, "E01", "Name", "D", "P", "001090012345", "", "0123456789", decimal.RequireFromString("-1000"), decimal.Zero, "", "", "a-141", "a-334")
		require.NoError(t, err)
		assert.True(t, emp.BaseSalary.IsZero())
		assert.Equal(t, "1", emp.SalaryCoefficient.String())

		// MaskCitizenID invalid length
		emp.CitizenID = "123"
		assert.Equal(t, "************", emp.MaskCitizenID())
	})

	t.Run("UOM edge cases", func(t *testing.T) {
		// Empty from or to UOM ID
		_, err := catalog.NewUOMConversion("c-x", companyID, nil, "", "to", decimal.NewFromInt(1), catalog.ConversionTypeMultiply)
		require.Error(t, err)

		// Default conversion type
		conv, err := catalog.NewUOMConversion("c-x", companyID, nil, "from", "to", decimal.NewFromInt(1), "")
		require.NoError(t, err)
		assert.Equal(t, catalog.ConversionTypeMultiply, conv.ConversionType)
	})

	t.Run("Warehouse empty account code allowed if ID is valid", func(t *testing.T) {
		w, err := catalog.NewWarehouse("wh-x", companyID, nil, "WH", "Warehouse", "", "acc-id", "")
		require.NoError(t, err)
		assert.Equal(t, "WH", w.Code)

		_, err = catalog.NewWarehouse("wh-x", companyID, nil, "WH", "", "", "acc-id", "")
		require.Error(t, err)
	})

	t.Run("Customer without taxCode is allowed", func(t *testing.T) {
		c, err := catalog.NewCustomer("c-notax", companyID, "CNO", "Individual", "", "", "", "", "", 0, decimal.Zero, false, "a-1", "131")
		require.NoError(t, err)
		assert.Empty(t, c.TaxCode)
	})
}


