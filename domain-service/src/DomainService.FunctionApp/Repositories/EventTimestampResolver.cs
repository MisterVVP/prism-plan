using System;

namespace DomainService.Repositories;

internal static class EventTimestampResolver
{
    private const long NanosecondsPerTick = 100;
    private const long TicksPerMicrosecond = TimeSpan.TicksPerMillisecond / 1000;
    private static readonly long MaxTicksSinceUnixEpoch = DateTimeOffset.MaxValue.Ticks - DateTimeOffset.UnixEpoch.Ticks;

    public static DateTimeOffset ResolveStoredAt(long eventTimestamp, DateTimeOffset? tableTimestamp)
    {
        if (eventTimestamp <= 0)
        {
            return tableTimestamp ?? DateTimeOffset.MinValue;
        }

        if (TryCreateFromUnit(eventTimestamp, TimeSpan.TicksPerMillisecond, out var resolved))
        {
            return PreferTableTimestamp(tableTimestamp, resolved);
        }

        if (TryCreateFromUnit(eventTimestamp, TicksPerMicrosecond, out resolved))
        {
            return PreferTableTimestamp(tableTimestamp, resolved);
        }

        return PreferTableTimestamp(tableTimestamp, CreateFromNanoseconds(eventTimestamp));
    }

    private static bool TryCreateFromUnit(long eventTimestamp, long ticksPerUnit, out DateTimeOffset resolved)
    {
        if (eventTimestamp > 0 && eventTimestamp <= MaxTicksSinceUnixEpoch / ticksPerUnit)
        {
            var ticks = eventTimestamp * ticksPerUnit;
            resolved = DateTimeOffset.UnixEpoch.AddTicks(ticks);
            return true;
        }

        resolved = default;
        return false;
    }

    private static DateTimeOffset CreateFromNanoseconds(long eventTimestamp)
    {
        if (eventTimestamp <= 0)
        {
            return DateTimeOffset.UnixEpoch;
        }

        var ticks = eventTimestamp / NanosecondsPerTick;
        if (ticks > MaxTicksSinceUnixEpoch)
        {
            ticks = MaxTicksSinceUnixEpoch;
        }

        return DateTimeOffset.UnixEpoch.AddTicks(ticks);
    }

    private static DateTimeOffset PreferTableTimestamp(DateTimeOffset? tableTimestamp, DateTimeOffset resolved)
        => tableTimestamp.HasValue && tableTimestamp.Value > resolved ? tableTimestamp.Value : resolved;
}
