using System;
using DomainService.Domain;
using DomainService.Domain.CommandHandlers;
using DomainService.Domain.Commands;
using DomainService.Interfaces;
using Microsoft.Extensions.Logging.Abstractions;
using System.Text.Json;
using Xunit;

namespace DomainService.Tests
{
    public class HandlersTests
    {
        [Fact]
        public async Task CreateTask_adds_event()
        {
            var repo = new InMemoryTaskRepo();
            var dispatcher = new RecordingDispatcher();
            ICommandHandler<CreateTaskCommand> handler = new CreateTask(repo, dispatcher);
            var cmd = new CreateTaskCommand(JsonDocument.Parse("{\"title\":\"t\",\"notes\":\"n\",\"category\":\"c\"}").RootElement, "u1", 1, "ik-create");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Single(repo.Events);
            Assert.Single(dispatcher.Events);
            Assert.Equal("task-created", repo.Events[0].Type);
            Assert.False(string.IsNullOrWhiteSpace(repo.Events[0].EntityId));
        }

        [Fact]
        public async Task CreateTask_ignores_duplicate_idempotency_key()
        {
            var repo = new InMemoryTaskRepo();
            var dispatcher = new RecordingDispatcher();
            ICommandHandler<CreateTaskCommand> handler = new CreateTask(repo, dispatcher);
            var cmd = new CreateTaskCommand(JsonDocument.Parse("{\"title\":\"t\"}").RootElement, "u1", 1, "ik-dup");
            await handler.Handle(cmd, CancellationToken.None);
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Single(repo.Events);
            Assert.Single(dispatcher.Events);
        }
        [Fact]
        public async Task CreateTask_returns_when_command_in_progress()
        {
            var repo = new InMemoryTaskRepo();
            var dispatcher = new RecordingDispatcher();
            await repo.TryStartProcessing("ik-pending", CancellationToken.None);
            ICommandHandler<CreateTaskCommand> handler = new CreateTask(repo, dispatcher);
            var cmd = new CreateTaskCommand(JsonDocument.Parse("{\"title\":\"t\"}").RootElement, "u1", 1, "ik-pending");

            await handler.Handle(cmd, CancellationToken.None);

            Assert.Empty(repo.Events);
            Assert.Empty(dispatcher.Events);
        }


        [Fact]
        public async Task CreateTask_dispatches_event_when_queue_unavailable()
        {
            var repo = new InMemoryTaskRepo();
            var queue = new TransientFailureQueue();
            var fallback = new RecordingFallbackClient();
            var dispatcher = new ResilientEventDispatcher(queue, fallback, NullLogger<ResilientEventDispatcher>.Instance);
            ICommandHandler<CreateTaskCommand> handler = new CreateTask(repo, dispatcher);
            var cmd = new CreateTaskCommand(JsonDocument.Parse("{\"title\":\"t\"}").RootElement, "u1", 1, "ik-flaky");

            await handler.Handle(cmd, CancellationToken.None);

            Assert.Single(repo.Events);
            Assert.Single(fallback.Events);
            Assert.Equal(repo.Events[0].Id, fallback.Events[0].Id);
            Assert.True(repo.IsDispatched(repo.Events[0]));
        }

        [Fact]
        public async Task UpdateTask_adds_event_when_exists()
        {
            var repo = new InMemoryTaskRepo();
            var dispatcher = new RecordingDispatcher();
            var seed = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik-seed");
            await repo.Add(seed, CancellationToken.None);
            ICommandHandler<UpdateTaskCommand> handler = new UpdateTask(repo, dispatcher);
            var cmd = new UpdateTaskCommand("t1", JsonDocument.Parse("{\"notes\":\"n\"}").RootElement, "u1", 1, "ik-update");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Equal(2, repo.Events.Count);
            Assert.Equal("task-updated", repo.Events[1].Type);
        }

