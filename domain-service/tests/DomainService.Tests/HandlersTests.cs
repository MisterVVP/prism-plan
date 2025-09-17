using DomainService.Domain.CommandHandlers;
using DomainService.Domain.Commands;
using DomainService.Interfaces;
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
            var queue = new InMemoryQueue();
            ICommandHandler<CreateTaskCommand> handler = new CreateTask(repo, queue);
            var cmd = new CreateTaskCommand(JsonDocument.Parse("{\"title\":\"t\",\"notes\":\"n\",\"category\":\"c\"}").RootElement, "u1", 1, "ik-create");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Single(repo.Events);
            Assert.Single(queue.Events);
            Assert.Equal("task-created", repo.Events[0].Type);
            Assert.False(string.IsNullOrWhiteSpace(repo.Events[0].EntityId));
        }

        [Fact]
        public async Task CreateTask_ignores_duplicate_idempotency_key()
        {
            var repo = new InMemoryTaskRepo();
            var queue = new InMemoryQueue();
            ICommandHandler<CreateTaskCommand> handler = new CreateTask(repo, queue);
            var cmd = new CreateTaskCommand(JsonDocument.Parse("{\"title\":\"t\"}").RootElement, "u1", 1, "ik-dup");
            await handler.Handle(cmd, CancellationToken.None);
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Single(repo.Events);
            Assert.Single(queue.Events);
        }

        [Fact]
        public async Task UpdateTask_adds_event_when_exists()
        {
            var repo = new InMemoryTaskRepo();
            var queue = new InMemoryQueue();
            var seed = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik-seed");
            await repo.Add(seed, CancellationToken.None);
            ICommandHandler<UpdateTaskCommand> handler = new UpdateTask(repo, queue);
            var cmd = new UpdateTaskCommand("t1", JsonDocument.Parse("{\"notes\":\"n\"}").RootElement, "u1", 1, "ik-update");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Equal(2, repo.Events.Count);
            Assert.Equal("task-updated", repo.Events[1].Type);
        }

        [Fact]
        public async Task CompleteTask_adds_event_when_not_done()
        {
            var repo = new InMemoryTaskRepo();
            var queue = new InMemoryQueue();
            var seed = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik-seed");
            await repo.Add(seed, CancellationToken.None);
            ICommandHandler<CompleteTaskCommand> handler = new CompleteTask(repo, queue);
            var cmd = new CompleteTaskCommand("t1", "u1", 1, "ik-complete");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Equal(2, repo.Events.Count);
            Assert.Equal("task-completed", repo.Events[1].Type);
        }

        [Fact]
        public async Task ReopenTask_adds_event_when_done()
        {
            var repo = new InMemoryTaskRepo();
            var queue = new InMemoryQueue();
            var created = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik-seed1");
            await repo.Add(created, CancellationToken.None);
            var completed = new Event("e2", "t1", "task", "task-completed", null, 1, "u1", "ik-seed2");
            await repo.Add(completed, CancellationToken.None);
            ICommandHandler<ReopenTaskCommand> handler = new ReopenTask(repo, queue);
            var cmd = new ReopenTaskCommand("t1", "u1", 2, "ik-reopen");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Equal(3, repo.Events.Count);
            Assert.Equal("task-reopened", repo.Events[2].Type);
        }

        [Fact]
        public async Task LoginUser_logs_in_existing_user()
        {
            var repo = new InMemoryUserRepo();
            var queue = new InMemoryQueue();
            var seed = new Event("e1", "u1", "user", "user-created", null, 0, "u1", "ik-seed");
            await repo.Add(seed, CancellationToken.None);
            ICommandHandler<LoginUserCommand> handler = new LoginUser(repo, queue);
            var cmd = new LoginUserCommand("u1", "n", "e", 1, "ik-login");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Equal(2, repo.Events.Count);
            Assert.Equal("user-logged-in", repo.Events[1].Type);
        }

        [Fact]
        public async Task LogoutUser_enqueues_event()
        {
            var repo = new InMemoryUserRepo();
            var queue = new InMemoryQueue();
            ICommandHandler<LogoutUserCommand> handler = new LogoutUser(repo, queue);
            var cmd = new LogoutUserCommand("u1", 1, "ik-logout");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Single(repo.Events);
            Assert.Equal("user-logged-out", repo.Events[0].Type);
        }

        [Fact]
        public async Task UpdateUserSettings_adds_event()
        {
            var repo = new InMemoryUserRepo();
            var queue = new InMemoryQueue();
            ICommandHandler<UpdateUserSettingsCommand> handler = new UpdateUserSettings(repo, queue);
            var cmd = new UpdateUserSettingsCommand(JsonDocument.Parse("{\"tasksPerCategory\":5}").RootElement, "u1", 1, "ik-settings");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Single(repo.Events);
            Assert.Equal("user-settings-updated", repo.Events[0].Type);
        }

        [Fact]
        public async Task UpdateTask_reopens_when_moved_from_done()
        {
            var repo = new InMemoryTaskRepo();
            var queue = new InMemoryQueue();
            // seed task created and completed
            var created = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik-seed1");
            await repo.Add(created, CancellationToken.None);
            var completed = new Event("e2", "t1", "task", "task-completed", null, 1, "u1", "ik-seed2");
            await repo.Add(completed, CancellationToken.None);
            ICommandHandler<UpdateTaskCommand> handler = new UpdateTask(repo, queue);
            var cmd = new UpdateTaskCommand("t1", JsonDocument.Parse("{\"category\":\"fun\"}").RootElement, "u1", 2, "ik-update");
            await handler.Handle(cmd, CancellationToken.None);
            Assert.Equal(3, repo.Events.Count);
            Assert.Equal("task-updated", repo.Events[2].Type);
            Assert.Single(queue.Events);
            Assert.Equal("task-updated", queue.Events[0].Type);
            Assert.True(repo.Events[2].Data.HasValue);
            JsonElement updateData = repo.Events[2].Data ?? throw new InvalidOperationException();
            Assert.Equal("fun", updateData.GetProperty("category").GetString());
            Assert.False(updateData.GetProperty("done").GetBoolean());
        }

        [Fact]
        public async Task UpdateTask_removes_identifier_from_payload()
        {
            var repo = new InMemoryTaskRepo();
            var queue = new InMemoryQueue();
            var created = new Event("e1", "t1", "task", "task-created", JsonDocument.Parse("{\"title\":\"t\"}").RootElement, 0, "u1", "ik-seed1");
            await repo.Add(created, CancellationToken.None);
            ICommandHandler<UpdateTaskCommand> handler = new UpdateTask(repo, queue);
            var payload = JsonDocument.Parse("{\"id\":\"t1\",\"notes\":\"updated\"}").RootElement;
            var cmd = new UpdateTaskCommand("t1", payload, "u1", 2, "ik-update-id");

            await handler.Handle(cmd, CancellationToken.None);

            Assert.Equal(2, repo.Events.Count);
            var stored = repo.Events[^1];
            Assert.True(stored.Data.HasValue);
            JsonElement storedData = stored.Data ?? throw new InvalidOperationException();
            Assert.False(storedData.TryGetProperty("id", out _));
            Assert.Equal("updated", storedData.GetProperty("notes").GetString());

            Assert.Single(queue.Events);
            var queued = queue.Events[^1];
            Assert.True(queued.Data.HasValue);
            JsonElement queuedData = queued.Data ?? throw new InvalidOperationException();
            Assert.False(queuedData.TryGetProperty("id", out _));
        }
    }

    class InMemoryQueue : IEventQueue
    {
        public List<IEvent> Events { get; } = [];
        public Task Add(IEvent ev, CancellationToken ct)
        {
            Events.Add(ev);
            return Task.CompletedTask;
        }
    }

    class InMemoryTaskRepo : ITaskEventRepository
    {
        public List<IEvent> Events { get; } = [];
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
    }

    class InMemoryUserRepo : IUserEventRepository
    {
        public List<IEvent> Events { get; } = [];
        public Task Add(IEvent ev, CancellationToken ct)
        {
            Events.Add(ev);
            return Task.CompletedTask;
        }
        public Task<bool> Exists(string userId, CancellationToken ct)
        {
            return Task.FromResult(Events.Any(e => e.EntityId == userId));
        }
    }
}