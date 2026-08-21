package reporting

import "sync"

// ErrorHandler stores and invokes an optional concurrency-safe error callback.
type ErrorHandler struct {
	mu      sync.RWMutex
	handler func(error)
}

// SetErrorHandler replaces the callback used to report background errors.
func (h *ErrorHandler) SetErrorHandler(handler func(error)) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.handler = handler
}

// Report invokes the configured callback, if present.
func (h *ErrorHandler) Report(err error) {
	h.mu.RLock()
	handler := h.handler
	h.mu.RUnlock()

	if handler != nil {
		handler(err)
	}
}