        [Fact]
        public async Task UpdateTask_ignores_duplicate_idempotency_key()
        {
            var repo = new InMemoryTaskRepo();
            var dispatcher = new RecordingDispatcher();
            var seed = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik-seed");
            await repo.Add(seed, CancellationToken.None);
            ICommandHandler<UpdateTaskCommand> handler = new UpdateTask(repo, dispatcher);
            var payload = JsonDocument.Parse("{\"notes\":\"n\"}").RootElement;
            var cmd = new UpdateTaskCommand("t1", payload, "u1", 1, "ik-update");

            await handler.Handle(cmd, CancellationToken.None);
            await handler.Handle(cmd, CancellationToken.None);

            Assert.Equal(2, repo.Events.Count);
            Assert.Single(dispatcher.Events);
        }

        [Fact]
        public async Task CompleteTask_adds_event_when_not_done()
        {
            var repo = new InMemoryTaskRepo();
            var dispatcher = new RecordingDispatcher();
            var seed = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik-seed");
            await repo.Add(seed, CancellationToken.None);
            ICommandHandler<CompleteTaskCommand> handler = new CompleteTask(repo, dispatcher);
            var cmd = new CompleteTaskCommand("t1", "u1", 1, "ik-complete");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Equal(2, repo.Events.Count);
            Assert.Equal("task-completed", repo.Events[1].Type);
        }

        [Fact]
        public async Task CompleteTask_ignores_duplicate_idempotency_key()
        {
            var repo = new InMemoryTaskRepo();
            var dispatcher = new RecordingDispatcher();
            var seed = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik-seed");
            await repo.Add(seed, CancellationToken.None);
            ICommandHandler<CompleteTaskCommand> handler = new CompleteTask(repo, dispatcher);
            var cmd = new CompleteTaskCommand("t1", "u1", 1, "ik-complete");

            await handler.Handle(cmd, CancellationToken.None);
            await handler.Handle(cmd, CancellationToken.None);

            Assert.Equal(2, repo.Events.Count);
            Assert.Single(dispatcher.Events);
        }

        [Fact]
        public async Task CompleteTask_replays_existing_event_before_processing()
        {
            var repo = new InMemoryTaskRepo();
            var dispatcher = new RecordingDispatcher();
            var ev = new Event("e-existing", "t1", "task", "task-completed", null, 1, "u1", "ik-existing");
            await repo.Add(ev, CancellationToken.None);
            ICommandHandler<CompleteTaskCommand> handler = new CompleteTask(repo, dispatcher);
            var cmd = new CompleteTaskCommand("t1", "u1", 2, "ik-existing");

            await handler.Handle(cmd, CancellationToken.None);

            Assert.Single(dispatcher.Events);
            Assert.True(repo.IsDispatched(ev));
            Assert.Single(repo.Events);
        }

        [Fact]
        public async Task ReopenTask_adds_event_when_done()
        {
            var repo = new InMemoryTaskRepo();
            var dispatcher = new RecordingDispatcher();
            var created = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik-seed1");
            await repo.Add(created, CancellationToken.None);
            var completed = new Event("e2", "t1", "task", "task-completed", null, 1, "u1", "ik-seed2");
            await repo.Add(completed, CancellationToken.None);
            ICommandHandler<ReopenTaskCommand> handler = new ReopenTask(repo, dispatcher);
            var cmd = new ReopenTaskCommand("t1", "u1", 2, "ik-reopen");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Equal(3, repo.Events.Count);
            Assert.Equal("task-reopened", repo.Events[2].Type);
        }

        [Fact]
        public async Task ReopenTask_ignores_duplicate_idempotency_key()
        {
            var repo = new InMemoryTaskRepo();
            var dispatcher = new RecordingDispatcher();
            var created = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik-seed1");
            await repo.Add(created, CancellationToken.None);
            var completed = new Event("e2", "t1", "task", "task-completed", null, 1, "u1", "ik-seed2");
            await repo.Add(completed, CancellationToken.None);
            ICommandHandler<ReopenTaskCommand> handler = new ReopenTask(repo, dispatcher);
            var cmd = new ReopenTaskCommand("t1", "u1", 2, "ik-reopen");

            await handler.Handle(cmd, CancellationToken.None);
            await handler.Handle(cmd, CancellationToken.None);

            Assert.Equal(3, repo.Events.Count);
            Assert.Single(dispatcher.Events);
        }

