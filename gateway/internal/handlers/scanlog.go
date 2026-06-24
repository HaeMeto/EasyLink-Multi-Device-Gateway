package handlers

import (
 "net/http"
 "net/url"
 "strconv"
)

func (h *Handler) HandleScanlogNew(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 if sn == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn")
 return
 }
 data, err := h.Queue.Enqueue(sn, "scanlog/new", url.Values{})
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleScanlogAll(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 if sn == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn")
 return
 }
 limit := parseIntParam(r, "limit")
 params := url.Values{}
 if limit > 0 {
 params.Set("limit", strconv.Itoa(limit))
 }
 data, err := h.Queue.Enqueue(sn, "scanlog/all", params)
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleScanlogDelete(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 if sn == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn")
 return
 }
 data, err := h.Queue.Enqueue(sn, "scanlog/del", url.Values{})
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleScanlogGPS(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 if sn == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn")
 return
 }
 byDate := r.URL.Query().Get("by_date")
 params := url.Values{}
 if byDate != "" {
 params.Set("by_date", byDate)
 }
 data, err := h.Queue.Enqueue(sn, "scanlog/gps", params)
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}
