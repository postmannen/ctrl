---
trigger: always_on
---

# Actress Actor System — Architecture & Implementation Guide

## Overview

**Actress** is a concurrent actor framework written in Go. It models a system as a hierarchy of **processes** (actors) that communicate exclusively by passing **events** through channels. The framework provides built-in routers, error handling, and distributed processing scaffolding.

**Package**: `github.com/postmannen/actress`
**Status**: Active development — expect breaking changes between commits.

---

## 1. Core Concepts

### 1.1 Process (`*Process`)

A process is the fundamental unit of computation. It encapsulates:

- A **process function** (`fn func()`) — the goroutine body that does the work
- An **input channel** (`InCh chan Event`) — events are delivered here
- **Shared channels** — copied from parent, shared by all processes in the tree:
  - `StaticEventCh` — for `ET*` events
  - `ErrorEventCh` — for `ER*` events
  - `DynamicEventCh` — for `ED*` events
  - `CustomEventCh` — for `EC*` events
  - `SupervisorEventCh` — for `ES*` events
  - `TestCh` — primarily for testing
- **Process maps** — references to sibling process registries (static, dynamic, custom, error, supervisor)
- A **context** (`Ctx` / `Cancel`) — cancellation tree mirrors the process tree
- A **PID** (`pidnr`) — unique integer assigned from a shared counter
- **Configuration** (`Config`) — shared config from root

Every process also has an `Event` field (its event name/type) and a `Config` field.

### 1.2 Event (`Event`)

Events are the sole communication mechanism. They carry data, instructions, and workflow definitions:

```go
type Event struct {
    Nr            int               // Auto-incremented sequence number
    Name          EventName         // Identifies the target process type (e.g. "ETTest1")
    Cmd           []string          // Instructions or parameters
    Instruction   Instruction       // Single-word instruction for switch dispatch
    Data          []byte            // Payload data
    InternalCh    chan chan []byte  // Internal channel transfer (not serialized)
    Err           error             // Error message (used by ER* events)
    NextEvent     *Event            // Workflow chain: the next event to fire
    PreviousEvent *Event            // Auto-filled by routers with a copy of the event that triggered NextEvent
    DstNode       Node              // Target node for distributed delivery
    SrcNode       Node              // Source node (set by ETRemote router)
}
```

**Serialization tags**: `json`, `yaml`, `cbor` are all defined on `Name`, `Cmd`, `Data`, `Err`, `NextEvent`, `PreviousEvent`, `DstNode`, `SrcNode`. `InternalCh` is tagged `-` (not serialized).

**Construction**: Use `NewEvent(et EventName, opts ...EventOpt)` with functional options:
- `EvCmd([]string)` — set Cmd
- `EVData([]byte)` — set Data
- `EvNext(*Event)` — set NextEvent

### 1.3 Event Function (`ETFunc`)

The signature for process logic:

```go
type ETFunc func(context.Context, *Process) func()
```

An `ETFunc` receives the process context and process handle, and returns the actual goroutine function. This two-level function allows per-process setup before the main loop starts.

### 1.4 Type Aliases

| Type | Underlying | Purpose |
|------|-----------|---------|
| `EventName` | `string` | Identifies event/process type. Must start with `E` + type char |
| `Node` | `string` | Node name for distributed processing |
| `Instruction` | `string` | Single-word instruction for switch dispatch |
| `pidnr` | `int` | Process ID number |

---

## 2. Event Types & Process Maps

Event names use a two-character prefix to determine routing and process category:

| Prefix | Category | Process Map | Mutex | Deletable |
|--------|----------|-------------|-------|-----------|
| `ET*` | Static | `StaticProcesses` | Yes | **No** |
| `ED*` | Dynamic | `DynamicProcesses` | Yes | **Yes** |
| `ER*` | Error | `ErrorProcesses` | **No** | **No** |
| `EC*` | Custom | `CustomProcesses` | Yes | **Yes** |
| `ES*` | Supervisor | `supervisorProcesses` | Yes | **No** |

