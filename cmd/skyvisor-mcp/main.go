package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/SkyvisorInsights/skyvisor-mcp/internal/apiclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type flightInput struct {
	Number string `json:"number" jsonschema:"IATA flight number, for example TP1363"`
}
type situationLayersInput struct{}

type situationNewsInput struct {
	Languages string `json:"languages,omitempty" jsonschema:"comma-separated BCP-47 codes such as en,fr; omit for every language"`
	Countries string `json:"countries,omitempty" jsonschema:"comma-separated ISO 3166-1 alpha-2 codes such as US,GB"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"next_cursor from a previous call; omit for the newest page"`
	Limit     int    `json:"limit,omitempty" jsonschema:"items per page, default 50, maximum 200"`
}

type situationPointInput struct {
	ICAO      string  `json:"icao,omitempty" jsonschema:"aerodrome code such as LPPT or LIS; preferred over coordinates"`
	Latitude  float64 `json:"lat,omitempty" jsonschema:"latitude, used when no icao is given"`
	Longitude float64 `json:"lon,omitempty" jsonschema:"longitude, used when no icao is given"`
}

type airportBoardInput struct {
	IATA      string `json:"iata" jsonschema:"3-letter IATA airport code, for example LIS"`
	Direction string `json:"direction,omitempty" jsonschema:"arrivals, departures, or both (default both)"`
	Status    string `json:"status,omitempty" jsonschema:"optional flight_status filter such as scheduled or active"`
	Limit     int    `json:"limit,omitempty" jsonschema:"max flights per side, default 40"`
}
type analyticsInput struct {
	Airline    string `json:"airline,omitempty" jsonschema:"airline IATA code such as TP"`
	Airport    string `json:"airport,omitempty" jsonschema:"airport IATA code such as LIS"`
	WindowDays int    `json:"window_days,omitempty" jsonschema:"requested lookback days; Free capped at 7, Pro at 90"`
	Limit      int    `json:"limit,omitempty" jsonschema:"max flights in the sample"`
}
type logisticsInput struct {
	Airline string `json:"airline,omitempty" jsonschema:"cargo airline IATA such as FX"`
	Airport string `json:"airport,omitempty" jsonschema:"hub airport IATA such as MEM"`
	Limit   int    `json:"limit,omitempty" jsonschema:"max flights in the sample"`
}
type tripInput struct {
	Name    string   `json:"name" jsonschema:"short human-readable trip name"`
	Flights []string `json:"flights,omitempty" jsonschema:"optional IATA flight numbers to attach"`
}
type assistantInput struct {
	Question string `json:"question" jsonschema:"travel question that does not require inventing live data"`
	TripID   string `json:"trip_id,omitempty" jsonschema:"optional trip UUID to ground the answer on live segment facts"`
}
type whatIfInput struct {
	TripID                string `json:"trip_id" jsonschema:"trip UUID to evaluate"`
	SegmentIndex          int    `json:"segment_index" jsonschema:"0-based segment to delay"`
	DelayDepartureMinutes int    `json:"delay_departure_minutes,omitempty" jsonschema:"extra departure delay in minutes"`
	DelayArrivalMinutes   int    `json:"delay_arrival_minutes,omitempty" jsonschema:"extra arrival delay in minutes"`
	Question              string `json:"question,omitempty" jsonschema:"optional grounded co-pilot question about the scenario"`
}
type operationalCasesInput struct {
	Status string `json:"status,omitempty" jsonschema:"optional case status filter: open, monitoring, action_required, resolved, or closed"`
}
type operationalCaseIDInput struct {
	CaseID string `json:"case_id" jsonschema:"operational case UUID"`
}
type createOperationalCaseInput struct {
	Kind             string   `json:"kind" jsonschema:"shipment, passenger, crew, aircraft, or other"`
	Reference        string   `json:"reference" jsonschema:"customer shipment, PNR, crew, or aircraft reference"`
	Title            string   `json:"title" jsonschema:"short operational case title"`
	Description      string   `json:"description,omitempty"`
	TripID           string   `json:"trip_id,omitempty"`
	FlightNumbers    []string `json:"flight_numbers,omitempty"`
	Severity         string   `json:"severity,omitempty" jsonschema:"low, medium, high, or critical"`
	Owner            string   `json:"owner,omitempty"`
	EscalationPolicy string   `json:"escalation_policy,omitempty"`
	SLACurrency      string   `json:"sla_currency,omitempty" jsonschema:"ISO 4217 currency code"`
	SLAValueMinor    int64    `json:"sla_value_minor,omitempty" jsonschema:"SLA value in minor currency units"`
	FailureCostMinor int64    `json:"failure_cost_minor,omitempty" jsonschema:"estimated failure cost in minor currency units"`
}
type createDecisionInput struct {
	CaseID               string         `json:"case_id"`
	SignalType           string         `json:"signal_type"`
	SignalSource         string         `json:"signal_source"`
	SignalObservedAt     string         `json:"signal_observed_at,omitempty" jsonschema:"RFC3339 timestamp; current time when omitted"`
	Evidence             map[string]any `json:"evidence,omitempty"`
	PredictionType       string         `json:"prediction_type"`
	PredictedProbability *float64       `json:"predicted_probability,omitempty"`
	Confidence           *float64       `json:"confidence,omitempty"`
	HorizonMinutes       int            `json:"horizon_minutes,omitempty"`
	ModelVersion         string         `json:"model_version,omitempty"`
	ScopeAirport         string         `json:"scope_airport,omitempty"`
	ScopeAirline         string         `json:"scope_airline,omitempty"`
	ScopeRoute           string         `json:"scope_route,omitempty"`
	RecommendationType   string         `json:"recommendation_type"`
	Recommendation       string         `json:"recommendation"`
	Rationale            string         `json:"rationale,omitempty"`
	ProposedAction       string         `json:"proposed_action,omitempty"`
	ApprovalRequired     bool           `json:"approval_required,omitempty"`
}
type decisionActionInput struct {
	CaseID      string `json:"case_id"`
	DecisionID  string `json:"decision_id"`
	Decision    string `json:"decision" jsonschema:"approved, rejected, or executed"`
	ActionTaken string `json:"action_taken,omitempty"`
	Notes       string `json:"notes,omitempty"`
	ExternalRef string `json:"external_ref,omitempty"`
}
type decisionOutcomeInput struct {
	CaseID           string         `json:"case_id"`
	DecisionID       string         `json:"decision_id"`
	PredictionResult string         `json:"prediction_result" jsonschema:"occurred, not_occurred, or unknown"`
	ActionResult     string         `json:"action_result" jsonschema:"succeeded, failed, no_action, or unknown"`
	Actual           map[string]any `json:"actual,omitempty"`
	AvoidedCostMinor int64          `json:"avoided_cost_minor,omitempty"`
	Notes            string         `json:"notes,omitempty"`
}
type createWebhookInput struct {
	Name   string   `json:"name"`
	URL    string   `json:"url" jsonschema:"public HTTPS webhook destination"`
	Events []string `json:"events" jsonschema:"case.created, case.updated, decision.proposed, decision.approved, decision.rejected, decision.executed, decision.evaluated"`
}
type webhookIDInput struct {
	IntegrationID string `json:"integration_id"`
}
type revokeTrustShareInput struct {
	Token string `json:"token" jsonschema:"the trust share token to revoke"`
}

