using Accounting.Application.Common.Models;
using Accounting.Domain.Ledger;

namespace Accounting.Application.Common.Interfaces;

public interface IGlVoucherBridgeService
{
    Task<Result<VoucherId>> PostOperationalVoucherAsync(
        Voucher voucher,
        string postedBy,
        CancellationToken cancellationToken = default);
}
