# SkyVisor MCP

Official Go SDK MCP server for safe access to SkyVisor travel data. Credentials never enter tool arguments or model context.

## Transports

### Stdio (local / Claude Desktop / Cursor)

```sh
export SKYVISOR_API_URL=https://api.skyvisor.app
export SKYVISOR_OIDC_ACCESS_TOKEN=replace-with-short-lived-oidc-access-token
go run ./cmd/skyvisor-mcp
# MCP_TRANSPORT defaults to stdio
```

### Streamable HTTP (Kubernetes)

```sh
export MCP_TRANSPORT=http
export ADDR=0.0.0.0:8087
export SKYVISOR_API_URL=http://127.0.0.1:8080   # or in-cluster http://api.staging.svc.cluster.local:8080
go run ./cmd/skyvisor-mcp
```

Clients POST to the server URL with `Authorization: Bearer <OIDC access token>` (API audience). Optional `SKYVISOR_OIDC_ACCESS_TOKEN` is a fallback for smoke tests only.

Staging / production ingress (after cluster apply): `https://staging-mcp.skyvisor.app` · `https://mcp.skyvisor.app`.

All API calls send `X-SkyVisor-Client: mcp` for usage metering. Free plan is MCP read-only. Check quotas with `get_usage` or `GET /v1/usage`. In-app connect docs: web `/mcp`.

## Tools

- `get_flight`: current provider-backed flight record
- `get_airport_board`: arrivals/departures for one airport
- `run_analytics`: on-time / delay analytics (Free capped to 7-day window)
- `get_logistics_overview`: Business cargo + disruption snapshot
- `list_trips`: saved trips
- `create_trip`: create a named trip with optional flight numbers (MCP action)
- `list_watches` / `create_watch`: flight watches (`create_watch` = MCP action)
- `ask_travel_assistant`: grounded travel guidance; optional `trip_id` (assistant quota)
- `trip_what_if`: delay scenario on one segment; re-scores connections; does not persist
- `get_usage`: UTC-day MCP + assistant counters vs plan limits
- `get_operations_dashboard`: account-scoped priority queue, watched-flight risk, connection risk, and data freshness
- `list_operational_cases` / `get_operational_case`: Business case queue, decisions, outcomes, and audit history
- `create_operational_case`: persist shipment/passenger dependencies, SLA exposure, owners, and approved alternatives (action)
- `create_decision_record`: persist signal, prediction, confidence, recommendation, and approval requirement (action)
- `record_decision_action`: approve, reject, or execute; approval-required execution is rejected until approved (action)
- `record_decision_outcome`: record actual prediction/action result and avoided cost (action)
- `get_decision_trust`: measured precision, false-positive rate, lead time, action success, and scope breakdowns
- `list_trust_shares` / `revoke_trust_share`: manage published public trust report links (`revoke_trust_share` = MCP action); creating a share is web-only by design — publishing exposes customer data on an unauthenticated URL, so it stays a human-approved action in the web UI and is deliberately not exposed over MCP
- `list_webhook_integrations`: configured workflow destinations and delivery status
- `create_webhook_integration`: signed public-HTTPS endpoint; signing secret returned once (action)
- `test_webhook_integration`: sends and audits a signed test delivery (action)

## Remote connection (Streamable HTTP)

The deployed server listens at `https://mcp.skyvisor.app` (staging:
`https://staging-mcp.skyvisor.app`). Authenticate with a personal access
token created on the web app under Settings → MCP connector tokens:

```json
{
  "url": "https://mcp.skyvisor.app",
  "headers": { "Authorization": "Bearer svr_pat_..." }
}
```

PATs are long-lived, revocable, and validated by the API on every call; an
OIDC access token also works for short sessions. Local development keeps the
stdio transport: `MCP_TRANSPORT=stdio` (default) with
`SKYVISOR_OIDC_ACCESS_TOKEN` set.
# skyvisor-mcp
# skyvisor-mcp
