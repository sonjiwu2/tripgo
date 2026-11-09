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
)

type Gateway struct {
	endpoint string
	client   *http.Client
}

func New(base string, timeout time.Duration) *Gateway {
	return &Gateway{
		endpoint: strings.TrimRight(base, "/") + "/api/v1/push",
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				ResponseHeaderTimeout: timeout,
			},
		},
	}
}

func (g *Gateway) Close() {
	g.client.CloseIdleConnections()
}

func (g *Gateway) RequestPosition(ctx context.Context, driverID, tripID string) error {
	ctx, span := otel.Tracer("github.com/sonjiwu2/tripgo").Start(ctx, "push REQUEST_POSITION")
	defer span.End()

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

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("не достучался до push: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("push ответил кодом %d", resp.StatusCode)
	}
	return nil
}
