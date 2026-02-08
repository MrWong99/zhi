package webui

import (
	"encoding/json"
	"net/http"
)

// handleSaveTree persists the current tree to the store.
// POST /tree/save
func (s *Server) handleSaveTree(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := s.ctrl.SaveTree(ctx); err != nil {
		notif := notificationEvent{
			Type:    "error",
			Message: "Failed to save: " + err.Error(),
		}
		b, _ := json.Marshal(map[string]any{"showNotification": notif})
		w.Header().Set("HX-Trigger", string(b))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	notif := notificationEvent{
		Type:    "success",
		Message: "Tree saved successfully",
	}
	b, _ := json.Marshal(map[string]any{
		"showNotification": notif,
		"markSaved":        true,
	})
	w.Header().Set("HX-Trigger", string(b))
	w.WriteHeader(http.StatusOK)
}

// notificationEvent is the data sent via HX-Trigger for toast notifications.
type notificationEvent struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
