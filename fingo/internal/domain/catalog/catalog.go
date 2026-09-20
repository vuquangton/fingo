package catalog

import (
	"errors"
)

var (
	ErrInvalidTaxCode              = errors.New("tax code (MST) is invalid: must satisfy Circular 105 Modulo-11 checksum (10 digits or 13 digits with branch suffix)")
	ErrInvalidConversionMultiplier = errors.New("UOM conversion multiplier must be a strictly positive decimal (> 0)")
	ErrZeroUOMQuantity             = errors.New("quantity must be non-zero for UOM conversion")
	ErrIncompatibleWarehouseAccount= errors.New("warehouse default inventory account must belong to asset group 15 (151, 152, 153, 155, 156, 157, 158)")
	ErrInvalidCurrencyGLAlignment  = errors.New("bank account currency and GL account mismatch: VND requires sub-account of 1121, foreign currency requires sub-account of 1122")
	ErrCreditLimitExceeded         = errors.New("credit limit exceeded: outstanding AR exceeds customer authorized credit ceiling")
	ErrInvalidCitizenID            = errors.New("invalid citizen ID (CCCD): must be exactly 12 numeric digits")
	ErrInvalidSocialInsuranceNo    = errors.New("invalid social insurance number (BHXH): must be exactly 10 numeric digits")
	ErrInvalidCode                 = errors.New("code must be between 2 and 50 alphanumeric characters")
	ErrMissingBaseUOM              = errors.New("item requires a valid base unit of measure (BaseUOM)")
	ErrMissingAccount              = errors.New("mandatory default accounting account is missing")
	ErrIncompatibleItemTypeAccount = errors.New("item type is incompatible with inventory asset account")
)
