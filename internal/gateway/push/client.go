package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Gateway struct {
	endpoint string
	client   *http.Client
}

type StatusError struct {
	Code int
}

func (e StatusError) Error() string {
	return fmt.Sprintf("push ответил кодом %d", e.Code)
}

func New(base string, timeout time.Duration) *Gateway {
	return &Gateway{
		endpoint: strings.TrimRight(base, "/") + "/api/v1/push",
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				ResponseHeaderTimeout: timeout,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (g *Gateway) Close() {
	g.client.CloseIdleConnections()
}

func (g *Gateway) RequestPosition(ctx context.Context, driverID, tripID string) error {
	ctx, span := otel.Tracer("github.com/sonjiwu2/tripgo").Start(ctx, "push REQUEST_POSITION", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	err := g.send(ctx, driverID, tripID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "push не принял запрос")
	}
	return err
}

func (g *Gateway) send(ctx context.Context, driverID, tripID string) error {
	raw, err := json.Marshal(map[string]any{
		"request_id":   uuid.NewString(),
		"recipient_id": driverID,
		"kind":         "REQUEST_POSITION",
		"data":         map[string]string{"trip_id": tripID},
	})
	if err != nil {
		return fmt.Errorf("не получилось собрать запрос в push: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("не получилось собрать запрос в push: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("не достучался до push: %w", err)
	}
	defer resp.Body.Close()
	_, readErr := io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		return StatusError{Code: resp.StatusCode}
	}
	if readErr != nil {
		return fmt.Errorf("не получилось прочитать ответ push: %w", readErr)
	}
	return nil
}
