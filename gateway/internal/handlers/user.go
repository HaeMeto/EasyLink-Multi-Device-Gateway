package handlers

import (
 "encoding/json"
 "net/http"
 "net/url"
 "strconv"
)

type userSetRequest struct {
 Pin string `json:"pin"`
 Nama string `json:"nama"`
 Pwd string `json:"pwd"`
 RFID string `json:"rfid"`
 Priv string `json:"priv"`
 Tmp string `json:"tmp"`
}

type userSetAllRequest struct {
 Data string `json:"data"`
}

func (h *Handler) HandleUserAll(w http.ResponseWriter, r *http.Request) {
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
 data, err := h.Queue.Enqueue(sn, "user/all", params)
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleUserSet(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 if sn == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn")
 return
 }

 var req userSetRequest
 if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
 h.writeError(w, http.StatusBadRequest, "invalid JSON")
 return
 }

 params := url.Values{
 "sn": {sn},
 "pin": {req.Pin},
 "nama": {req.Nama},
 "pwd": {req.Pwd},
 "rfid": {req.RFID},
 "priv": {req.Priv},
 "tmp": {req.Tmp},
 }

 data, err := h.Queue.Enqueue(sn, "user/set", params)
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleUserSetAll(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 if sn == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn")
 return
 }

 var req userSetAllRequest
 if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
 h.writeError(w, http.StatusBadRequest, "invalid JSON")
 return
 }

 params := url.Values{
 "sn": {sn},
 "data": {req.Data},
 }

 data, err := h.Queue.Enqueue(sn, "user/set-all", params)
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleUserDelete(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 pin := extractPathParam(r, 4)
 if sn == "" || pin == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn or pin")
 return
 }

 params := url.Values{"sn": {sn}, "pin": {pin}}
 data, err := h.Queue.Enqueue(sn, "user/del", params)
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleUserDeleteAll(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 if sn == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn")
 return
 }

 data, err := h.Queue.Enqueue(sn, "user/delall", url.Values{"sn": {sn}})
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}
