---
name: skyvisor-flight-ops
description: Use when tracking flights, diagnosing airport delays, monitoring air-cargo disruption, or running an operational decision loop over them — checking a flight's status or risk, watching a flight for changes, testing what a delay does to a connection, or recording and scoring an operations decision. Connects to SkyVisor over MCP.
---

# SkyVisor flight operations

SkyVisor answers questions about flights and airports, and — unusually — records
the decisions taken in response so their accuracy can be measured later. Use
the read tools freely; treat the action tools as writes to someone's operational
record.

## Connecting

SkyVisor is a remote MCP server at `https://mcp.skyvisor.app`. It connects over
OAuth, so there is no API key to ask the user for:

```sh
claude mcp add --transport http skyvisor https://mcp.skyvisor.app
```

The host opens a browser for consent. If tools return `403
insufficient_scope`, the connection was approved read-only — tell the user to
reconnect and approve actions, rather than retrying.

If tools return `402` with `usage_limit_reached`, the account's daily quota is
spent. Free includes a small action budget; say which quota ran out instead of
retrying.

## Choosing a tool

| Question | Tool |
|---|---|
| Where is this flight, is it late | `get_flight` |
| What is happening at this airport | `get_airport_board` |
| How often is this route or airport late | `run_analytics` |
| What should I be worried about right now | `get_operations_dashboard` |
| What cargo is disrupted (Business) | `get_logistics_overview` |
| What is saved on this account | `list_trips`, `list_watches` |
| How accurate have our calls been | `get_decision_trust` |
| What did I miss since last time | `list_agent_inbox` |

Start with `get_operations_dashboard` when the user asks an open question about
their own operation — it is account-scoped and ranks what matters, so it avoids
guessing which flight they meant.

## Catching up after a gap

You are disconnected between turns, and the event stream drops what it cannot
deliver. `list_agent_inbox` is the durable queue that survives that gap.

1. `list_agent_inbox` at the start of a session, or whenever the user asks what
   changed. It returns oldest first with a `pending` total.
2. Report what actually matters rather than replaying the list. Several events
   for one flight usually describe one story.
3. `ack_agent_inbox` with the IDs you reported, so the next drain does not
   repeat them. Do not acknowledge events you have not surfaced — that is the
   only way they get lost.
4. If `truncated` is true, drain again before concluding you are caught up.

Acknowledging costs no action quota and works on a read-only connection, so
there is no reason to skip it to save budget.

## Recipes

### Is my connection at risk?

1. `get_flight` on the inbound segment for current status and delay.
2. `trip_what_if` with the observed delay on that segment. It re-scores the
   connection and does **not** persist anything, so it is safe to run while
   exploring.
3. Report the re-scored connection risk and the slack left, not just the delay.

### Watch a flight and get told when it changes

1. `get_flight` first, to confirm the flight number resolves and is the one the
   user means.
2. `create_watch`. This is an action: it consumes action quota and, on Free,
   counts against a 5-watch limit.
3. Confirm what will trigger an alert rather than implying continuous polling.

### Diagnose a delay pattern before committing to a route

1. `run_analytics` for the route or airport. Free is capped to a 7-day window,
   Pro and Business to 90 — state the window with the numbers, because a 7-day
   sample is weak evidence.
2. `get_airport_board` for the current picture, to separate a systemic pattern
   from today's weather.

### Record an operations decision so it can be scored (Business)

1. `create_operational_case` with the shipment or passenger dependency and SLA
   exposure.
2. `create_decision_record` with the signal, the prediction, your confidence,
   and the recommendation. Set approval-required for anything with external
   cost.
3. `record_decision_action` to approve, reject, or execute. Approval-required
   execution is refused until approved — that refusal is the control working,
   not an error to route around.
4. Later, `record_decision_outcome` with what actually happened and the cost
   avoided. Without this step `get_decision_trust` has nothing to measure, so
   the loop is only worth starting if the outcome will be recorded.

## Rules

- Read before writing. Confirm the flight or case exists before creating
  anything against it.
- Never invent a flight number, IATA code, or case ID. If the user's reference
  is ambiguous, ask.
- Say when data is stale. Read tools carry provenance and freshness; a
  confident answer over a stale record is worse than saying it is stale.
- Publishing a public trust report is deliberately not available over MCP — it
  exposes customer data on an unauthenticated URL, so it stays a human action
  in the web UI. Direct the user there rather than looking for a tool.
- One what-if at a time. `trip_what_if` scores a single segment's delay;
  chaining guesses compounds error without saying so.
