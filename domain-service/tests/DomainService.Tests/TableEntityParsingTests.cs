using System.Reflection;
using Azure;
using Azure.Data.Tables;
using DomainService.Interfaces;
using DomainService.Repositories;
using Xunit;

namespace DomainService.Tests;

public sealed class TableEntityParsingTests
{
    [Theory]
    [InlineData(typeof(TableUserEventRepository), EntityTypes.User, UserEventTypes.Created)]
    [InlineData(typeof(TableTaskEventRepository), EntityTypes.Task, TaskEventTypes.Created)]
    public void TryParseEvent_AllowsBinaryDataStrings(Type repositoryType, string entityType, string eventType)
    {
        var entity = new TableEntity("partition", "row")
        {
            ["Type"] = BinaryData.FromString(eventType),
            ["EventTimestamp"] = BinaryData.FromString("1697040000123"),
            ["UserId"] = BinaryData.FromString("user-123"),
            ["IdempotencyKey"] = BinaryData.FromString("ik-123"),
            ["EntityType"] = BinaryData.FromString(entityType),
            ["Data"] = BinaryData.FromString("{\"foo\":\"bar\"}"),
        };

        var method = repositoryType.GetMethod(
            "TryParseEvent",
            BindingFlags.Static | BindingFlags.NonPublic);
        Assert.NotNull(method);

        var args = new object?[] { entity, null };
        var result = (bool)(method!.Invoke(null, args) ?? false);

        Assert.True(result);
        var ev = Assert.IsType<Event>(args[1]);
        Assert.Equal("row", ev.Id);
        Assert.Equal("partition", ev.EntityId);
        Assert.Equal(entityType, ev.EntityType);
        Assert.Equal(eventType, ev.Type);
        Assert.Equal("ik-123", ev.IdempotencyKey);
        Assert.Equal("user-123", ev.UserId);
        Assert.Equal(1697040000123L, ev.Timestamp);
        Assert.True(ev.Data.HasValue);
        Assert.Equal("bar", ev.Data?.GetProperty("foo").GetString());
    }

    [Fact]
    public void ExtractInt64_HandlesDoubleAndBinaryData()
    {
        var entity = new TableEntity("partition", "row")
        {
            ["EventTimestamp"] = 1234.0,
        };

        var method = typeof(TableUserEventRepository).GetMethod(
            "ExtractInt64",
            BindingFlags.Static | BindingFlags.NonPublic);
        Assert.NotNull(method);

        var value = (long)(method!.Invoke(null, new object?[] { entity, "EventTimestamp" }) ?? 0L);
        Assert.Equal(1234L, value);

        entity["EventTimestamp"] = BinaryData.FromString("4321");
        value = (long)(method.Invoke(null, new object?[] { entity, "EventTimestamp" }) ?? 0L);
        Assert.Equal(4321L, value);
    }
}
