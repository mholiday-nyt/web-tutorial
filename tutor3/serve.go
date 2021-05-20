package tutor3

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gorilla/mux"
)

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)

		next.ServeHTTP(w, r)
	})
}

func basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()

		if !ok || user != "admin" || pass != "secret" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (a *app) list(w http.ResponseWriter, r *http.Request) {
	items, err := a.db.ListItems(r.Context())

	if err != nil {
		if errors.Is(err, errNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err = json.NewEncoder(w).Encode(items); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
	}
}

func (a *app) listSKU(w http.ResponseWriter, r *http.Request) {
	items, err := a.db.ListSKUs(r.Context())

	if err != nil {
		if errors.Is(err, errNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err = json.NewEncoder(w).Encode(items); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
	}
}

func (a *app) location(url *url.URL, host, id string) string {
	return fmt.Sprintf("http://%s%s/%s", host, url.String(), id)
}

func (a *app) add(w http.ResponseWriter, r *http.Request) {
	var item Item

	err := json.NewDecoder(r.Body).Decode(&item)

	if err != nil || item.Name == "" {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	if item.ID != "" {
		http.Error(w, "Key assigned", http.StatusConflict)
		return
	}

	id, err := a.db.AddItem(r.Context(), &item)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", a.location(r.URL, r.Host, id))
	w.WriteHeader(http.StatusCreated)

	// we're not going to return an error if the encoding
	// fails, because we've already returned Location

	_ = json.NewEncoder(w).Encode(item)
}

func (a *app) get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	item, err := a.db.GetItem(r.Context(), id)

	if err != nil {
		if errors.Is(err, errNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err = json.NewEncoder(w).Encode(item); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
	}
}

func (a *app) getSKU(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	s := vars["sku"]

	sku, err := strconv.Atoi(s)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	item, err := a.db.GetItemBySKU(r.Context(), sku)

	if err != nil {
		if errors.Is(err, errNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err = json.NewEncoder(w).Encode(item); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
	}
}

func (a *app) put(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var item Item

	err := json.NewDecoder(r.Body).Decode(&item)

	if err != nil || item.Name == "" {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	item.ID = id // in case it was left out of the object data

	if err = a.db.UpdateItem(r.Context(), &item); err != nil {
		if errors.Is(err, errNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *app) drop(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := a.db.DeleteItem(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
