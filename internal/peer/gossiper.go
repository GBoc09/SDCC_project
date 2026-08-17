package peer

import (
	"context"
	"fmt"
	"sync"

	"github.com/GBoc09/SDCC_project/internal/registry"
)

type Gossiper struct {
	client *Client
	peers  []string

	notifyMu sync.Mutex
	updates  chan registry.RegistryState

	errorMu      sync.RWMutex
	errorHandler func(error)
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

func (g *Gossiper) SetErrorHandler(
	handler func(error),
) {
	g.errorMu.Lock()
	defer g.errorMu.Unlock()

	g.errorHandler = handler
}
func (g *Gossiper) reportError(err error) {
	g.errorMu.RLock()
	handler := g.errorHandler
	g.errorMu.RUnlock()

	if handler != nil {
		handler(err)
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
				g.reportError(err)
			}
		}
	}
}