        [Fact]
        public async Task LoginUser_logs_in_existing_user()
        {
            var repo = new InMemoryUserRepo();
            var dispatcher = new RecordingDispatcher();
            var seed = new Event("e1", "u1", "user", "user-created", null, 0, "u1", "ik-seed");
            await repo.Add(seed, CancellationToken.None);
            ICommandHandler<LoginUserCommand> handler = new LoginUser(repo, dispatcher);
            var cmd = new LoginUserCommand("u1", "n", "e", 1, "ik-login");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Equal(2, repo.Events.Count);
            Assert.Equal("user-logged-in", repo.Events[1].Type);
        }

        [Fact]
        public async Task LoginUser_replays_events_when_queue_unavailable()
        {
            var repo = new InMemoryUserRepo();
            var queue = new TransientFailureQueue();
            var fallback = new RecordingFallbackClient();
            ICommandHandler<LoginUserCommand> handler = new LoginUser(repo, new ResilientEventDispatcher(queue, fallback, NullLogger<ResilientEventDispatcher>.Instance));
            var cmd = new LoginUserCommand("u2", "n", "e", 1, "ik-login-transient");

            await handler.Handle(cmd, CancellationToken.None);

            Assert.Equal(2, repo.Events.Count);
            Assert.Single(fallback.Events);
            Assert.Single(queue.Events);
            Assert.All(repo.Events, ev => Assert.True(repo.IsDispatched(ev)));

            await handler.Handle(cmd, CancellationToken.None);

            Assert.Equal(2, repo.Events.Count);
            Assert.Single(fallback.Events);
            Assert.Single(queue.Events);
        }

        [Fact]
        public async Task LoginUser_replays_existing_event_before_processing()
        {
            var repo = new InMemoryUserRepo();
            var dispatcher = new RecordingDispatcher();
            var ev = new Event("e-login", "u1", "user", "user-logged-in", null, 1, "u1", "ik-login-existing");
            await repo.Add(ev, CancellationToken.None);
            ICommandHandler<LoginUserCommand> handler = new LoginUser(repo, dispatcher);
            var cmd = new LoginUserCommand("u1", "n", "e", 2, "ik-login-existing");

            await handler.Handle(cmd, CancellationToken.None);

            Assert.Single(dispatcher.Events);
            Assert.True(repo.IsDispatched(ev));
            Assert.Single(repo.Events);
        }

        [Fact]
        public async Task LogoutUser_enqueues_event()
        {
            var repo = new InMemoryUserRepo();
            var dispatcher = new RecordingDispatcher();
            ICommandHandler<LogoutUserCommand> handler = new LogoutUser(repo, dispatcher);
            var cmd = new LogoutUserCommand("u1", 1, "ik-logout");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Single(repo.Events);
            Assert.Equal("user-logged-out", repo.Events[0].Type);
        }

        [Fact]
        public async Task UpdateUserSettings_adds_event()
        {
            var repo = new InMemoryUserRepo();
            var dispatcher = new RecordingDispatcher();
            ICommandHandler<UpdateUserSettingsCommand> handler = new UpdateUserSettings(repo, dispatcher);
            var cmd = new UpdateUserSettingsCommand(JsonDocument.Parse("{\"tasksPerCategory\":5}").RootElement, "u1", 1, "ik-settings");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Single(repo.Events);
            Assert.Equal("user-settings-updated", repo.Events[0].Type);
        }

