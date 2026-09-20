package system

import (
	"context"
	"time"
)

// SystemConfigHistory records an audit trail entry for any system configuration mutation
type SystemConfigHistory struct {
	ID               string    `json:"id"`
	CompanyProfileID string    `json:"company_profile_id"`
	OptionKey        string    `json:"option_key"`
	OldValue         *string   `json:"old_value,omitempty"`
	NewValue         string    `json:"new_value"`
	ChangedBy        string    `json:"changed_by"`
	ChangedAt        time.Time `json:"changed_at"`
	Reason           *string   `json:"reason,omitempty"`
	ClientIP         *string   `json:"client_ip,omitempty"`
}

// SystemOptionRepository defines the persistence seam for hierarchical system configuration options
type SystemOptionRepository interface {
	UpsertOption(ctx context.Context, opt *SystemOption) error
	GetOption(ctx context.Context, companyID string, branchID *string, key string) (*SystemOption, error)
	GetEffectiveOption(ctx context.Context, companyID string, branchID *string, key string) (*SystemOption, error)
	ListOptionsByCompany(ctx context.Context, companyID string) ([]SystemOption, error)
	ListOptionsByCategory(ctx context.Context, companyID string, category OptionCategory) ([]SystemOption, error)
	RecordHistory(ctx context.Context, history *SystemConfigHistory) error
	ListHistory(ctx context.Context, companyID, key string, limit int32) ([]SystemConfigHistory, error)
}

// VoucherNumberingRepository defines the persistence and atomic sequence reservation seam
type VoucherNumberingRepository interface {
	UpsertConfig(ctx context.Context, cfg *VoucherNumberingConfig) error
	GetConfigByID(ctx context.Context, id string) (*VoucherNumberingConfig, error)
	GetEffectiveConfig(ctx context.Context, companyID string, branchID *string, voucherType string) (*VoucherNumberingConfig, error)
	NextSequence(ctx context.Context, companyID string, branchID *string, voucherType string, voucherDate time.Time) (string, *VoucherNumberingConfig, error)
	ListConfigs(ctx context.Context, companyID string) ([]VoucherNumberingConfig, error)
}
