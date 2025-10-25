using System;
using System.Collections.Generic;
using System.IO;
using System.Threading;
using System.Threading.Tasks;
using Azure;
using Azure.Core;
using Azure.Data.Tables;
using DomainService.Interfaces;
using DomainService.Repositories;
using Moq;
using Xunit;

namespace DomainService.Tests;

public class TableEventRepositoryTests
{
    [Fact]
    public async Task Task_repository_orders_events_by_timestamp_then_id()
    {
        var idempotencyKey = "ik-task";
        var entities = new[]
        {
            CreateEntity("task-1", "evt-02", idempotencyKey, EntityTypes.Task, TaskEventTypes.Created, timestamp: 2),
            CreateEntity("task-1", "evt-01", idempotencyKey, EntityTypes.Task, TaskEventTypes.Updated, timestamp: 1),
        };

        var client = CreateTableClientMock(entities);
        var repository = new TableTaskEventRepository(client.Object);

        var stored = await repository.FindByIdempotencyKey(idempotencyKey, CancellationToken.None);

        Assert.Collection(stored,
            first =>
            {
                Assert.Equal("evt-01", first.Event.Id);
                Assert.Equal(1, first.Event.Timestamp);
            },
            second =>
            {
                Assert.Equal("evt-02", second.Event.Id);
                Assert.Equal(2, second.Event.Timestamp);
            });
    }

    [Fact]
    public async Task User_repository_orders_events_by_timestamp_and_id()
    {
        var idempotencyKey = "ik-user";
        var entities = new[]
        {
            CreateEntity("user-1", "evt-b", idempotencyKey, EntityTypes.User, UserEventTypes.SettingsCreated, timestamp: 1, dispatched: true),
            CreateEntity("user-1", "evt-a", idempotencyKey, EntityTypes.User, UserEventTypes.Created, timestamp: 1),
            CreateEntity("user-1", "evt-c", idempotencyKey, EntityTypes.UserSettings, UserEventTypes.SettingsUpdated, timestamp: 3),
        };

        var client = CreateTableClientMock(entities);
        var repository = new TableUserEventRepository(client.Object);

        var stored = await repository.FindByIdempotencyKey(idempotencyKey, CancellationToken.None);

        Assert.Collection(stored,
            first =>
            {
                Assert.Equal("evt-a", first.Event.Id);
                Assert.Equal(1, first.Event.Timestamp);
                Assert.False(first.Dispatched);
            },
            second =>
            {
                Assert.Equal("evt-b", second.Event.Id);
                Assert.Equal(1, second.Event.Timestamp);
                Assert.True(second.Dispatched);
            },
            third =>
            {
                Assert.Equal("evt-c", third.Event.Id);
                Assert.Equal(3, third.Event.Timestamp);
            });
    }

    private static Mock<TableClient> CreateTableClientMock(IReadOnlyList<TableEntity> entities)
    {
        var pageable = new TestAsyncPageable(entities);
        var client = new Mock<TableClient>();
        client
            .Setup(c => c.QueryAsync<TableEntity>(It.IsAny<string>(), It.IsAny<int?>(), It.IsAny<IEnumerable<string>>(), It.IsAny<CancellationToken>()))
            .Returns(pageable);
        return client;
    }

    private static TableEntity CreateEntity(
        string partitionKey,
        string rowKey,
        string idempotencyKey,
        string entityType,
        string eventType,
        long timestamp,
        bool dispatched = false)
    {
        var entity = new TableEntity(partitionKey, rowKey)
        {
            ["Type"] = eventType,
            ["EventTimestamp"] = timestamp,
            ["UserId"] = "user-1",
            ["IdempotencyKey"] = idempotencyKey,
            ["EntityType"] = entityType,
            ["Dispatched"] = dispatched,
        };

        return entity;
    }

    private sealed class TestAsyncPageable : AsyncPageable<TableEntity>
    {
        private readonly IReadOnlyList<TableEntity> _entities;

        public TestAsyncPageable(IReadOnlyList<TableEntity> entities)
        {
            _entities = entities;
        }

        public override IAsyncEnumerable<Page<TableEntity>> AsPages(string? continuationToken = null, int? pageSizeHint = null)
        {
            return GetPages();

            async IAsyncEnumerable<Page<TableEntity>> GetPages()
            {
                await Task.Yield();
                yield return Page<TableEntity>.FromValues(_entities, null, new TestResponse());
            }
        }
    }

    private sealed class TestResponse : Response
    {
        public override int Status => 200;

        public override string ReasonPhrase => "OK";

        public override Stream? ContentStream { get; set; }
            = Stream.Null;

        public override string ClientRequestId { get; set; } = Guid.NewGuid().ToString();

        public override void Dispose()
        {
        }

        protected override bool TryGetHeader(string name, out string value)
        {
            value = string.Empty;
            return false;
        }

        protected override bool TryGetHeaderValues(string name, out IEnumerable<string> values)
        {
            values = Array.Empty<string>();
            return false;
        }

        protected override IEnumerable<HttpHeader> EnumerateHeaders()
        {
            return Array.Empty<HttpHeader>();
        }

        protected override bool ContainsHeader(string name)
        {
            return false;
        }
    }
}
