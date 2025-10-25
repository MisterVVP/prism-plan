using System;
using DomainService.Repositories;
using Xunit;

namespace DomainService.Tests.Repositories;

public class EventTimestampResolverTests
{
    [Fact]
    public void ResolveStoredAt_ReturnsTableTimestamp_WhenEventTimestampIsNonPositive()
    {
        var tableTimestamp = DateTimeOffset.UtcNow;

        var result = EventTimestampResolver.ResolveStoredAt(0, tableTimestamp);

        Assert.Equal(tableTimestamp, result);
    }

    [Fact]
    public void ResolveStoredAt_ReturnsMinValue_WhenEventTimestampIsNonPositiveAndTableTimestampMissing()
    {
        var result = EventTimestampResolver.ResolveStoredAt(-1, null);

        Assert.Equal(DateTimeOffset.MinValue, result);
    }

    [Fact]
    public void ResolveStoredAt_InterpretsMillisecondsSinceUnixEpoch()
    {
        const long eventTimestamp = 1_700_000_000_123;
        var expected = DateTimeOffset.UnixEpoch.AddTicks(eventTimestamp * TimeSpan.TicksPerMillisecond);

        var result = EventTimestampResolver.ResolveStoredAt(eventTimestamp, null);

        Assert.Equal(expected, result);
    }

    [Fact]
    public void ResolveStoredAt_InterpretsMicrosecondsSinceUnixEpoch()
    {
        const long eventTimestamp = 1_700_000_000_123_456;
        var ticksPerMicrosecond = TimeSpan.TicksPerMillisecond / 1_000;
        var expected = DateTimeOffset.UnixEpoch.AddTicks(eventTimestamp * ticksPerMicrosecond);

        var result = EventTimestampResolver.ResolveStoredAt(eventTimestamp, null);

        Assert.Equal(expected, result);
    }

    [Fact]
    public void ResolveStoredAt_InterpretsNanosecondsSinceUnixEpoch()
    {
        const long eventTimestamp = 1_700_000_000_123_456_789;
        var expected = DateTimeOffset.UnixEpoch.AddTicks(eventTimestamp / 100);

        var result = EventTimestampResolver.ResolveStoredAt(eventTimestamp, null);

        Assert.Equal(expected, result);
    }

    [Fact]
    public void ResolveStoredAt_IgnoresTableTimestampWhenEventTimestampIsValid()
    {
        const long eventTimestamp = 42;
        var tableTimestamp = new DateTimeOffset(2024, 01, 01, 0, 0, 0, TimeSpan.Zero);
        var expected = DateTimeOffset.UnixEpoch.AddTicks(eventTimestamp * TimeSpan.TicksPerMillisecond);

        var result = EventTimestampResolver.ResolveStoredAt(eventTimestamp, tableTimestamp);

        Assert.Equal(expected, result);
    }

    [Fact]
    public void ResolveStoredAt_HandlesExtremeNanosecondTimestampsWithoutOverflow()
    {
        const long eventTimestamp = long.MaxValue;
        var expectedTicks = DateTimeOffset.UnixEpoch.Ticks + eventTimestamp / 100;
        var expected = new DateTimeOffset(expectedTicks, TimeSpan.Zero);

        var result = EventTimestampResolver.ResolveStoredAt(eventTimestamp, null);

        Assert.Equal(expected, result);
    }
}
