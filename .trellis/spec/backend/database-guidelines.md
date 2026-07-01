# Database Guidelines

> Database patterns and conventions for this project.

---

## Overview

The project currently has no database, ORM, migration system, or persistent
storage layer. Forum state is kept in memory by `api/forum.Store`; restarting
the process resets threads and replies to the seeded examples.

Do not introduce a database casually. Persistence is a product and architecture
change that needs an explicit task because it affects runtime configuration,
tests, deployment, and data migration.

## Current Storage Contract

* In-memory storage type: `api/forum.Store`
* Constructor: `forum.NewStore() *forum.Store`
* Thread list: `ListThreads() []Thread`
* Thread lookup: `GetThread(id string) (Thread, error)`
* Thread creation: `CreateThread(title, author, category, body string) (Thread, error)`
* Reply creation: `AddReply(threadID, author, body string) (Thread, error)`
* Concurrency: `Store` uses `sync.RWMutex`; keep reads and writes synchronized.
* Returned `Thread` values clone the replies slice so callers do not mutate store internals.

## Query Patterns

There are no SQL queries. Current lookup and mutation patterns are direct
in-memory operations:

```go
thread, err := store.GetThread(c.Param("id"))
if err != nil {
    return routes.handleForumError(c, err)
}
```

When adding new in-memory operations:

* Trim and validate user-provided fields at the store boundary.
* Return package sentinel errors such as `forum.ErrInvalidInput`.
* Clone slices before returning values that should not be externally mutated.
* Keep sorting deterministic for UI display, as `ListThreads` does by newest first.

## Migrations

No migrations exist. If a future task adds persistence, it must also define:

* Database engine and driver.
* Configuration source for connection details.
* Migration file location and naming.
* Migration execution command.
* Test database strategy.
* Rollback or forward-fix policy.

## Naming Conventions

Current domain type names are singular Go nouns:

* `Thread`
* `Reply`
* `Store`

IDs are strings exposed to routes and templates. Forum-generated IDs are stable
slug-like values with a numeric suffix, for example `welcome` for seed data or
`my-thread-3` for new in-memory threads.

## Common Mistakes

* Do not assume forum data survives process restart.
* Do not return internal slices directly from store methods.
* Do not add file or database persistence without updating this spec, README,
  tests, and runtime configuration docs.
