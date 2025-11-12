# Application Event Flow

This document illustrates how application events travel through the system and how they are handled.

## Command Handling

```mermaid
sequenceDiagram
    participant UI as Frontend
    participant API as Prism API
    participant CQ as Command Queue
    participant DS as Domain Service
    participant IT as Idempotency Table
    participant ES as Event Store

    UI->>API: HTTP Command Request
    API->>CQ: Enqueue Command (with idempotency key)
    API-->>UI: Return Idempotency Key(s)
    CQ->>DS: Deliver Command
    DS->>IT: Reserve/Check Idempotency Key
    alt First attempt
        DS->>ES: Append Domain Events
        DS-->>IT: Mark Completed
    else Duplicate
        IT-->>DS: Key already completed
    end
```

The domain service keeps idempotency keys inside the Azure Table that backs the task and user event stores. Each command run reuses the key provided by the Prism API; if the table already records the key as completed, the handler simply skips emitting duplicate events.

## Query Handling

```mermaid
sequenceDiagram
    participant UI as Frontend
    participant API as Prism API
    participant RS as Redis
    participant RM as Read Model Store

    UI->>API: HTTP Query Request
    API->>RS: Read cache entry (user slice + projections)
    alt Cache hit
        RS-->>API: Return cached payload
    else Cache miss
        RS-->>API: Miss
        API->>RM: Fetch projection snapshot
        RM-->>API: Projection payload
        API->>RS: Cache payload & user slice (TTL scoped)
    end
    API-->>UI: Return projection data
```

Prism API queries Redis before touching the read-model store. Cache hits now satisfy requests entirely from Redis, cutting down on
database round trips. When a miss occurs, the API reads the projection once, stores a small slice of user data alongside the
projection payload, and serves the response. Subsequent requests reuse the cached snapshot until the TTL expires or a projection
update invalidates the entry.

## Projection Updates

```mermaid
flowchart LR
    DS[Domain Service] -->|Publish Domain Event| DEQ[Domain Events Queue]
    DEQ --> RMU[Read-Model Updater]
    RMU --> RM[(Read Model Store)]
    RMU -->|Publish Update| REDIS[(Redis Pub/Sub)]
    REDIS --> SS[Stream Service]
    RM --> API
    SS --> UI
    API --> UI
```

## Command Catalog

For a complete list of domain commands and their payload structures, see [commands.md](./commands.md).

## Event Catalog

For a complete list of domain events and their payload structures, see [events.md](./events.md).
