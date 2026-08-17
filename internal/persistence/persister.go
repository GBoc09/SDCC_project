package persistence

import (
	"context"
	"sync"
	"time"

	"github.com/GBoc09/SDCC_project/internal/registry"
)

type Persister struct {
	fileStore *FileStore
	registry  *registry.Registry
	interval  time.Duration

	errorMu      sync.RWMutex
	errorHandler func(error)
}

func NewPersister(
	fileStore *FileStore,
	store *registry.Registry,
	interval time.Duration,
) *Persister {
	return &Persister{
		fileStore: fileStore,
		registry:  store,
		interval:  interval,
	}
}

func (p *Persister) SetErrorHandler(
	handler func(error),
) {
	p.errorMu.Lock()
	defer p.errorMu.Unlock()

	p.errorHandler = handler
}

func (p *Persister) reportError(err error) {
	p.errorMu.RLock()
	handler := p.errorHandler
	p.errorMu.RUnlock()

	if handler != nil {
		handler(err)
	}
}

func (p *Persister) save() {
	state := p.registry.Snapshot()

	if err := p.fileStore.Save(state); err != nil {
		p.reportError(err)
	}
}

func (p *Persister) Run(ctx context.Context) {
	p.save()

	if p.interval <= 0 {
		<-ctx.Done()
		p.save()
		return
	}

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.save()
			return

		case <-ticker.C:
			p.save()
		}
	}
}
