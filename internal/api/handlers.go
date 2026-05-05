package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jameynakama/APPNAME/internal/store"
)

const defaultLimit = 20

type thingRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *Handler) listThings(w http.ResponseWriter, r *http.Request) {
	limit := int32(defaultLimit)
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = int32(v)
		}
	}

	offset := int32(0)
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = int32(v)
		}
	}

	params := store.ListThingsParams{Limit: limit, Offset: offset}
	res, err := h.queries.ListThings(r.Context(), params)
	if err != nil {
		log.Printf("listThings: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	if res == nil {
		res = []store.Thing{}
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) getThing(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id must be a number")
		return
	}

	res, err := h.queries.GetThing(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		log.Printf("getThing: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) createThing(w http.ResponseWriter, r *http.Request) {
	var req thingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name must be provided")
		return
	}

	params := store.CreateThingParams{Name: req.Name, Description: req.Description}
	t, err := h.queries.CreateThing(r.Context(), params)
	if err != nil {
		log.Printf("createThing: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) updateThing(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id must be a number")
		return
	}

	var req thingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name must be provided")
		return
	}

	params := store.UpdateThingParams{ID: id, Name: req.Name, Description: req.Description}
	t, err := h.queries.UpdateThing(r.Context(), params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		log.Printf("updateThing: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) deleteThing(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id must be a number")
		return
	}

	err = h.queries.DeleteThing(r.Context(), id)
	if err != nil {
		log.Printf("deleteThing: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
