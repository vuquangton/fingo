package catalog

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// UnitOfMeasure represents a measurement unit (Cái, Kg, Thùng, Hộp, Mét...)
type UnitOfMeasure struct {
	ID               string    `json:"id"`
	CompanyProfileID string    `json:"company_profile_id"`
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func NewUnitOfMeasure(id, companyID, code, name, description string) (*UnitOfMeasure, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) < 2 || len(code) > 50 {
		return nil, fmt.Errorf("%w: '%s'", ErrInvalidCode, code)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("UOM name cannot be empty")
	}

	return &UnitOfMeasure{
		ID:               id,
		CompanyProfileID: companyID,
		Code:             code,
		Name:             name,
		Description:      description,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

type ConversionType string

const (
	ConversionTypeMultiply ConversionType = "MULTIPLY" // Base = Trans * Multiplier
	ConversionTypeDivide   ConversionType = "DIVIDE"   // Base = Trans / Multiplier
)

// UOMConversion represents a conversion multiplier between two UOMs (e.g. 1 Thùng = 24 Lon)
type UOMConversion struct {
	ID               string          `json:"id"`
	CompanyProfileID string          `json:"company_profile_id"`
	ItemID           *string         `json:"item_id,omitempty"` // Specific item or universal
	FromUOMID        string          `json:"from_uom_id"`
	ToUOMID          string          `json:"to_uom_id"`
	Multiplier       decimal.Decimal `json:"multiplier"`
	ConversionType   ConversionType  `json:"conversion_type"`
	IsActive         bool            `json:"is_active"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

func NewUOMConversion(
	id, companyID string,
	itemID *string,
	fromUOMID, toUOMID string,
	multiplier decimal.Decimal,
	cType ConversionType,
) (*UOMConversion, error) {
	if multiplier.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidConversionMultiplier
	}
	if fromUOMID == "" || toUOMID == "" {
		return nil, fmt.Errorf("from_uom_id and to_uom_id are required")
	}
	if fromUOMID == toUOMID {
		return nil, fmt.Errorf("from_uom_id and to_uom_id cannot be identical")
	}
	if cType == "" {
		cType = ConversionTypeMultiply
	}

	return &UOMConversion{
		ID:               id,
		CompanyProfileID: companyID,
		ItemID:           itemID,
		FromUOMID:        fromUOMID,
		ToUOMID:          toUOMID,
		Multiplier:       multiplier,
		ConversionType:   cType,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

// ConvertQuantity converts a quantity using the conversion definition with Banker's Rounding
func (c *UOMConversion) ConvertQuantity(qty decimal.Decimal, decimals int32) (decimal.Decimal, error) {
	if c.Multiplier.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, ErrInvalidConversionMultiplier
	}

	var result decimal.Decimal
	if c.ConversionType == ConversionTypeDivide {
		result = qty.DivRound(c.Multiplier, decimals+4)
	} else {
		result = qty.Mul(c.Multiplier)
	}

	return result.RoundBank(decimals), nil
}

// Invert creates an inverted conversion relationship
func (c *UOMConversion) Invert() *UOMConversion {
	invertedType := ConversionTypeDivide
	if c.ConversionType == ConversionTypeDivide {
		invertedType = ConversionTypeMultiply
	}
	return &UOMConversion{
		ID:               c.ID,
		CompanyProfileID: c.CompanyProfileID,
		ItemID:           c.ItemID,
		FromUOMID:        c.ToUOMID,
		ToUOMID:          c.FromUOMID,
		Multiplier:       c.Multiplier,
		ConversionType:   invertedType,
		IsActive:         c.IsActive,
		CreatedAt:        c.CreatedAt,
		UpdatedAt:        c.UpdatedAt,
	}
}
