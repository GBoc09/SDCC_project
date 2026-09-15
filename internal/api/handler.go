package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/GBoc09/SDCC_project/internal/registry"
)

// StateNotifier permette agli handler di notificare uno snapshot senza dipendere dall'implementazione del gossip.

type StateNotifier interface {
	Notify(registry.RegistryState)
}

type Handler struct {
	registry *registry.Registry
	notifier StateNotifier
}

func NewHandler(store *registry.Registry) http.Handler {
	return NewHandlerWithNotifier(store, nil)
}

func NewHandlerWithNotifier(
	store *registry.Registry,
	notifier StateNotifier,
) http.Handler {
	handler := &Handler{
		registry: store,
		notifier: notifier,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.health)
	mux.HandleFunc("GET /internal/state", handler.getState)
	mux.HandleFunc("PUT /internal/state", handler.putState)
	mux.HandleFunc(
		"PUT /services/{name}/instances/{id}",
		handler.insertUpdateInstance,
	)
	mux.HandleFunc(
		"DELETE /services/{name}/instances/{id}",
		handler.deleteInstance,
	)
	mux.HandleFunc("GET /services/{name}", handler.discover)
	mux.HandleFunc(
		"DELETE /services/{name}",
		handler.deleteService,
	)

	return mux
}
func (h *Handler) notifyStateChange() {
	if h.notifier == nil {
		return
	}

	h.notifier.Notify(h.registry.Snapshot())
}
func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
func (h *Handler) getState(
	w http.ResponseWriter,
	_ *http.Request,
) {
	state := h.registry.Snapshot()
	writeJSON(w, http.StatusOK, state)
}
func (h *Handler) putState(
	w http.ResponseWriter,
	request *http.Request,
) {
	var state registry.RegistryState

	decoder := json.NewDecoder(
		io.LimitReader(request.Body, 1<<20),
	)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&state); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid registry state",
		)
		return
	}

	if err := ensureSingleJSONValue(decoder); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"request body must contain one JSON object",
		)
		return
	}

	applied := h.registry.MergeState(state)

	writeJSON(
		w,
		http.StatusOK,
		map[string]int{"applied": applied},
	)
}

// Restituisce 201 per una creazione o riattivazione, 200 per un aggiornamento.
func (h *Handler) insertUpdateInstance(w http.ResponseWriter, request *http.Request) {
	var input registry.InstanceInput
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := ensureSingleJSONValue(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "request body must contain one JSON object")
		return
	}

	instance, created, err := h.registry.InsertUpdateInstance(
		request.PathValue("name"),
		request.PathValue("id"),
		input,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, instance)
	h.notifyStateChange()
}

// discover restituisce le istanze attive presenti nello stato locale.
// Un servizio assente o eliminato produce una lista vuota.
func (h *Handler) discover(w http.ResponseWriter, request *http.Request) {
	instances := h.registry.Discover(request.PathValue("name"))
	writeJSON(w, http.StatusOK, instances)
}

// Restituisce 404 se non esiste, 204 anche se era già eliminata
func (h *Handler) deleteInstance(w http.ResponseWriter, request *http.Request) {
	err := h.registry.DeleteInstance(request.PathValue("name"), request.PathValue("id"))
	if errors.Is(err, registry.ErrInstanceNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
	h.notifyStateChange()
}

// Restituisce 404 se il servizio non esiste, altrimenti 204
func (h *Handler) deleteService(w http.ResponseWriter, request *http.Request) {
	err := h.registry.DeleteService(request.PathValue("name"))
	if errors.Is(err, registry.ErrServiceNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
	h.notifyStateChange()
}

func ensureSingleJSONValue(decoder *json.Decoder) error {
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

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
