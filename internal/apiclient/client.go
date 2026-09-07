package apiclient

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"strings"

	shared "github.com/SkyvisorInsights/skyvisor-go-shared/apiclient"
	"github.com/SkyvisorInsights/skyvisor-go-shared/domain"
)

// Client wraps the shared skyvisor-api HTTP client with a fixed access token
// (MCP stdio servers typically receive one env-supplied token).
type Client struct {
	inner *shared.Client
	token string
}

func New(baseURL, accessToken string) (*Client, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, errors.New("SKYVISOR_OIDC_ACCESS_TOKEN is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" {
		return nil, errors.New("SKYVISOR_API_URL must be an absolute HTTP(S) URL")
	}
	if parsed.Scheme == "http" && !isLoopback(parsed.Hostname()) && !isClusterLocal(parsed.Hostname()) {
		return nil, errors.New("SKYVISOR_API_URL must use HTTPS unless targeting loopback or in-cluster DNS")
	}
	inner, err := shared.New(baseURL)
	if err != nil {
		return nil, err
	}
	inner = inner.WithClientName(domain.ClientHeaderMCP)
	return &Client{inner: inner, token: accessToken}, nil
}

func (c *Client) Flight(ctx context.Context, number string) (domain.Flight, error) {
	return c.inner.GetFlight(ctx, c.token, number)
}

func (c *Client) Trips(ctx context.Context) ([]domain.Trip, error) {
	return c.inner.ListTrips(ctx, c.token)
}

func (c *Client) CreateTrip(ctx context.Context, name string, flights []string) (domain.Trip, error) {
	segments := make([]domain.TripSegment, 0, len(flights))
	for _, f := range flights {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		segments = append(segments, domain.TripSegment{FlightNumber: f})
	}
	return c.inner.CreateTrip(ctx, c.token, domain.CreateTrip{Name: name, Segments: segments})
}

func (c *Client) Watches(ctx context.Context) ([]domain.Watch, error) {
	return c.inner.ListWatches(ctx, c.token)
}

func (c *Client) CreateWatch(ctx context.Context, flightNumber string) (domain.Watch, error) {
	return c.inner.CreateWatch(ctx, c.token, domain.CreateWatch{FlightNumber: flightNumber})
}

// AgentInbox drains pending watch notifications for the account.
func (c *Client) AgentInbox(ctx context.Context, limit int) (domain.AgentInboxPage, error) {
	return c.inner.AgentInbox(ctx, c.token, limit)
}

// AckAgentInbox acknowledges drained notifications.
func (c *Client) AckAgentInbox(ctx context.Context, eventIDs []string) (domain.AgentInboxAck, error) {
	return c.inner.AckAgentInbox(ctx, c.token, domain.AckAgentInbox{EventIDs: eventIDs})
}

func (c *Client) Usage(ctx context.Context) (domain.UsageSnapshot, error) {
	return c.inner.GetUsage(ctx, c.token)
}

func (c *Client) OperationsDashboard(ctx context.Context) (domain.OperationsDashboard, error) {
	return c.inner.OperationsDashboard(ctx, c.token)
}

func (c *Client) OperationalCases(ctx context.Context, status string) ([]domain.OperationalCase, error) {
	return c.inner.ListOperationalCases(ctx, c.token, status)
}

func (c *Client) OperationalCase(ctx context.Context, caseID string) (domain.OperationalCaseDetail, error) {
	return c.inner.GetOperationalCase(ctx, c.token, caseID)
}

func (c *Client) CreateOperationalCase(ctx context.Context, input domain.CreateOperationalCase) (domain.OperationalCase, error) {
	return c.inner.CreateOperationalCase(ctx, c.token, input)
}

func (c *Client) CreateDecisionRecord(ctx context.Context, caseID string, input domain.CreateDecisionRecord) (domain.DecisionRecord, error) {
	return c.inner.CreateDecisionRecord(ctx, c.token, caseID, input)
}

func (c *Client) RecordDecisionAction(ctx context.Context, caseID, decisionID string, input domain.RecordDecisionAction) (domain.DecisionRecord, error) {
	return c.inner.RecordDecisionAction(ctx, c.token, caseID, decisionID, input)
}

func (c *Client) RecordDecisionOutcome(ctx context.Context, caseID, decisionID string, input domain.RecordDecisionOutcome) (domain.DecisionRecord, error) {
	return c.inner.RecordDecisionOutcome(ctx, c.token, caseID, decisionID, input)
}

func (c *Client) DecisionTrustMetrics(ctx context.Context) (domain.DecisionTrustMetrics, error) {
	return c.inner.DecisionTrustMetrics(ctx, c.token)
}

func (c *Client) WebhookIntegrations(ctx context.Context) ([]domain.WebhookIntegration, error) {
	return c.inner.ListWebhookIntegrations(ctx, c.token)
}

func (c *Client) CreateWebhookIntegration(ctx context.Context, input domain.CreateWebhookIntegration) (domain.WebhookIntegrationCreated, error) {
	return c.inner.CreateWebhookIntegration(ctx, c.token, input)
}

func (c *Client) TestWebhookIntegration(ctx context.Context, integrationID string) (domain.WebhookDelivery, error) {
	return c.inner.TestWebhookIntegration(ctx, c.token, integrationID)
}

func (c *Client) TrustShares(ctx context.Context) ([]domain.TrustShareLink, error) {
	return c.inner.TrustShares(ctx, c.token)
}

func (c *Client) RevokeTrustShare(ctx context.Context, token string) error {
	return c.inner.RevokeTrustShare(ctx, c.token, token)
}

func (c *Client) Me(ctx context.Context) (shared.Me, error) {
	return c.inner.Me(ctx, c.token)
}

func (c *Client) Ask(ctx context.Context, question, tripID string) (string, error) {
	resp, err := c.inner.AskAssistant(ctx, c.token, question, tripID)
	if err != nil {
		return "", err
	}
	return resp.Answer, nil
}

func (c *Client) WhatIf(ctx context.Context, tripID string, req domain.WhatIfRequest) (domain.WhatIfResult, error) {
	return c.inner.TripWhatIf(ctx, c.token, tripID, req)
}

func (c *Client) AirportBoard(ctx context.Context, iata, direction, status string, limit int) (domain.AirportBoard, error) {
	return c.inner.AirportBoard(ctx, c.token, iata, shared.AirportBoardQuery{
		Direction: direction,
		Status:    status,
		Limit:     limit,
	})
}

func (c *Client) Analytics(ctx context.Context, airline, airport string, windowDays, limit int) (domain.AnalyticsReport, error) {
	return c.inner.Analytics(ctx, c.token, shared.AnalyticsQuery{
		Airline:    airline,
		Airport:    airport,
		WindowDays: windowDays,
		Limit:      limit,
	})
}

func (c *Client) LogisticsOverview(ctx context.Context, airline, airport string, limit int) (domain.LogisticsOverview, error) {
	return c.inner.LogisticsOverview(ctx, c.token, domain.LogisticsQuery{
		Airline: airline,
		Airport: airport,
		Limit:   limit,
	})
}

// Types re-exported for MCP tool schemas.
type (
	Flight                    = domain.Flight
	Trip                      = domain.Trip
	AirportBoard              = domain.AirportBoard
	AnalyticsReport           = domain.AnalyticsReport
	SituationLayer            = domain.SituationLayer
	SituationNewsPage         = domain.SituationNewsPage
	SituationPoint            = domain.SituationPoint
	WhatIfRequest             = domain.WhatIfRequest
	WhatIfResult              = domain.WhatIfResult
	LogisticsOverview         = domain.LogisticsOverview
	Watch                     = domain.Watch
	UsageSnapshot             = domain.UsageSnapshot
	OperationsDashboard       = domain.OperationsDashboard
	OperationalCase           = domain.OperationalCase
	OperationalCaseDetail     = domain.OperationalCaseDetail
	DecisionRecord            = domain.DecisionRecord
	DecisionTrustMetrics      = domain.DecisionTrustMetrics
	CreateOperationalCase     = domain.CreateOperationalCase
	CreateDecisionRecord      = domain.CreateDecisionRecord
	RecordDecisionAction      = domain.RecordDecisionAction
	RecordDecisionOutcome     = domain.RecordDecisionOutcome
	WebhookIntegration        = domain.WebhookIntegration
	CreateWebhookIntegration  = domain.CreateWebhookIntegration
	WebhookIntegrationCreated = domain.WebhookIntegrationCreated
	WebhookDelivery           = domain.WebhookDelivery
	TrustShareLink            = domain.TrustShareLink
	AgentInboxPage            = domain.AgentInboxPage
	AgentInboxAck             = domain.AgentInboxAck
)

var (
	_ = io.EOF
	_ = shared.Me{}
)

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isClusterLocal(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return strings.HasSuffix(host, ".svc") || strings.HasSuffix(host, ".svc.cluster.local")
}

// SituationLayers returns the global situation layer catalogue.
func (c *Client) SituationLayers(ctx context.Context) ([]domain.SituationLayer, error) {
	return c.inner.SituationLayers(ctx, c.token)
}

// SituationNews returns one page of the situation news rail.
func (c *Client) SituationNews(ctx context.Context, languages, countries, cursor string, limit int) (domain.SituationNewsPage, error) {
	return c.inner.SituationNews(ctx, c.token, shared.SituationNewsQuery{
		Languages: splitCodes(languages),
		Countries: splitCodes(countries),
		Cursor:    strings.TrimSpace(cursor),
		Limit:     limit,
	})
}

// SituationPoint returns current conditions for one place.
func (c *Client) SituationPoint(ctx context.Context, icao string, latitude, longitude float64) (domain.SituationPoint, error) {
	return c.inner.SituationPoint(ctx, c.token, shared.SituationPointQuery{
		ICAO:      strings.TrimSpace(icao),
		Latitude:  latitude,
		Longitude: longitude,
	})
}

// splitCodes reads a comma-separated filter from a tool argument, dropping
// empties so a trailing comma is not sent on as a filter matching nothing.
func splitCodes(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	codes := make([]string, 0, len(parts))
	for _, part := range parts {
		if code := strings.TrimSpace(part); code != "" {
			codes = append(codes, code)
		}
	}
	return codes
}