Each process map is a `map[EventName]*Process` with an optional mutex. The `errorProcesses` struct notably has **no mutex** — this is intentional and documented.

---

## 3. Event Routing Architecture

### 3.1 The Routing Pipeline

```
User calls p.AddEvent(event)
        │
        ▼
  Check DstNode ≠ current node?
        ├── Yes → Wrap in ETRemote event (original in NextEvent), send to StaticEventCh
        └── No  → Parse event.Name[1] to determine category:
                    'T' → StaticEventCh
                    'D' → DynamicEventCh
                    'R' → ErrorEventCh
                    'C' → CustomEventCh
                    'S' → SupervisorEventCh
```

Each channel is consumed by a dedicated **router process** that looks up the target process in the appropriate map and delivers the event to its `InCh`.

### 3.2 Router Behavior by Type

All five routers follow the same basic pattern:
1. Listen on their event channel
2. If `NextEvent != nil`, copy current event fields to `NextEvent.PreviousEvent` via `CopyEventFields()`
3. Look up target process in the map
4. Deliver event to `process.InCh`

**Differences**:
- **ETRouter**: Also handles `ETRemote` events — logs error if no process registered, otherwise routes normally
- **EDRouter / ECRouter**: If the target process is not yet registered, they spawn a goroutine that retries up to 3 times (1s sleep between retries), so the router doesn't block
- **ERRouter**: Direct lookup, no mutex protection (error processes map has no mutex)
- **ESRouter**: Direct lookup with mutex protection

### 3.3 NextEvent → PreviousEvent Copying

When any router sees `event.NextEvent != nil`, it calls `CopyEventFields(&event)` and assigns the result to `event.NextEvent.PreviousEvent`. This gives the next process in the chain access to metadata from the triggering event (sequence number, source node, etc.) without copying channels or large Data payloads.

`CopyEventFields` copies: `Nr`, `Name`, `Cmd`, `Instruction`, `Err`, `DstNode`, `SrcNode` — **not** `Data`, `InternalCh`, `NextEvent`, `PreviousEvent`.

---

## 4. Lifecycle

### 4.1 Bootstrap: `NewRootProcess`

```go
func NewRootProcess(ctx context.Context, fn ETFunc, conf *Config) *Process
```

Creates the root process and bootstraps the entire system in this order:

1. **Create root Process struct** — initializes all process maps, shared channels (buffered, cap 10), event counter, PID allocator, and context
2. **Set up logging** — configures `slog` based on `conf.LogLevel` (debug/info/error/fatal/none). Env var `LOGLEVEL` always wins
3. **Create registration collector** — `registerProcessInfo` slice to track processes for `ESProcesses`
4. **Start built-in error processes** (via `actForRoot`):
   - `ERLog` — error logging
   - `ERTest` — error testing
   - `ERNone` — error dropping
   - `ETPrint` — print to stdout
5. **Start built-in routers** (via `actForRoot`):
   - `ETRouter` — static event router
   - `ERRouter` — error event router
   - `EDRouter` — dynamic event router
   - `ECRouter` — custom event router
   - `ESRouter` — supervisor event router
6. **Start supervisor process** (via `actForRoot`):
   - `ESProcesses` — process tracking
7. **Start remaining built-ins** (via `actForRoot`):
   - `ETOsSignal` — SIGINT/CTRL+C handler
   - `ETTestCh` — test forwarding
   - `ETPid` — PID queries
   - `ETReadFile` — file reading
   - `ETDone` — event logging
   - `ETExit` — system exit
   - `ETPidGetAll` — CBOR-encoded PID map
8. **Call `RegisterProcessesInESProcesses`** — currently a no-op (body commented out)
9. **Return root process**

The root process's `Event` field is set to `ETRoot`. The `fn` parameter is optional — if nil, the root has no process function.

### 4.2 Child Creation: `NewProcess`