        [Fact]
        public async Task UpdateTask_reopens_when_moved_from_done()
        {
            var repo = new InMemoryTaskRepo();
            var dispatcher = new RecordingDispatcher();
            // seed task created and completed
            var created = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik-seed1");
            await repo.Add(created, CancellationToken.None);
            var completed = new Event("e2", "t1", "task", "task-completed", null, 1, "u1", "ik-seed2");
            await repo.Add(completed, CancellationToken.None);
            ICommandHandler<UpdateTaskCommand> handler = new UpdateTask(repo, dispatcher);
            var cmd = new UpdateTaskCommand("t1", JsonDocument.Parse("{\"category\":\"fun\"}").RootElement, "u1", 2, "ik-update");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Equal(3, repo.Events.Count);
            Assert.Equal("task-updated", repo.Events[2].Type);
            Assert.Single(dispatcher.Events);
            Assert.Equal("task-updated", dispatcher.Events[0].Type);
            Assert.True(repo.Events[2].Data.HasValue);
            JsonElement updateData = repo.Events[2].Data ?? throw new InvalidOperationException();
            Assert.Equal("fun", updateData.GetProperty("category").GetString());
            Assert.False(updateData.GetProperty("done").GetBoolean());
        }

        [Fact]
        public async Task UpdateTask_removes_identifier_from_payload()
        {
            var repo = new InMemoryTaskRepo();
            var dispatcher = new RecordingDispatcher();
            var created = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik-seed1");
            await repo.Add(created, CancellationToken.None);
            ICommandHandler<UpdateTaskCommand> handler = new UpdateTask(repo, dispatcher);
            var payload = JsonDocument.Parse("{\"id\":\"t1\",\"notes\":\"updated\"}").RootElement;
            var cmd = new UpdateTaskCommand("t1", payload, "u1", 2, "ik-update-id");

            await handler.Handle(cmd, CancellationToken.None);

            Assert.Equal(2, repo.Events.Count);
            var stored = repo.Events[^1];
            Assert.True(stored.Data.HasValue);
            JsonElement storedData = stored.Data ?? throw new InvalidOperationException();
            Assert.False(storedData.TryGetProperty("id", out _));
            Assert.Equal("updated", storedData.GetProperty("notes").GetString());

            Assert.Single(dispatcher.Events);
            var queued = dispatcher.Events[^1];
            Assert.True(queued.Data.HasValue);
            JsonElement queuedData = queued.Data ?? throw new InvalidOperationException();
            Assert.False(queuedData.TryGetProperty("id", out _));
        }
    }

    class TransientFailureQueue : IEventQueue
    {
        private int _attempts;
        public List<IEvent> Events { get; } = [];

        public Task Add(IEvent ev, CancellationToken ct)
        {
            if (_attempts++ == 0)
            {
                throw new InvalidOperationException("Queue unavailable");
            }
            Events.Add(ev);
            return Task.CompletedTask;
        }
    }

    class InMemoryTaskRepo : ITaskEventRepository
    {
        public List<IEvent> Events { get; } = [];
        private readonly HashSet<string> _dispatched = new();
        private readonly Dictionary<string, IdempotencyState> _idempotency = new();

        public Task Add(IEvent ev, CancellationToken ct)
        {
            Events.Add(ev);
            return Task.CompletedTask;
        }

        public Task<IReadOnlyList<IEvent>> Get(string taskId, CancellationToken ct)
        {
            return Task.FromResult<IReadOnlyList<IEvent>>([.. Events.Where(e => e.EntityId == taskId)]);
        }

        public Task<bool> Exists(string idempotencyKey, CancellationToken ct)
        {
            return Task.FromResult(Events.Any(e => e.IdempotencyKey == idempotencyKey));
        }

        public Task<IReadOnlyList<StoredEvent>> FindByIdempotencyKey(string idempotencyKey, CancellationToken ct)
        {
            var matches = Events
                .Where(e => e.IdempotencyKey == idempotencyKey)
                .Select(e => new StoredEvent(e, _dispatched.Contains(e.Id), DateTimeOffset.FromUnixTimeMilliseconds(e.Timestamp)))
                .ToList();
            return Task.FromResult<IReadOnlyList<StoredEvent>>(matches);
        }

