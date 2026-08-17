package peer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GBoc09/SDCC_project/internal/registry"
)

type Synchronizer struct {
	client   *Client
	registry *registry.Registry
	peers    []string
	interval time.Duration

	errorMu      sync.RWMutex
	errorHandler func(error)
}

func NewSynchronizer(
	client *Client,
	store *registry.Registry,
	peers []string,
	interval time.Duration,
) *Synchronizer {
	return &Synchronizer{
		client:   client,
		registry: store,
		peers:    append([]string(nil), peers...),
		interval: interval,
	}
}
func (s *Synchronizer) SetErrorHandler(
	handler func(error),
) {
	s.errorMu.Lock()
	defer s.errorMu.Unlock()

	s.errorHandler = handler
}

func (s *Synchronizer) reportError(err error) {
	s.errorMu.RLock()
	handler := s.errorHandler
	s.errorMu.RUnlock()

	if handler != nil {
		handler(err)
	}
}

func (s *Synchronizer) syncAndReport(
	ctx context.Context,
) {
	syncErrors := s.SyncOnce(ctx)

	for _, err := range syncErrors {
		s.reportError(err)
	}
}
func (s *Synchronizer) SyncOnce(
	ctx context.Context,
) []error {
	var syncErrors []error

	for _, peerURL := range s.peers {
		_, err := s.client.Sync(
			ctx,
			peerURL,
			s.registry,
		)
		if err != nil {
			syncErrors = append(
				syncErrors,
				fmt.Errorf(
					"synchronize with peer %q: %w",
					peerURL,
					err,
				),
			)
		}
	}

	return syncErrors
}
func (s *Synchronizer) Run(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}

	s.syncAndReport(ctx)

	if s.interval <= 0 {
		return
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			s.syncAndReport(ctx)
		}
	}
}
