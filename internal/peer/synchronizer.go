package peer

import (
	"context"
	"fmt"
	"time"

	"github.com/GBoc09/SDCC_project/internal/registry"
	"github.com/GBoc09/SDCC_project/internal/reporting"
)

// Synchronizer recupera gli snapshot dei peer e li integra nel registry locale.
// La sincronizzazione periodica permette di recuperare aggiornamenti non ricevuti tramite gossip.
type Synchronizer struct {
	reporting.ErrorHandler

	client   *Client
	registry *registry.Registry
	peers    []string
	interval time.Duration
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

// syncAndReport esegue un ciclo di sincronizzazione
func (s *Synchronizer) syncAndReport(
	ctx context.Context,
) {
	syncErrors := s.SyncOnce(ctx)

	for _, err := range syncErrors {
		s.Report(err)
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

// Run esegue una prima sincronizzazione senza attendere l'intervallo periodico
// Dopo il primo ciclo avvia il timer ed esegue i cicli successivi
func (s *Synchronizer) Run(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}

	s.syncAndReport(ctx)

	if s.interval <= 0 {
		return
	}

	// I cicli vengono eseguiti uno alla volta: un ciclo lento può ritardare quelli successivi.
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
