package system_test

import (
	"testing"

	"fingo/internal/domain/system"

	"github.com/stretchr/testify/require"
)

func TestSoD_ValidateRoleAssignment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		existingRoles []system.SystemRoleCode
		newRole       system.SystemRoleCode
		wantErr       bool
		errExpected   error
	}{
		{
			name:          "General Accountant adding Sales Accountant (Allowed)",
			existingRoles: []system.SystemRoleCode{system.RoleGeneralAccountant},
			newRole:       system.RoleSalesAccountant,
			wantErr:       false,
		},
		{
			name:          "General Accountant adding Cashier (Forbidden by Law 88/2015 Article 52.2)",
			existingRoles: []system.SystemRoleCode{system.RoleGeneralAccountant},
			newRole:       system.RoleCashier,
			wantErr:       true,
			errExpected:   system.ErrSoDConflictViolation,
		},
		{
			name:          "Cashier adding Chief Accountant (Forbidden by Law 88/2015 Article 52.2)",
			existingRoles: []system.SystemRoleCode{system.RoleCashier},
			newRole:       system.RoleChiefAccountant,
			wantErr:       true,
			errExpected:   system.ErrSoDConflictViolation,
		},
		{
			name:          "Warehouse Keeper adding Purchase Accountant (Forbidden by Law 88/2015 Article 52.2)",
			existingRoles: []system.SystemRoleCode{system.RoleWarehouseKeeper},
			newRole:       system.RolePurchaseAccountant,
			wantErr:       true,
			errExpected:   system.ErrSoDConflictViolation,
		},
		{
			name:          "System Admin adding Chief Accountant (Forbidden by SoD)",
			existingRoles: []system.SystemRoleCode{system.RoleSystemAdmin},
			newRole:       system.RoleChiefAccountant,
			wantErr:       true,
			errExpected:   system.ErrSoDConflictViolation,
		},
		{
			name:          "Director adding Cashier (Forbidden by Law 88/2015 Article 52.2)",
			existingRoles: []system.SystemRoleCode{system.RoleDirector},
			newRole:       system.RoleCashier,
			wantErr:       true,
			errExpected:   system.ErrSoDConflictViolation,
		},
		{
			name:          "Director adding Chief Accountant (Forbidden by Law 88/2015 Article 52.1)",
			existingRoles: []system.SystemRoleCode{system.RoleDirector},
			newRole:       system.RoleChiefAccountant,
			wantErr:       true,
			errExpected:   system.ErrSoDConflictViolation,
		},
		{
			name:          "Director adding Auditor (Forbidden by VSA 200)",
			existingRoles: []system.SystemRoleCode{system.RoleDirector},
			newRole:       system.RoleAuditor,
			wantErr:       true,
			errExpected:   system.ErrSoDConflictViolation,
		},
		{
			name:          "Auditor adding General Accountant (Forbidden: Auditor must remain read-only)",
			existingRoles: []system.SystemRoleCode{system.RoleAuditor},
			newRole:       system.RoleGeneralAccountant,
			wantErr:       true,
			errExpected:   system.ErrSoDConflictViolation,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := system.ValidateRoleAssignment(tt.existingRoles, tt.newRole)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errExpected != nil {
					require.ErrorIs(t, err, tt.errExpected)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestSoD_CheckMakerChecker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		creatorID   string
		approverID  string
		wantErr     bool
		errExpected error
	}{
		{
			name:       "Creator and Approver are different users (Compliant)",
			creatorID:  "user-creator-01",
			approverID: "user-approver-02",
			wantErr:    false,
		},
		{
			name:        "Creator attempts self-approval (Violation of Circular 99/2025)",
			creatorID:   "user-001",
			approverID:  "user-001",
			wantErr:     true,
			errExpected: system.ErrMakerCheckerViolation,
		},
		{
			name:        "Empty creator ID",
			creatorID:   "",
			approverID:  "user-002",
			wantErr:     true,
			errExpected: system.ErrInvalidUserData,
		},
		{
			name:        "Empty approver ID",
			creatorID:   "user-001",
			approverID:  "",
			wantErr:     true,
			errExpected: system.ErrInvalidUserData,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := system.CheckMakerChecker(tt.creatorID, tt.approverID)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errExpected != nil {
					require.ErrorIs(t, err, tt.errExpected)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}
