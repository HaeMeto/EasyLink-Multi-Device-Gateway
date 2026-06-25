package handlers

import (
	"encoding/json"
	"net/http"
)

type configSetRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (h *Handler) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query("SELECT key, value, updated_at FROM config")
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var configs []map[string]string
	for rows.Next() {
		var key, value, updatedAt string
		if rows.Scan(&key, &value, &updatedAt) == nil {
			configs = append(configs, map[string]string{
				"key":        key,
				"value":      value,
				"updated_at": updatedAt,
			})
		}
	}
	if configs == nil {
		configs = []map[string]string{}
	}
	h.writeJSON(w, http.StatusOK, configs)
}

func (h *Handler) HandlePutConfig(w http.ResponseWriter, r *http.Request) {
	var req configSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Key == "" {
		h.writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	_, err := h.DB.Exec(
		"INSERT INTO config (key, value, updated_at) VALUES (?, ?, datetime('now')) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at",
		req.Key, req.Value,
	)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]string{
		"key":   req.Key,
		"value": req.Value,
		"status": "updated",
	})
}
