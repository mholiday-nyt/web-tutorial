package tutor

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RequestURI)

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

func (a *app) withDeadline(next http.Handler) http.Handler {
	return http.TimeoutHandler(next, a.timeout, "Timeout\n")
}

func slow(w http.ResponseWriter, r *http.Request) {
	time.Sleep(6 * time.Second)
	fmt.Fprintln(w, "slow response")
}

func list(w http.ResponseWriter, r *http.Request) {
	key := r.FormValue("key")
	vars := mux.Vars(r)
	item := vars["item"]

	if item == "" {
		http.Error(w, "No item", http.StatusNotFound)
		return
	}

	if key != "" {
		fmt.Fprintln(w, "the item is", item, "and the key is", key)
	} else {
		fmt.Fprintln(w, "the item is", item, "(no key)")
	}
}

func enter(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	item := vars["item"]

	if item == "" {
		http.Error(w, "No item", http.StatusNotFound)
		return
	}

	fmt.Fprintln(w, item, "has been posted")
}

type app struct {
	addr    string
	server  *http.Server
	timeout time.Duration
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

func (a *app) runServer() int {
	router := mux.NewRouter()

	router.Use(logRequest)
	router.Use(basicAuth)
	router.Use(a.withDeadline)

	router.HandleFunc("/db/{item}", list).Methods("GET")
	router.HandleFunc("/db/{item}", enter).Methods("POST")
	router.HandleFunc("/slow", slow).Methods("GET")

	a.server = &http.Server{
		Addr:    a.addr,
		Handler: router,

		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 20 * time.Second,
	}

	return a.serve()
}

func (a *app) fromArgs(args []string) error {
	fl := flag.NewFlagSet("service", flag.ContinueOnError)

	fl.StringVar(&a.addr, "addr", "localhost:8080", "server address")
	fl.DurationVar(&a.timeout, "time", 5*time.Second, "method timeout")

	if err := fl.Parse(args); err != nil {
		return err
	}

	return nil
}

func runApp(args []string) int {
	var a app

	if err := a.fromArgs(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return -2
	}

	return a.runServer()
}
