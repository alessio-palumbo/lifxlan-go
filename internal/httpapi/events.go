package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	sseHeartbeatPeriod = 15 * time.Second
	sseWriteTimeout    = 10 * time.Second
)

func handleDeviceEvents(w http.ResponseWriter, r *http.Request, service service) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported", "HTTP streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	responseController := http.NewResponseController(w)
	_ = responseController.SetWriteDeadline(time.Time{})

	events := service.SubscribeDevices(r.Context())
	heartbeat := time.NewTicker(sseHeartbeatPeriod)
	defer heartbeat.Stop()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			payload, err := json.Marshal(newDeviceEventResponse(event))
			if err != nil {
				return
			}
			if err := writeSSE(responseController, w, event.Revision, event.Type.String(), payload); err != nil {
				return
			}
		case <-heartbeat.C:
			if err := writeSSEHeartbeat(responseController, w); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

func writeSSE(controller *http.ResponseController, w http.ResponseWriter, revision uint64, eventType string, payload []byte) error {
	_ = controller.SetWriteDeadline(time.Now().Add(sseWriteTimeout))
	if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", revision, eventType, payload); err != nil {
		return err
	}
	if err := controller.Flush(); err != nil {
		return err
	}
	_ = controller.SetWriteDeadline(time.Time{})
	return nil
}

func writeSSEHeartbeat(controller *http.ResponseController, w http.ResponseWriter) error {
	_ = controller.SetWriteDeadline(time.Now().Add(sseWriteTimeout))
	if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
		return err
	}
	if err := controller.Flush(); err != nil {
		return err
	}
	_ = controller.SetWriteDeadline(time.Time{})
	return nil
}