```go
func NewProcess(ctx context.Context, parentP *Process, event EventName, fn ETFunc) *Process
```

Creates a child process:
1. Creates a **new context** (cancelled when parent is cancelled)
2. Creates a **new unbuffered `InCh`** — unique to this process
3. **Copies references** to parent's shared channels (`StaticEventCh`, `ErrorEventCh`, `TestCh`, `DynamicEventCh`, `CustomEventCh`, `SupervisorEventCh`)
4. **Copies references** to all process maps (not copies — shared with parent)
5. **Copies** `EventNr`, `Config`, `pids` (shared PID allocator)
6. Assigns a **new PID** from the shared `pids` counter
7. Calls `fn(ctx, &p)` to get the process function

### 4.3 Starting: `Act()` vs `actForRoot()`

**`Act() error`** — for user-created processes:
1. Adds PID to `pids.toProc` map
2. Adds process to the appropriate process map (via `addToProcessesMap`)
3. If `fn != nil`: creates `readyCh`, spawns goroutine for `fn()`, calls `WaitForReady()`
4. Returns nil

**`actForRoot(pi *registerProcessInfo) error`** — internal, for root's built-ins:
1. Same PID/map registration as `Act()`
2. If `fn != nil`: spawns goroutine for `fn()` — **does NOT** create `readyCh` or call `WaitForReady()`
3. Appends process name to `pi` for later `ESProcesses` registration
4. Returns nil

### 4.4 Readiness: `SignalReady()` / `WaitForReady()`

- **`SignalReady()`**: Closes `readyCh` exactly once via `sync.Once`. Multiple calls are no-ops.
- **`WaitForReady()`**: Waits on `readyCh` with a 5ms timeout fallback. If the user forgot to call `SignalReady()`, it returns after 5ms.

### 4.5 Stopping: `Stop()`

```go
func (p *Process) Stop() error
```
1. Calls `Cancel()` on the process context
2. Removes process from its map (via `deleteFromProcessesMap`)
3. Removes PID from `pids.toProc`

**Deletion rules** (enforced by `deleteFromProcessesMap`):
- `ET*` (static): **Cannot delete** — switch case is empty
- `ED*` (dynamic): Deletes from `DynamicProcesses.procMap`
- `EC*` (custom): Deletes from `CustomProcesses.procMap`
- `ER*` (error): **Cannot delete** — switch case is empty
- `ES*` (supervisor): **Cannot delete** — switch case is empty

### 4.6 Event Numbering

A shared `eventNr` struct (mutex-protected) provides a global counter. `IncrementEventNr()` atomically increments and returns the new value. `CurrentEventNr()` returns the current value without incrementing. Every call to `AddEvent()` calls `IncrementEventNr()` to assign the event's `Nr` field.

---

## 5. Process Map Registration

### 5.1 `addToProcessesMap()`

When a process is added, it checks if a process for the same event name already exists in the map. If so, it **cancels** the existing process first, then replaces it. This allows process replacement/redefinition.

### 5.2 `deleteFromProcessesMap()`

Only `ED*` and `EC*` processes can be deleted. The `ET*`, `ER*`, and `ES*` cases are empty (with commented-out error logging).

---

## 6. Distributed Processing

### 6.1 ETRemote Event Wrapping

When `AddEvent()` is called with `event.DstNode` set to a value different from `p.Config.NodeName` and non-empty:

1. A new `ETRemote` event is created
2. The original event is placed in `NextEvent` of the wrapper
3. The wrapper is sent to `StaticEventCh`
4. The `ETRouter` delivers it to the user-provided `ETRemote` process function

The user must implement their own `ETFunc` for `ETRemote` and register it:

```go
actress.NewProcess(ctx, rootAct, actress.ETRemote, myRemoteFunc).Act()
```