        public Task MarkAsDispatched(IEvent ev, CancellationToken ct)
        {
            _dispatched.Add(ev.Id);
            return Task.CompletedTask;
        }

        public Task<IdempotencyResult> TryStartProcessing(string idempotencyKey, CancellationToken ct)
        {
            if (_idempotency.TryGetValue(idempotencyKey, out var state))
            {
                return Task.FromResult(state == IdempotencyState.Completed ? IdempotencyResult.AlreadyProcessed : IdempotencyResult.InProgress);
            }

            _idempotency[idempotencyKey] = IdempotencyState.Processing;
            return Task.FromResult(IdempotencyResult.Started);
        }

        public Task MarkProcessingSucceeded(string idempotencyKey, CancellationToken ct)
        {
            _idempotency[idempotencyKey] = IdempotencyState.Completed;
            return Task.CompletedTask;
        }

        public Task MarkProcessingFailed(string idempotencyKey, CancellationToken ct)
        {
            _idempotency.Remove(idempotencyKey);
            return Task.CompletedTask;
        }

        public bool IsDispatched(IEvent ev) => _dispatched.Contains(ev.Id);

        private enum IdempotencyState
        {
            Processing,
            Completed
        }
    }

    class InMemoryUserRepo : IUserEventRepository
    {
        public List<IEvent> Events { get; } = [];
        private readonly HashSet<string> _dispatched = new();
        private readonly Dictionary<string, IdempotencyState> _idempotency = new();

        public Task Add(IEvent ev, CancellationToken ct)
        {
            Events.Add(ev);
            return Task.CompletedTask;
        }

        public Task<bool> Exists(string userId, CancellationToken ct)
        {
            return Task.FromResult(Events.Any(e => e.EntityId == userId));
        }

        public Task<IReadOnlyList<StoredEvent>> FindByIdempotencyKey(string idempotencyKey, CancellationToken ct)
        {
            var matches = Events
                .Where(e => e.IdempotencyKey == idempotencyKey)
                .Select(e => new StoredEvent(e, _dispatched.Contains(e.Id), DateTimeOffset.FromUnixTimeMilliseconds(e.Timestamp)))
                .ToList();
            return Task.FromResult<IReadOnlyList<StoredEvent>>(matches);
        }

        public Task MarkAsDispatched(IEvent ev, CancellationToken ct)
        {
            _dispatched.Add(ev.Id);
            return Task.CompletedTask;
        }

        public Task<IdempotencyResult> TryStartProcessing(string idempotencyKey, CancellationToken ct)
        {
            if (_idempotency.TryGetValue(idempotencyKey, out var state))
            {
                return Task.FromResult(state == IdempotencyState.Completed ? IdempotencyResult.AlreadyProcessed : IdempotencyResult.InProgress);
            }

            _idempotency[idempotencyKey] = IdempotencyState.Processing;
            return Task.FromResult(IdempotencyResult.Started);
        }

        public Task MarkProcessingSucceeded(string idempotencyKey, CancellationToken ct)
        {
            _idempotency[idempotencyKey] = IdempotencyState.Completed;
            return Task.CompletedTask;
        }

        public Task MarkProcessingFailed(string idempotencyKey, CancellationToken ct)
        {
            _idempotency.Remove(idempotencyKey);
            return Task.CompletedTask;
        }

        public bool IsDispatched(IEvent ev) => _dispatched.Contains(ev.Id);

        private enum IdempotencyState
        {
            Processing,
            Completed
        }
    }

    class RecordingDispatcher : IEventDispatcher
    {
        public List<IEvent> Events { get; } = [];

        public Task Dispatch(IEvent ev, CancellationToken ct)
        {
            Events.Add(ev);
            return Task.CompletedTask;
        }
    }

    class RecordingFallbackClient : IReadModelUpdaterClient
    {
        public List<IEvent> Events { get; } = [];

        public Task SendAsync(IEvent ev, CancellationToken ct)
        {
            Events.Add(ev);
            return Task.CompletedTask;
        }
    }
}
