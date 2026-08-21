package peer

import (
	"context"
	"fmt"
	"sync"

	"github.com/GBoc09/SDCC_project/internal/registry"
	"github.com/GBoc09/SDCC_project/internal/reporting"
)

type Gossiper struct {
	reporting.ErrorHandler

	client *Client
	peers  []string

	notifyMu sync.Mutex
	updates  chan registry.RegistryState
}

func NewGossiper(
	client *Client,
	peers []string,
) *Gossiper {
	return &Gossiper{
		client:  client,
		peers:   append([]string(nil), peers...),
		updates: make(chan registry.RegistryState, 1),
	}
}

func (g *Gossiper) Publish(
	ctx context.Context,
	state registry.RegistryState,
) []error {
	var publishErrors []error

	for _, peerURL := range g.peers {
		_, err := g.client.Push(
			ctx,
			peerURL,
			state,
		)
		if err != nil {
			publishErrors = append(
				publishErrors,
				fmt.Errorf(
					"publish state to peer %q: %w",
					peerURL,
					err,
				),
			)
		}
	}

	return publishErrors
}
func (g *Gossiper) Notify(
	state registry.RegistryState,
) {
	g.notifyMu.Lock()
	defer g.notifyMu.Unlock()

	select {
	case g.updates <- state:
		return
	default:
	}

	select {
	case <-g.updates:
	default:
	}

	select {
	case g.updates <- state:
	default:
	}
}

func (g *Gossiper) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case state := <-g.updates:
			publishErrors := g.Publish(ctx, state)

			for _, err := range publishErrors {
				g.Report(err)
			}
		}
	}
}
