package peer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/GBoc09/SDCC_project/internal/registry"
)

const maxStateSize = 1 << 20

type Client struct {
	httpClient *http.Client
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Acquisizione snapshot tramite GET
func (c *Client) Sync(
	ctx context.Context,
	peerURL string,
	target *registry.Registry,
) (int, error) {
	stateURL := strings.TrimRight(peerURL, "/") +
		"/internal/state"

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		stateURL,
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("create state request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, fmt.Errorf("request peer state: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf(
			"peer returned status %d",
			response.StatusCode,
		)
	}

	var state registry.RegistryState

	decoder := json.NewDecoder(
		io.LimitReader(response.Body, maxStateSize),
	)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&state); err != nil {
		return 0, fmt.Errorf("decode peer state: %w", err)
	}

	if err := ensureEndOfJSON(decoder); err != nil {
		return 0, fmt.Errorf("decode peer state: %w", err)
	}

	return target.MergeState(state), nil
}

// Invia lo snapshot locale tramite PUT
func (c *Client) Push(
	ctx context.Context,
	peerURL string,
	state registry.RegistryState,
) (int, error) {
	body, err := json.Marshal(state)
	if err != nil {
		return 0, fmt.Errorf(
			"encode registry state: %w",
			err,
		)
	}

	stateURL := strings.TrimRight(peerURL, "/") +
		"/internal/state"

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		stateURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return 0, fmt.Errorf(
			"create state request: %w",
			err,
		)
	}

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, fmt.Errorf(
			"push peer state: %w",
			err,
		)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf(
			"peer returned status %d",
			response.StatusCode,
		)
	}

	var result struct {
		Applied int `json:"applied"`
	}

	decoder := json.NewDecoder(
		io.LimitReader(response.Body, maxStateSize),
	)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&result); err != nil {
		return 0, fmt.Errorf(
			"decode peer response: %w",
			err,
		)
	}

	if err := ensureEndOfJSON(decoder); err != nil {
		return 0, fmt.Errorf(
			"decode peer response: %w",
			err,
		)
	}

	return result.Applied, nil
}

func ensureEndOfJSON(decoder *json.Decoder) error {
	var extra any

	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values")
	}

	return err
}
