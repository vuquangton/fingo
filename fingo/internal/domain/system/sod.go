package system

import (
	"errors"
	"fmt"
)

var (
	ErrSoDConflictViolation = errors.New("segregation of duties violation: incompatible roles cannot be assigned together")
	ErrMakerCheckerViolation = errors.New("maker-checker violation: voucher creator cannot approve or post their own voucher")
)

// SoDConflictRule defines statutory and operational incompatibility between roles
type SoDConflictRule struct {
	RoleA       SystemRoleCode
	RoleB       SystemRoleCode
	LegalBasis  string
	Description string
}

// defaultSoDRules encapsulates statutory prohibitions under Law 88/2015 and Circular 99/2025
var defaultSoDRules = []SoDConflictRule{
	// Article 52.2 Law 88/2015: Cashier cannot hold any Accounting role
	{RoleA: RoleCashier, RoleB: RoleChiefAccountant, LegalBasis: "Điều 52.2 Luật Kế toán 88/2015/QH13", Description: "Thủ quỹ không được kiêm Kế toán trưởng"},
	{RoleA: RoleCashier, RoleB: RoleGeneralAccountant, LegalBasis: "Điều 52.2 Luật Kế toán 88/2015/QH13", Description: "Thủ quỹ không được kiêm Kế toán tổng hợp"},
	{RoleA: RoleCashier, RoleB: RoleSalesAccountant, LegalBasis: "Điều 52.2 Luật Kế toán 88/2015/QH13", Description: "Thủ quỹ không được kiêm Kế toán bán hàng"},
	{RoleA: RoleCashier, RoleB: RolePurchaseAccountant, LegalBasis: "Điều 52.2 Luật Kế toán 88/2015/QH13", Description: "Thủ quỹ không được kiêm Kế toán mua hàng"},
	{RoleA: RoleCashier, RoleB: RoleWarehouseKeeper, LegalBasis: "Thông tư 99/2025/TT-BTC & COSO", Description: "Thủ quỹ không được kiêm Thủ kho"},
	{RoleA: RoleCashier, RoleB: RoleDirector, LegalBasis: "Điều 52.2 Luật Kế toán 88/2015/QH13", Description: "Người quản lý, điều hành không được kiêm Thủ quỹ"},

	// Article 52.2 Law 88/2015: Storekeeper cannot hold any Accounting role
	{RoleA: RoleWarehouseKeeper, RoleB: RoleChiefAccountant, LegalBasis: "Điều 52.2 Luật Kế toán 88/2015/QH13", Description: "Thủ kho không được kiêm Kế toán trưởng"},
	{RoleA: RoleWarehouseKeeper, RoleB: RoleGeneralAccountant, LegalBasis: "Điều 52.2 Luật Kế toán 88/2015/QH13", Description: "Thủ kho không được kiêm Kế toán tổng hợp"},
	{RoleA: RoleWarehouseKeeper, RoleB: RoleSalesAccountant, LegalBasis: "Điều 52.2 Luật Kế toán 88/2015/QH13", Description: "Thủ kho không được kiêm Kế toán bán hàng"},
	{RoleA: RoleWarehouseKeeper, RoleB: RolePurchaseAccountant, LegalBasis: "Điều 52.2 Luật Kế toán 88/2015/QH13", Description: "Thủ kho không được kiêm Kế toán mua hàng"},
	{RoleA: RoleWarehouseKeeper, RoleB: RoleDirector, LegalBasis: "Điều 52.2 Luật Kế toán 88/2015/QH13", Description: "Người quản lý, điều hành không được kiêm Thủ kho"},

	// Article 52.1 Law 88/2015: Director cannot be Chief Accountant
	{RoleA: RoleDirector, RoleB: RoleChiefAccountant, LegalBasis: "Điều 52.1 Luật Kế toán 88/2015/QH13", Description: "Người đứng đầu / Giám đốc không được kiêm Kế toán trưởng"},
	{RoleA: RoleDirector, RoleB: RoleAuditor, LegalBasis: "Chuẩn mực Kiểm toán VSA 200", Description: "Người quản lý / Giám đốc không được kiêm Kiểm toán viên"},

	// System Admin cannot hold financial execution roles
	{RoleA: RoleSystemAdmin, RoleB: RoleDirector, LegalBasis: "COSO & VSA 240", Description: "Quản trị hệ thống IT không được kiêm Giám đốc"},
	{RoleA: RoleSystemAdmin, RoleB: RoleChiefAccountant, LegalBasis: "COSO & VSA 240", Description: "Quản trị hệ thống IT không được kiêm Kế toán trưởng"},
	{RoleA: RoleSystemAdmin, RoleB: RoleGeneralAccountant, LegalBasis: "COSO & VSA 240", Description: "Quản trị hệ thống IT không được kiêm Kế toán tổng hợp"},
	{RoleA: RoleSystemAdmin, RoleB: RoleSalesAccountant, LegalBasis: "COSO & VSA 240", Description: "Quản trị hệ thống IT không được kiêm Kế toán bán hàng"},
	{RoleA: RoleSystemAdmin, RoleB: RolePurchaseAccountant, LegalBasis: "COSO & VSA 240", Description: "Quản trị hệ thống IT không được kiêm Kế toán mua hàng"},
	{RoleA: RoleSystemAdmin, RoleB: RoleCashier, LegalBasis: "COSO & VSA 240", Description: "Quản trị hệ thống IT không được kiêm Thủ quỹ"},
	{RoleA: RoleSystemAdmin, RoleB: RoleWarehouseKeeper, LegalBasis: "COSO & VSA 240", Description: "Quản trị hệ thống IT không được kiêm Thủ kho"},
	{RoleA: RoleSystemAdmin, RoleB: RoleAuditor, LegalBasis: "COSO & VSA 240", Description: "Quản trị hệ thống IT không được kiêm Kiểm toán viên"},

	// Auditor must remain strictly read-only and independent
	{RoleA: RoleAuditor, RoleB: RoleChiefAccountant, LegalBasis: "Chuẩn mực Kiểm toán VSA 200", Description: "Kiểm toán viên phải độc lập, không kiêm Kế toán trưởng"},
	{RoleA: RoleAuditor, RoleB: RoleGeneralAccountant, LegalBasis: "Chuẩn mực Kiểm toán VSA 200", Description: "Kiểm toán viên phải độc lập, không kiêm Kế toán tổng hợp"},
	{RoleA: RoleAuditor, RoleB: RoleSalesAccountant, LegalBasis: "Chuẩn mực Kiểm toán VSA 200", Description: "Kiểm toán viên phải độc lập, không kiêm Kế toán bán hàng"},
	{RoleA: RoleAuditor, RoleB: RolePurchaseAccountant, LegalBasis: "Chuẩn mực Kiểm toán VSA 200", Description: "Kiểm toán viên phải độc lập, không kiêm Kế toán mua hàng"},
	{RoleA: RoleAuditor, RoleB: RoleCashier, LegalBasis: "Chuẩn mực Kiểm toán VSA 200", Description: "Kiểm toán viên phải độc lập, không kiêm Thủ quỹ"},
	{RoleA: RoleAuditor, RoleB: RoleWarehouseKeeper, LegalBasis: "Chuẩn mực Kiểm toán VSA 200", Description: "Kiểm toán viên phải độc lập, không kiêm Thủ kho"},
}

// ValidateRoleAssignment checks if assigning newRole to a user with existingRoles violates statutory SoD rules
func ValidateRoleAssignment(existingRoles []SystemRoleCode, newRole SystemRoleCode) error {
	for _, existing := range existingRoles {
		if existing == newRole {
			continue
		}
		for _, rule := range defaultSoDRules {
			if (rule.RoleA == existing && rule.RoleB == newRole) ||
				(rule.RoleA == newRole && rule.RoleB == existing) {
				return fmt.Errorf("%w: role %s conflicts with role %s (%s: %s)",
					ErrSoDConflictViolation, existing, newRole, rule.LegalBasis, rule.Description)
			}
		}
	}
	return nil
}

// CheckMakerChecker enforces the Four-Eyes Principle (Thông tư 99/2025/TT-BTC & VSA 240)
func CheckMakerChecker(creatorID, approverID string) error {
	if creatorID == "" || approverID == "" {
		return fmt.Errorf("%w: creator and approver IDs must not be empty", ErrInvalidUserData)
	}
	if creatorID == approverID {
		return fmt.Errorf("%w: creator [%s] cannot approve own voucher", ErrMakerCheckerViolation, creatorID)
	}
	return nil
}
