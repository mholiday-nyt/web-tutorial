package tutor2

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

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

type app struct {
	router     *mux.Router
	server     *http.Server
	db         DB
	addr       string
	project    string
	collection string
	noAuth     bool
	debug      bool
}

func (a *app) serve() int {
	done := make(chan os.Signal, 1)

	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	log.Print("server started on ", a.addr)
	<-done
	log.Print("server stopping")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer func() {
		cancel()
		log.Print("server stopped")
	}()

	if err := a.server.Shutdown(ctx); err != nil {
		log.Printf("server shutdown: %s", err)
		return -1
	}

	return 0
}

func (a *app) createClient() (err error) {
	a.db, err = NewClient(a.project, a.collection)

	return
}

func (a *app) makeServer() {
	a.server = &http.Server{
		Addr:    a.addr,
		Handler: a.router,

		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 20 * time.Second,
	}
}

func (a *app) addRoutes() {
	a.router.Use(logRequest)

	if a.noAuth {
		log.Println("AUTH DISABLED")
	} else {
		a.router.Use(basicAuth)
	}

	a.router.HandleFunc("/items", a.list).Methods("GET")
	a.router.HandleFunc("/items", a.add).Methods("POST")

	a.router.HandleFunc("/items/{id}", a.get).Methods("GET")
	a.router.HandleFunc("/items/{id}", a.put).Methods("PUT")
	a.router.HandleFunc("/items/{id}", a.drop).Methods("DELETE")
}

func (a *app) fromArgs(args []string) error {
	fl := flag.NewFlagSet("service", flag.ContinueOnError)

	fl.StringVar(&a.addr, "addr", "localhost:8080", "server address")
	fl.StringVar(&a.project, "proj", "tutor-dev", "GCP project")
	fl.StringVar(&a.collection, "coll", "items", "FS collection")

	fl.BoolVar(&a.debug, "debug", false, "enable debugging")
	fl.BoolVar(&a.noAuth, "no-auth", false, "disable auth")

	if err := fl.Parse(args); err != nil {
		return err
	}

	return nil
}

func (a *app) listRoutes() {
	visit := func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		t, err := route.GetPathTemplate()

		if err != nil {
			return err
		}

		m, err := route.GetMethods()

		if err != nil {
			return err
		}

		log.Println("route", t, m)
		return nil
	}

	if err := a.router.Walk(visit); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func RunApp(args []string) int {
	a := app{router: mux.NewRouter()}

	if err := a.fromArgs(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return -2
	}

	if err := a.createClient(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return -2
	}

	a.makeServer()
	a.addRoutes()

	if a.debug {
		a.listRoutes()
	}

	return a.serve()
}
