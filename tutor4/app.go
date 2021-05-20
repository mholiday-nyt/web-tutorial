package tutor4

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

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"

	"tutor4/db"
	"tutor4/graph"
	"tutor4/graph/generated"
)

type app struct {
	router  *mux.Router
	server  *http.Server
	graphql *handler.Server
	db      db.DB
	addr    string
	project string
	data    string
	util    string
	noAuth  bool
	debug   bool
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
	a.db, err = db.NewClient(a.project, a.data, a.util)

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
	r := graph.Resolver{Client: a.db}
	c := generated.Config{Resolvers: &r}
	s := generated.NewExecutableSchema(c)

	a.graphql = handler.NewDefaultServer(s)

	a.router.Use(logRequest)

	if a.noAuth {
		log.Println("AUTH DISABLED")
	} else {
		a.router.Use(basicAuth)
	}

	a.router.Handle("/", playground.Handler("GraphQL playground", "/graphql"))
	a.router.Handle("/graphql", a.graphql)

	a.router.HandleFunc("/items", a.list).Methods("GET")
	a.router.HandleFunc("/items", a.add).Methods("POST")

	a.router.HandleFunc("/items/{id}", a.get).Methods("GET")
	a.router.HandleFunc("/items/{id}", a.put).Methods("PUT")
	a.router.HandleFunc("/items/{id}", a.drop).Methods("DELETE")

	a.router.HandleFunc("/skus", a.listSKU).Methods("GET")

	a.router.HandleFunc("/skus/{sku}", a.getSKU).Methods("GET")
}

func (a *app) fromArgs(args []string) error {
	fl := flag.NewFlagSet("service", flag.ContinueOnError)

	fl.StringVar(&a.addr, "addr", "localhost:8080", "server address")
	fl.StringVar(&a.project, "proj", "tutor-dev", "GCP project")
	fl.StringVar(&a.data, "data", "items", "FS data collection")
	fl.StringVar(&a.util, "util", "util", "FS util collection")

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