The remote implementation typically:
- Extracts `ev.NextEvent` (the original event)
- Uses `ev.NextEvent.DstNode` as the delivery target (topic, subject, etc.)
- Marshals the event (JSON, CBOR, etc.) and sends it over the network
- Sets `SrcNode` on the original event for reply tracking

### 6.2 Receiving Remote Events

On the remote node, the user typically creates a receiver process (e.g., MQTT/NATS receiver) that unmarshals incoming events and calls `p.AddEvent(ev)` to inject them into the local routing system. The receiver should clear `ev.DstNode` to prevent forwarding loops.

---

## 7. Built-in Processes Reference

### 7.1 Routers

| Event Name | Channel | Behavior |
|-----------|---------|----------|
| `ETRouter` | `StaticEventCh` | Routes `ET*` events. Also handles `ETRemote`. Copies `NextEvent → PreviousEvent`. |
| `ERRouter` | `ErrorEventCh` | Routes `ER*` events. No mutex on lookup. Copies `NextEvent → PreviousEvent`. |
| `EDRouter` | `DynamicEventCh` | Routes `ED*` events. Retries 3× in goroutine if process not yet registered. |
| `ECRouter` | `CustomEventCh` | Routes `EC*` events. Retries 3× in goroutine if process not yet registered. |
| `ESRouter` | `SupervisorEventCh` | Routes `ES*` events. Mutex-protected lookup. Copies `NextEvent → PreviousEvent`. |

### 7.2 Static Processes (`ET*`)

| Event Name | Function | Description |
|-----------|----------|-------------|
| `ETRoot` | (user-provided or nil) | Root process handler |
| `ETRemote` | (user-provided) | Distributed delivery. User implements the transport logic. |
| `ETOsSignal` | `etOsSignalFn` | Waits for SIGINT, logs, calls `os.Exit(0)` |
| `ETTestCh` | `etTestChFn` | Forwards received events to `TestCh` |
| `ETPrint` | `etPrintFn` | Prints `event.Data` to stdout |
| `ETExit` | `etExitFn` | Logs and calls `os.Exit(0)` |
| `ETTest` | `ETTestfn(testCh)` | Factory: forwards `event.Data` to `testCh`, handles `InstructionCmdEOF` to close channel |
| `ETPid` | `etPidFn` | PID queries. `Cmd`: `["action", "pid", "name"]`. Actions: `pidGet`, `pidGetAll` |
| `ETPidGetAll` | `etPidGetAllFn` | Returns CBOR-encoded `PidVsProcMap` in `NextEvent.Data` |
| `ETReadFile` | `ETReadFileFn` | Reads file at `Cmd[0]`, puts content in `NextEvent.Data` |
| `ETDone` | `etDoneFn` | Logs received events at info/error level |

### 7.3 Error Processes (`ER*`)

| Event Name | Function | Description |
|-----------|----------|-------------|
| `ERLog` | `erLogFn` | Logs based on `Instruction`: `InstructionError` → slog.Error, `InstructionInfo` → slog.Info, `InstructionDebug` → slog.Debug, `InstructionFatal` → slog.Error + `os.Exit(1)` |
| `ERTest` | `erTestFn` | Error testing handler |
| `ERNone` | `erNoneFn` | Drops error events (for testing) |

### 7.4 Dynamic Processes (`ED*`)

| Event Name | Function | Description |
|-----------|----------|-------------|
| `EDSync` | `EDSyncFn(syncCh)` | Factory: signals `syncCh` when event received. Used for synchronizing one-off events. |

### 7.5 Custom Processes (`EC*`)

| Event Name | Function | Description |
|-----------|----------|-------------|
| `ECGeneralDelivery` | `ecGeneralDeliveryFn` | Forwards `event.Data` to `NextEvent` (testing) |

### 7.6 Supervisor Processes (`ES*`)

| Event Name | Function | Description |
|-----------|----------|-------------|
| `ESProcesses` | `esProcessesFn()` | Tracks running processes. Instructions: `InstructionESProcessesAdd`, `InstructionESProcessesDelete`, `InstructionESProcessesGetAll`. Uses CBOR-encoded `esProcessesMapDataIn` structs. |

