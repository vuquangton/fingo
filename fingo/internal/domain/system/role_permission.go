package system

import (
	"time"
)

// ActionType defines atomic operations on financial domain resources
type ActionType string

const (
	ActionView    ActionType = "VIEW"    // Xem danh sách và chi tiết
	ActionCreate  ActionType = "CREATE"  // Lập mới chứng từ / danh mục
	ActionUpdate  ActionType = "UPDATE"  // Chỉnh sửa chứng từ ở trạng thái DRAFT
	ActionDelete  ActionType = "DELETE"  // Xóa chứng từ ở trạng thái DRAFT
	ActionPost    ActionType = "POST"    // Ghi sổ Cái (Chuyển vào sổ sách kế toán chính thức)
	ActionUnpost  ActionType = "UNPOST"  // Bỏ ghi sổ (Rút chứng từ về trạng thái nháp để điều chỉnh)
	ActionApprove ActionType = "APPROVE" // Phê duyệt thanh toán / mua hàng / chi lương
	ActionLock    ActionType = "LOCK"    // Khóa kỳ kế toán (Chốt sổ không cho sửa)
	ActionExport  ActionType = "EXPORT"  // Xuất dữ liệu ra Excel, PDF, XML
	ActionSign    ActionType = "SIGN"    // Ký số điện tử (USB Token / HSM / Cloud CA)
)

// ResourceModule defines the major functional accounting modules
type ResourceModule string

const (
	ModuleSystem    ResourceModule = "SYSTEM"
	ModuleCatalog   ResourceModule = "CATALOG"
	ModuleGL        ResourceModule = "GL"
	ModuleCash      ResourceModule = "CASH"
	ModuleSales     ResourceModule = "SALES"
	ModulePurchase  ResourceModule = "PURCHASE"
	ModuleInventory ResourceModule = "INVENTORY"
	ModuleAsset     ResourceModule = "ASSET"
	ModuleTax       ResourceModule = "TAX"
	ModulePayroll   ResourceModule = "PAYROLL"
	ModuleReport    ResourceModule = "REPORT"
	ModuleClosing   ResourceModule = "CLOSING"
)

// SystemRoleCode defines the authoritative Vietnamese accounting roles
type SystemRoleCode string

const (
	RoleSystemAdmin        SystemRoleCode = "ROLE_SYSTEM_ADMIN"        // Quản trị IT
	RoleDirector           SystemRoleCode = "ROLE_DIRECTOR"            // Giám đốc / Người đại diện PL
	RoleChiefAccountant    SystemRoleCode = "ROLE_CHIEF_ACCOUNTANT"    // Kế toán trưởng
	RoleGeneralAccountant  SystemRoleCode = "ROLE_GENERAL_ACCOUNTANT"  // Kế toán tổng hợp
	RoleSalesAccountant    SystemRoleCode = "ROLE_SALES_ACCOUNTANT"    // Kế toán bán hàng & công nợ phải thu
	RolePurchaseAccountant SystemRoleCode = "ROLE_PURCHASE_ACCOUNTANT" // Kế toán mua hàng & công nợ phải trả
	RoleCashier            SystemRoleCode = "ROLE_CASHIER"             // Thủ quỹ
	RoleWarehouseKeeper    SystemRoleCode = "ROLE_WAREHOUSE_KEEPER"    // Thủ kho
	RoleAuditor            SystemRoleCode = "ROLE_AUDITOR"             // Kiểm toán viên (Read-only)
)

// Role defines an organizational permission bundle
type Role struct {
	ID          string         `json:"id"`
	Code        SystemRoleCode `json:"code"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	IsSystem    bool           `json:"is_system"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Permission defines atomic authorization on a resource
type Permission struct {
	ID          string         `json:"id"`
	Code        string         `json:"code"` // e.g. "GL_VOUCHER_POST", "SALES_INVOICE_SIGN"
	Module      ResourceModule `json:"module"`
	Resource    string         `json:"resource"` // e.g. "vouchers", "invoices", "bank_accounts"
	Action      ActionType     `json:"action"`
	Description string         `json:"description"`
}

// UserOrgUnitScope defines Row-Level Security: which branches/departments a user can access
type UserOrgUnitScope struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	OrgUnitID       string    `json:"org_unit_id"`
	IsDefault       bool      `json:"is_default"`
	IncludeChildren bool      `json:"include_children"`
	CreatedAt       time.Time `json:"created_at"`
}