type flightOutput struct {
	Flight apiclient.Flight `json:"flight" jsonschema:"current flight record"`
}
type situationLayersOutput struct {
	Layers []apiclient.SituationLayer `json:"layers" jsonschema:"every known layer, with entitlement, availability, freshness and required attribution"`
}

type situationNewsOutput struct {
	Page apiclient.SituationNewsPage `json:"page" jsonschema:"one page of the news rail plus the cursor for the next"`
}

type situationPointOutput struct {
	Point apiclient.SituationPoint `json:"point" jsonschema:"conditions by block; a block with freshness unknown could not be reached"`
}

type airportBoardOutput struct {
	Board apiclient.AirportBoard `json:"board" jsonschema:"arrivals and departures for the airport"`
}
type analyticsOutput struct {
	Report apiclient.AnalyticsReport `json:"report" jsonschema:"analytics summary, routes, and airports"`
}
type logisticsOutput struct {
	Overview apiclient.LogisticsOverview `json:"overview" jsonschema:"cargo flights and disruption signals"`
}
type tripsOutput struct {
	Trips []apiclient.Trip `json:"trips" jsonschema:"saved trips"`
}
type tripOutput struct {
	Trip apiclient.Trip `json:"trip" jsonschema:"created trip"`
}
type assistantOutput struct {
	Answer string `json:"answer" jsonschema:"travel assistant response"`
}
type whatIfOutput struct {
	Result apiclient.WhatIfResult `json:"result" jsonschema:"scenario trip, baseline connections, and summary"`
}
type watchesOutput struct {
	Watches []apiclient.Watch `json:"watches" jsonschema:"active watches"`
}
type watchOutput struct {
	Watch apiclient.Watch `json:"watch" jsonschema:"created watch"`
}
type agentInboxInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"maximum events to return; server default 20, maximum 100"`
}
type agentInboxOutput struct {
	Inbox apiclient.AgentInboxPage `json:"inbox" jsonschema:"pending watch notifications, oldest first"`
}
type ackAgentInboxInput struct {
	EventIDs []string `json:"event_ids" jsonschema:"IDs of events to acknowledge, from list_agent_inbox"`
}
type ackAgentInboxOutput struct {
	Result apiclient.AgentInboxAck `json:"result" jsonschema:"how many were acknowledged and how many remain"`
}
type usageOutput struct {
	Usage apiclient.UsageSnapshot `json:"usage" jsonschema:"daily usage vs plan limits"`
}
type operationsOutput struct {
	Dashboard apiclient.OperationsDashboard `json:"dashboard" jsonschema:"account-scoped risks, watches, connections, and data freshness"`
}
type operationalCasesOutput struct {
	Cases []apiclient.OperationalCase `json:"cases"`
}
type operationalCaseOutput struct {
	Case apiclient.OperationalCase `json:"case"`
}
type operationalCaseDetailOutput struct {
	Detail apiclient.OperationalCaseDetail `json:"detail"`
}
type decisionOutput struct {
	Decision apiclient.DecisionRecord `json:"decision"`
}
type trustOutput struct {
	Trust apiclient.DecisionTrustMetrics `json:"trust"`
}
type webhooksOutput struct {
	Integrations []apiclient.WebhookIntegration `json:"integrations"`
}
type webhookCreatedOutput struct {
	Created apiclient.WebhookIntegrationCreated `json:"created"`
}
type webhookDeliveryOutput struct {
	Delivery apiclient.WebhookDelivery `json:"delivery"`
}
type trustSharesOutput struct {
	Shares []apiclient.TrustShareLink `json:"shares" jsonschema:"active public trust report links"`
}
type revokeTrustShareOutput struct {
	Revoked bool `json:"revoked"`
}
type watchInput struct {
	Number string `json:"number" jsonschema:"IATA flight number to watch"`
}

func main() {
	apiURL := value("SKYVISOR_API_URL", "http://127.0.0.1:8080")
	transport := strings.ToLower(value("MCP_TRANSPORT", "stdio"))

	switch transport {
	case "http", "streamable", "streamable-http":
		if err := runHTTP(apiURL); err != nil {
			slog.Error("run MCP HTTP server", "error", err)
			os.Exit(1)
		}
	case "stdio", "":
		client, err := apiclient.New(apiURL, os.Getenv("SKYVISOR_OIDC_ACCESS_TOKEN"))
		if err != nil {
			slog.Error("configure API client", "error", err)
			os.Exit(1)
		}
		server := newMCPServer(client)
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("run MCP server", "error", err)
			os.Exit(1)
		}
	default:
		slog.Error("unknown MCP_TRANSPORT", "value", transport, "want", "stdio|http")
		os.Exit(1)
	}
}

func runHTTP(apiURL string) error {
	addr := value("ADDR", "0.0.0.0:8087")
	fallbackToken := strings.TrimSpace(os.Getenv("SKYVISOR_OIDC_ACCESS_TOKEN"))
	// resourceURL is this server's own public identity and authServerURL is
	// where a client should go to get a token. Both are advertised through
	// RFC 9728 metadata so an MCP client can connect without a human pasting
	// a token, which is the whole point of one-click connect.
	resourceURL := strings.TrimRight(value("MCP_PUBLIC_URL", "http://127.0.0.1:"+portFromAddr(addr)), "/")
	authServerURL := strings.TrimRight(value("SKYVISOR_AUTH_SERVER_URL", apiURL), "/")

	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		token := bearerToken(r)
		if token == "" {
			token = fallbackToken
		}
		if token == "" {
			return nil
		}
		client, err := apiclient.New(apiURL, token)
		if err != nil {
			slog.Warn("reject MCP session", "error", err)
			return nil
		}
		return newMCPServer(client)
	}, &mcp.StreamableHTTPOptions{Stateless: true})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/.well-known/oauth-protected-resource", protectedResourceMetadata(resourceURL, authServerURL))
	mux.Handle("/", requireBearerOrFallback(fallbackToken != "", resourceURL, mcpHandler))

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("MCP streamable HTTP listening", "addr", addr, "api", apiURL)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// protectedResourceMetadata serves RFC 9728 metadata. A client reads it to
// discover which authorization server issues tokens for this resource, which
// is what lets it start an OAuth flow on its own.
func protectedResourceMetadata(resourceURL, authServerURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"resource":                 resourceURL,
			"authorization_servers":    []string{authServerURL},
			"scopes_supported":         []string{"skyvisor:read", "skyvisor:act"},
			"bearer_methods_supported": []string{"header"},
		}); err != nil {
			slog.Error("write protected resource metadata", "error", err)
		}
	}
}

func requireBearerOrFallback(allowFallback bool, resourceURL string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if bearerToken(r) == "" && !allowFallback {
			// The WWW-Authenticate challenge is what actually starts
			// discovery: RFC 9728 has clients follow resource_metadata from a
			// 401 to find the authorization server. Without this header a
			// client shows an auth error instead of opening a browser, so
			// one-click connect never begins.
			w.Header().Set("WWW-Authenticate",
				`Bearer resource_metadata="`+resourceURL+`/.well-known/oauth-protected-resource"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"missing_bearer","message":"Authorization required. Connect via OAuth or supply a bearer token."}` + "\n"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// portFromAddr extracts the port from a listen address so the default public
// URL matches the port actually bound.
func portFromAddr(addr string) string {
	if _, port, err := net.SplitHostPort(addr); err == nil && port != "" {
		return port
	}
	return "8087"
}

func bearerToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(h) < 8 || !strings.EqualFold(h[:7], "bearer ") {
		return ""
	}
	return strings.TrimSpace(h[7:])
}

func newMCPServer(client *apiclient.Client) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "skyvisor-travel", Version: "0.1.0"}, &mcp.ServerOptions{Instructions: "Use SkyVisor tools for the user's flight and trip data. Treat provider status as time-sensitive and never infer a live status."})
	mcp.AddTool(server, &mcp.Tool{Name: "get_flight", Description: "Get the latest available record for one IATA flight number."}, getFlight(client))
	mcp.AddTool(server, &mcp.Tool{Name: "get_airport_board", Description: "Get arrivals and/or departures for one IATA airport (for example LIS)."}, getAirportBoard(client))
	mcp.AddTool(server, &mcp.Tool{Name: "run_analytics", Description: "Run on-time and delay analytics for an airline and/or airport. Free plans are capped to a 7-day window."}, runAnalytics(client))
	mcp.AddTool(server, &mcp.Tool{Name: "get_logistics_overview", Description: "Business-only cargo and disruption overview for logistics ops."}, getLogistics(client))
	mcp.AddTool(server, &mcp.Tool{Name: "list_trips", Description: "List the user's saved SkyVisor trips."}, listTrips(client))
	mcp.AddTool(server, &mcp.Tool{Name: "create_trip", Description: "Create a SkyVisor trip and optionally attach flight numbers. Counts as an MCP action; every plan including Free has a daily action budget. Check get_usage if unsure."}, createTrip(client))
	mcp.AddTool(server, &mcp.Tool{Name: "list_watches", Description: "List active flight watches for the authenticated account."}, listWatches(client))
	mcp.AddTool(server, &mcp.Tool{Name: "create_watch", Description: "Start watching a flight number. Counts as an MCP action; every plan including Free has a daily action budget, and Free also caps concurrent watches. Try it rather than assuming the plan forbids it."}, createWatch(client))
	mcp.AddTool(server, &mcp.Tool{Name: "list_agent_inbox", Description: "Drain watch notifications that arrived while this agent was not connected, oldest first. Call this to catch up on flight changes rather than re-polling every watch. Read tool."}, listAgentInbox(client))
	mcp.AddTool(server, &mcp.Tool{Name: "ack_agent_inbox", Description: "Acknowledge inbox events so later drains skip them. Does not delete the record; only this account's queue advances. Does not consume the action quota."}, ackAgentInbox(client))
	mcp.AddTool(server, &mcp.Tool{Name: "get_usage", Description: "Return today's MCP and assistant usage counters vs plan limits."}, getUsage(client))
	mcp.AddTool(server, &mcp.Tool{Name: "get_operations_dashboard", Description: "Return the authenticated account's priority queue, watched-flight risk, connection risk, and source freshness."}, getOperationsDashboard(client))
	mcp.AddTool(server, &mcp.Tool{Name: "list_operational_cases", Description: "Business-only list of shipment, passenger, crew, aircraft, and other operational cases."}, listOperationalCases(client))
	mcp.AddTool(server, &mcp.Tool{Name: "get_operational_case", Description: "Load one operational case with its decision history and immutable audit trail."}, getOperationalCase(client))
	mcp.AddTool(server, &mcp.Tool{Name: "create_operational_case", Description: "Create customer-specific operational context. Governed MCP action; Business plan required."}, createOperationalCase(client))
	mcp.AddTool(server, &mcp.Tool{Name: "create_decision_record", Description: "Persist a signal, prediction, confidence, recommendation, and approval requirement for an operational case."}, createDecisionRecord(client))
	mcp.AddTool(server, &mcp.Tool{Name: "record_decision_action", Description: "Approve, reject, or execute a recommendation. Approval-required actions cannot execute before approval."}, recordDecisionAction(client))
	mcp.AddTool(server, &mcp.Tool{Name: "record_decision_outcome", Description: "Record what actually happened and whether the action succeeded so trust metrics can be evaluated."}, recordDecisionOutcome(client))
	mcp.AddTool(server, &mcp.Tool{Name: "get_decision_trust", Description: "Return measured prediction precision, false-positive rate, lead time, action success, and avoided cost."}, getDecisionTrust(client))
	mcp.AddTool(server, &mcp.Tool{Name: "list_trust_shares", Description: "List active public trust report links for the authenticated Business account."}, listTrustShares(client))
	mcp.AddTool(server, &mcp.Tool{Name: "revoke_trust_share", Description: "Revoke a published public trust report link. Creating one is intentionally not available over MCP: publishing a trust report is a human-approved, web-only action."}, revokeTrustShare(client))
	mcp.AddTool(server, &mcp.Tool{Name: "list_webhook_integrations", Description: "List configured Business workflow webhooks and their latest delivery status."}, listWebhookIntegrations(client))
	mcp.AddTool(server, &mcp.Tool{Name: "create_webhook_integration", Description: "Create a signed HTTPS workflow webhook. The signing secret is returned once. Governed MCP action."}, createWebhookIntegration(client))
	mcp.AddTool(server, &mcp.Tool{Name: "test_webhook_integration", Description: "Send a signed test event and record the delivery result. Governed MCP action."}, testWebhookIntegration(client))
	mcp.AddTool(server, &mcp.Tool{Name: "get_situation_layers", Description: "List the global situation layers this account can read: hazards, seismic, fire, conflict and news. Says which are entitled, which need a credential this deployment lacks, how fresh each is, and the attribution its licence requires."}, getSituationLayers(client))
	mcp.AddTool(server, &mcp.Tool{Name: "get_situation_news", Description: "Read the world news rail, newest first, with keyset paging. Available on every plan. Each item carries the attribution its licence requires; reproduce it when quoting."}, getSituationNews(client))
	mcp.AddTool(server, &mcp.Tool{Name: "get_situation_point", Description: "Current conditions for one place, by ICAO aerodrome code or by lat/lon: weather, sea state, air quality, and (paid plans) METAR and TAF. A block marked freshness unknown could not be reached and must not be reported as calm."}, getSituationPoint(client))
	mcp.AddTool(server, &mcp.Tool{Name: "ask_travel_assistant", Description: "Ask for concise travel guidance. Live facts remain grounded in SkyVisor provider data. Optionally pass trip_id. Counts toward assistant daily quota."}, askAssistant(client))
	mcp.AddTool(server, &mcp.Tool{Name: "trip_what_if", Description: "Simulate a delay on one trip segment and re-score connection risk without persisting. Counts as MCP action + assistant when AI is enabled."}, tripWhatIf(client))
	return server
}

func getSituationLayers(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, situationLayersInput) (*mcp.CallToolResult, situationLayersOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ situationLayersInput) (*mcp.CallToolResult, situationLayersOutput, error) {
		layers, err := client.SituationLayers(ctx)
		return nil, situationLayersOutput{Layers: layers}, err
	}
}

func getSituationNews(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, situationNewsInput) (*mcp.CallToolResult, situationNewsOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input situationNewsInput) (*mcp.CallToolResult, situationNewsOutput, error) {
		page, err := client.SituationNews(ctx, input.Languages, input.Countries, input.Cursor, input.Limit)
		return nil, situationNewsOutput{Page: page}, err
	}
}

func getSituationPoint(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, situationPointInput) (*mcp.CallToolResult, situationPointOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input situationPointInput) (*mcp.CallToolResult, situationPointOutput, error) {
		// Either form is valid, but not neither: without a place the server
		// would have to guess, and 0,0 is in the Gulf of Guinea.
		if strings.TrimSpace(input.ICAO) == "" && input.Latitude == 0 && input.Longitude == 0 {
			return nil, situationPointOutput{}, errors.New("give icao, or lat and lon")
		}
		point, err := client.SituationPoint(ctx, input.ICAO, input.Latitude, input.Longitude)
		return nil, situationPointOutput{Point: point}, err
	}
}

func getFlight(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, flightInput) (*mcp.CallToolResult, flightOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input flightInput) (*mcp.CallToolResult, flightOutput, error) {
		if strings.TrimSpace(input.Number) == "" {
			return nil, flightOutput{}, errors.New("number is required")
		}
		flight, err := client.Flight(ctx, input.Number)
		return nil, flightOutput{Flight: flight}, err
	}
}

func getAirportBoard(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, airportBoardInput) (*mcp.CallToolResult, airportBoardOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input airportBoardInput) (*mcp.CallToolResult, airportBoardOutput, error) {
		if strings.TrimSpace(input.IATA) == "" {
			return nil, airportBoardOutput{}, errors.New("iata is required")
		}
		board, err := client.AirportBoard(ctx, input.IATA, input.Direction, input.Status, input.Limit)
		return nil, airportBoardOutput{Board: board}, err
	}
}

func runAnalytics(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, analyticsInput) (*mcp.CallToolResult, analyticsOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input analyticsInput) (*mcp.CallToolResult, analyticsOutput, error) {
		if strings.TrimSpace(input.Airline) == "" && strings.TrimSpace(input.Airport) == "" {
			return nil, analyticsOutput{}, errors.New("airline or airport is required")
		}
		report, err := client.Analytics(ctx, input.Airline, input.Airport, input.WindowDays, input.Limit)
		return nil, analyticsOutput{Report: report}, err
	}
}

func getLogistics(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, logisticsInput) (*mcp.CallToolResult, logisticsOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input logisticsInput) (*mcp.CallToolResult, logisticsOutput, error) {
		overview, err := client.LogisticsOverview(ctx, input.Airline, input.Airport, input.Limit)
		return nil, logisticsOutput{Overview: overview}, err
	}
}

func listTrips(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, tripsOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, tripsOutput, error) {
		items, err := client.Trips(ctx)
		return nil, tripsOutput{Trips: items}, err
	}
}

func createTrip(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, tripInput) (*mcp.CallToolResult, tripOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input tripInput) (*mcp.CallToolResult, tripOutput, error) {
		if strings.TrimSpace(input.Name) == "" {
			return nil, tripOutput{}, errors.New("name is required")
		}
		trip, err := client.CreateTrip(ctx, input.Name, input.Flights)
		return nil, tripOutput{Trip: trip}, err
	}
}

func listWatches(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, watchesOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, watchesOutput, error) {
		items, err := client.Watches(ctx)
		return nil, watchesOutput{Watches: items}, err
	}
}

func createWatch(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, watchInput) (*mcp.CallToolResult, watchOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input watchInput) (*mcp.CallToolResult, watchOutput, error) {
		if strings.TrimSpace(input.Number) == "" {
			return nil, watchOutput{}, errors.New("number is required")
		}
		watch, err := client.CreateWatch(ctx, input.Number)
		return nil, watchOutput{Watch: watch}, err
	}
}

// listAgentInbox drains watch notifications that accumulated while the agent
// was not connected. The SSE stream cannot serve this: it drops events for
// consumers that are not attached, and an agent is detached between turns.
func listAgentInbox(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, agentInboxInput) (*mcp.CallToolResult, agentInboxOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input agentInboxInput) (*mcp.CallToolResult, agentInboxOutput, error) {
		page, err := client.AgentInbox(ctx, input.Limit)
		return nil, agentInboxOutput{Inbox: page}, err
	}
}

func ackAgentInbox(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, ackAgentInboxInput) (*mcp.CallToolResult, ackAgentInboxOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input ackAgentInboxInput) (*mcp.CallToolResult, ackAgentInboxOutput, error) {
		if len(input.EventIDs) == 0 {
			return nil, ackAgentInboxOutput{}, errors.New("event_ids is required")
		}
		result, err := client.AckAgentInbox(ctx, input.EventIDs)
		return nil, ackAgentInboxOutput{Result: result}, err
	}
}

func getUsage(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, usageOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, usageOutput, error) {
		snap, err := client.Usage(ctx)
		return nil, usageOutput{Usage: snap}, err
	}
}

func getOperationsDashboard(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, operationsOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, operationsOutput, error) {
		dashboard, err := client.OperationsDashboard(ctx)
		return nil, operationsOutput{Dashboard: dashboard}, err
	}
}

func listOperationalCases(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, operationalCasesInput) (*mcp.CallToolResult, operationalCasesOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input operationalCasesInput) (*mcp.CallToolResult, operationalCasesOutput, error) {
		items, err := client.OperationalCases(ctx, input.Status)
		return nil, operationalCasesOutput{Cases: items}, err
	}
}

func getOperationalCase(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, operationalCaseIDInput) (*mcp.CallToolResult, operationalCaseDetailOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input operationalCaseIDInput) (*mcp.CallToolResult, operationalCaseDetailOutput, error) {
		if strings.TrimSpace(input.CaseID) == "" {
			return nil, operationalCaseDetailOutput{}, errors.New("case_id is required")
		}
		item, err := client.OperationalCase(ctx, input.CaseID)
		return nil, operationalCaseDetailOutput{Detail: item}, err
	}
}

func createOperationalCase(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, createOperationalCaseInput) (*mcp.CallToolResult, operationalCaseOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input createOperationalCaseInput) (*mcp.CallToolResult, operationalCaseOutput, error) {
		if strings.TrimSpace(input.Reference) == "" || strings.TrimSpace(input.Title) == "" {
			return nil, operationalCaseOutput{}, errors.New("reference and title are required")
		}
		item, err := client.CreateOperationalCase(ctx, apiclient.CreateOperationalCase{
			Kind: input.Kind, Reference: input.Reference, Title: input.Title, Description: input.Description,
			TripID: input.TripID, FlightNumbers: input.FlightNumbers, Severity: input.Severity, Owner: input.Owner,
			EscalationPolicy: input.EscalationPolicy, SLACurrency: input.SLACurrency,
			SLAValueMinor: input.SLAValueMinor, FailureCostMinor: input.FailureCostMinor,
		})
		return nil, operationalCaseOutput{Case: item}, err
	}
}

func createDecisionRecord(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, createDecisionInput) (*mcp.CallToolResult, decisionOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input createDecisionInput) (*mcp.CallToolResult, decisionOutput, error) {
		var observedAt time.Time
		var err error
		if strings.TrimSpace(input.SignalObservedAt) != "" {
			observedAt, err = time.Parse(time.RFC3339, input.SignalObservedAt)
			if err != nil {
				return nil, decisionOutput{}, errors.New("signal_observed_at must be RFC3339")
			}
		}
		item, err := client.CreateDecisionRecord(ctx, input.CaseID, apiclient.CreateDecisionRecord{
			SignalType: input.SignalType, SignalSource: input.SignalSource, SignalObservedAt: observedAt, Evidence: input.Evidence,
			PredictionType: input.PredictionType, PredictedProbability: input.PredictedProbability, Confidence: input.Confidence,
			HorizonMinutes: input.HorizonMinutes, ModelVersion: input.ModelVersion, ScopeAirport: input.ScopeAirport,
			ScopeAirline: input.ScopeAirline, ScopeRoute: input.ScopeRoute, RecommendationType: input.RecommendationType,
			Recommendation: input.Recommendation, Rationale: input.Rationale, ProposedAction: input.ProposedAction,
			ApprovalRequired: input.ApprovalRequired,
		})
		return nil, decisionOutput{Decision: item}, err
	}
}

func recordDecisionAction(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, decisionActionInput) (*mcp.CallToolResult, decisionOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input decisionActionInput) (*mcp.CallToolResult, decisionOutput, error) {
		item, err := client.RecordDecisionAction(ctx, input.CaseID, input.DecisionID, apiclient.RecordDecisionAction{Decision: input.Decision, ActionTaken: input.ActionTaken, Notes: input.Notes, ExternalRef: input.ExternalRef})
		return nil, decisionOutput{Decision: item}, err
	}
}

func recordDecisionOutcome(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, decisionOutcomeInput) (*mcp.CallToolResult, decisionOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input decisionOutcomeInput) (*mcp.CallToolResult, decisionOutput, error) {
		item, err := client.RecordDecisionOutcome(ctx, input.CaseID, input.DecisionID, apiclient.RecordDecisionOutcome{PredictionResult: input.PredictionResult, ActionResult: input.ActionResult, Actual: input.Actual, AvoidedCostMinor: input.AvoidedCostMinor, Notes: input.Notes})
		return nil, decisionOutput{Decision: item}, err
	}
}

func getDecisionTrust(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, trustOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, trustOutput, error) {
		metrics, err := client.DecisionTrustMetrics(ctx)
		return nil, trustOutput{Trust: metrics}, err
	}
}

// listTrustShares and revokeTrustShare are deliberately the only trust-share
// tools exposed over MCP. Creating a share publishes customer operational
// data to a public, unauthenticated URL — exactly the class of action the
// platform's human-approval gate exists to stop an agent taking
// unsupervised. Revoking is the safe direction, so an agent may do it;
// publishing stays human-only in the web UI. See TestTrustShareToolsAreAsymmetric.
func listTrustShares(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, trustSharesOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, trustSharesOutput, error) {
		items, err := client.TrustShares(ctx)
		return nil, trustSharesOutput{Shares: items}, err
	}
}

func revokeTrustShare(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, revokeTrustShareInput) (*mcp.CallToolResult, revokeTrustShareOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input revokeTrustShareInput) (*mcp.CallToolResult, revokeTrustShareOutput, error) {
		token := strings.TrimSpace(input.Token)
		if token == "" {
			return nil, revokeTrustShareOutput{}, errors.New("token is required")
		}
		if err := client.RevokeTrustShare(ctx, token); err != nil {
			return nil, revokeTrustShareOutput{}, err
		}
		return nil, revokeTrustShareOutput{Revoked: true}, nil
	}
}

func listWebhookIntegrations(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, webhooksOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, webhooksOutput, error) {
		items, err := client.WebhookIntegrations(ctx)
		return nil, webhooksOutput{Integrations: items}, err
	}
}

func createWebhookIntegration(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, createWebhookInput) (*mcp.CallToolResult, webhookCreatedOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input createWebhookInput) (*mcp.CallToolResult, webhookCreatedOutput, error) {
		created, err := client.CreateWebhookIntegration(ctx, apiclient.CreateWebhookIntegration{Name: input.Name, URL: input.URL, Events: input.Events})
		return nil, webhookCreatedOutput{Created: created}, err
	}
}

func testWebhookIntegration(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, webhookIDInput) (*mcp.CallToolResult, webhookDeliveryOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input webhookIDInput) (*mcp.CallToolResult, webhookDeliveryOutput, error) {
		delivery, err := client.TestWebhookIntegration(ctx, input.IntegrationID)
		return nil, webhookDeliveryOutput{Delivery: delivery}, err
	}
}

func askAssistant(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, assistantInput) (*mcp.CallToolResult, assistantOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input assistantInput) (*mcp.CallToolResult, assistantOutput, error) {
		if strings.TrimSpace(input.Question) == "" {
			return nil, assistantOutput{}, errors.New("question is required")
		}
		answer, err := client.Ask(ctx, input.Question, input.TripID)
		return nil, assistantOutput{Answer: answer}, err
	}
}

func tripWhatIf(client *apiclient.Client) func(context.Context, *mcp.CallToolRequest, whatIfInput) (*mcp.CallToolResult, whatIfOutput, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input whatIfInput) (*mcp.CallToolResult, whatIfOutput, error) {
		if strings.TrimSpace(input.TripID) == "" {
			return nil, whatIfOutput{}, errors.New("trip_id is required")
		}
		if input.DelayDepartureMinutes == 0 && input.DelayArrivalMinutes == 0 {
			return nil, whatIfOutput{}, errors.New("delay_departure_minutes and/or delay_arrival_minutes is required")
		}
		result, err := client.WhatIf(ctx, input.TripID, apiclient.WhatIfRequest{
			SegmentIndex:          input.SegmentIndex,
			DelayDepartureMinutes: input.DelayDepartureMinutes,
			DelayArrivalMinutes:   input.DelayArrivalMinutes,
			Question:              input.Question,
		})
		return nil, whatIfOutput{Result: result}, err
	}
}

func value(key, fallback string) string {
	if current := os.Getenv(key); current != "" {
		return current
	}
	return fallback
}