---

## 8. Configuration

```go
type Config struct {
    CustomEvents     bool      // Enable custom events
    Metrics          bool      // Enable metrics
    CustomEventsPath string    // Path for custom events (default: "customevents")
    NodeName         Node      // Node name for distributed processing
    LogLevel         string    // Log level: "debug", "info", "error", "fatal", "none"
}
```

**Creation**:
```go
cfg, fs := actress.NewConfig(logLevel string)
// Modify cfg as needed, then fs.Parse(os.Args)
```

**Environment variable overrides** (always win over flag values):
- `CUSTOMEVENTS` — "1" or "true" → true
- `METRICS` — "1" or "true" → true
- `CUSTOMEVENTSPATH` — string value
- `NODENAME` — string value
- `LOGLEVEL` — log level string

---

## 9. Utilities

### 9.1 EventRW — `io.Reader` / `io.Writer` for Events

```go
type EventRW struct {
    P    *Process
    Ev   *Event
    Info string
    Pos  int
}
```

- **`Write(b []byte)`**: Sets `Ev.Data = b`, then calls `P.AddEvent(*Ev)` to send the event
- **`Read(b []byte)`**: Reads sequentially from `Ev.Data`, returns `io.EOF` when exhausted

Useful for integrating with Go's `io` ecosystem (e.g., `io.ReadAll(erw)`).

### 9.2 Buffer — Thread-Safe `bytes.Buffer`

```go
type Buffer struct {
    buffer bytes.Buffer
    mu     sync.Mutex
}
```

Wraps `bytes.Buffer` with mutex-protected `Read` and `Write` methods.

### 9.3 CopyEventFields

```go
func CopyEventFields(ev *Event) *Event
```

Creates a shallow copy of an event's metadata fields only: `Nr`, `Name`, `Cmd`, `Instruction`, `Err`, `DstNode`, `SrcNode`. **Excludes**: `Data`, `InternalCh`, `NextEvent`, `PreviousEvent`. Returns `nil` if input is `nil`.

### 9.4 NewUUID

```go
func NewUUID() string
```

Returns a UUID-prefixed dynamic event name in format `ED-<uuid>`. Used for creating unique names for dynamic processes.

---

## 10. Key Principles & Gotchas

1. **Event names must start with `E`** followed by a type character: `T` (static), `D` (dynamic), `R` (error), `C` (custom), `S` (supervisor). `AddEvent` panics otherwise.

2. **`NewProcess` does NOT start the process** — you must call `.Act()` separately.

3. **All processes in a tree share** the same channels (`StaticEventCh`, `ErrorEventCh`, etc.) and process maps. Only `InCh` is unique per process.

4. **Error processes have no mutex** — accessing `ErrorProcesses.procMap` concurrently is not protected. Be aware.

5. **`ETRemote` is just a constant** (`const ETRemote EventName = "ETRemote"`). No built-in implementation exists. You must create your own `ETFunc` and register it.

6. **`actForRoot` vs `Act`**: During root bootstrap, built-in processes use `actForRoot` which skips the readiness mechanism (`readyCh`/`WaitForReady`) because the routers and error processes need to be available before other processes can register with `ESProcesses`.

7. **`RegisterProcessesInESProcesses` is currently a no-op** — the body is commented out. The infrastructure (`registerProcessInfo`, `actForRoot`'s collection logic) is in place but not active.

8. **State sharing**: Normally avoid shared state by passing data via events. If needed, share state by passing the same struct/pointer to multiple process functions via the `ETFunc`.

9. **Process replacement**: Adding a process with an event name that already exists cancels the old process first. This allows redefinition.

10. **Remote events are wrapped**: When `DstNode` is set, `AddEvent` wraps the original event in an `ETRemote` event with the original in `NextEvent`. The `ETRemote` process unwraps it and handles the transport.
