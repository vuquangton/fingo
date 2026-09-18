namespace Accounting.Domain.Treasury;

public enum TreasuryTransactionType
{
    CashReceipt = 1,        // Phiếu thu tiền mặt (PT)
    CashDisbursement = 2,   // Phiếu chi tiền mặt (PC)
    BankDeposit = 3,        // Giấy báo Có ngân hàng (BC)
    BankPaymentOrder = 4    // Ủy nhiệm chi / Giấy báo Nợ ngân hàng (UNC)
}
