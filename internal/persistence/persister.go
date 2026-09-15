package persistence

import (
	"context"
	"time"

	"github.com/GBoc09/SDCC_project/internal/registry"
	"github.com/GBoc09/SDCC_project/internal/reporting"
)

// Persister salva periodicamente lo stato del registry tramite FileStore.
// Il salvataggio avviene in background, separatamente dalle richieste HTTP.

type Persister struct {
	reporting.ErrorHandler

	fileStore *FileStore
	registry  *registry.Registry
	interval  time.Duration
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

func (p *Persister) save() {
	state := p.registry.Snapshot()

	if err := p.fileStore.Save(state); err != nil {
		p.Report(err)
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
