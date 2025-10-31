using Azure;

namespace DomainService.Repositories;


internal static class AzTableHelpers
{
    internal static bool IsInsertConflict(RequestFailedException ex)
    {
        if (ex.Status == 409 || ex.Status == 412)
        {
            return true;
        }

        return string.Equals(ex.ErrorCode, "EntityAlreadyExists", StringComparison.OrdinalIgnoreCase)
            || string.Equals(ex.ErrorCode, "ResourceAlreadyExists", StringComparison.OrdinalIgnoreCase)
            || string.Equals(ex.ErrorCode, "UpdateConditionNotSatisfied", StringComparison.OrdinalIgnoreCase);
    }
}
