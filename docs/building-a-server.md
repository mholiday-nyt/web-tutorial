# Building a web server

Let's start with the simplest possible web server in Go:

```go
package main

import (
	"fmt"
	"log"
	"net/http"
)

func hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello, web!")
}

func main() {
	http.HandleFunc("/", hello)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

The call to `http.ListenAndServe()` starts a server on the indicated address; it normally does not return unless an error occurs, which we'll log.

We must indicate how the server should respond to a particular _route_, which is the part of the URL after the hostname. Here we assign a single route `/` (which is basically any and every query) using a handler `hello()`.

Routes are managed by a _router_ which here is built into the standard library's HTTP package.

Every handler takes two parameters:

- a `ResponseWriter` which is used to send the response
- the incoming `Request` send by the client/browser

Note that the handler doesn't return anything; any error must be indicated by sending a particular HTTP status and/or message through the `ResponseWriter`.

We can exercise this server using [`curl`](https://curl.haxx.se/book.html):

```
$ curl http://localhost:8080
Hello, web!
```

We'll be using `curl` as well as [`jq`](https://stedolan.github.io/jq/) throughout this tutorial.

## Routes, middleware, and building out the app

First, create a project for this part of the tutorial:

```
$ mkdir tutor
$ cd tutor
$ go mod init tutor
```

### Routing

We're going to introduce a very important 3rd-party library right off, [Gorilla mux](https://github.com/gorilla/mux). Gorilla mux will replace the standard router and make a lot of things easier.

For example, with the standard library, if we want to allow a route to be accessed only with the GET HTTP method, we'd have to write something like this:

```go
func hello(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http. StatusMethodNotAllowed)
		return
	}

	fmt.Fprintln(w, "Hello, web!")
}
```

**Note**: any time we set an error, we must remember to return from the function early, and avoid sending non-error responses by accident.

Now, using Gorilla mux, we can let the router solve that problem for us:

```go
func main() {
	router := mux.NewRouter()

	router.HandleFunc("/", hello)
	log.Fatal(http.ListenAndServe(":8080", router))
}
```

In this case, we create a router, add routes to it, and pass that router to the `ListenAndServe` call.

This code allows any HTTP method to access `/`, but a one-line change can fix that:

```go
	router.HandleFunc("/", hello).Methods(http.MethodGet)
```

In this case, only the GET method can access `/`; other methods will get a 404 response.

An HTTP router works by pattern matching: it takes into account both the route, e.g., `/` or `/list` as well as the method(s). The same route may be assigned two different handlers if their allowed methods differ:

```go
	router.HandleFunc("/", hello).Methods(http.MethodGet)
	router.HandleFunc("/", goodbye).Methods(http.MethodPost)
```

Also, routes are evaluated in the order they're installed into the router. Gorilla mux allows variable matching in a route

```go
	router.HandleFunc("/list/{item}", list)
```

which we'll see more in a bit. But we can match a fixed value before the variable:

```go
	router.HandleFunc("/list/all", listAll)
	router.HandleFunc("/list/{item}", list)
```
If these two lines were reversed, "all" would always be picked up by the variable match and `listAll()` wouldn't be called.

```go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func list(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	item := vars["item"]

	// we don't need this code; it the variable isn't
	// provided (e.g., /db/) it won't match the route
	//
	// if item == "" {
	//	    http.Error(w, "No item", http.StatusNotFound)
	//	    return
	// }

	fmt.Fprintln(w, "the item is", item)
}

func main() {
	router := mux.NewRouter()

	router.HandleFunc("/db/{item}", list)
	log.Fatal(http.ListenAndServe(":8080", router))
}
```

Let's try this:

```
$ curl http://localhost:8080/db/a
the item is a
$ curl http://localhost:8080/db/
404 page not found
```

where the 404 error message is returned from the router on our behalf.

We'll now use separate methods for the route:

```go
func enter(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	item := vars["item"]

	fmt.Fprintln(w, item, "has been posted")
}

func main() {
	router := mux.NewRouter()

	router.HandleFunc("/db/{item}", list).Methods(http.MethodGet)
	router.HandleFunc("/db/{item}", enter).Methods(http.MethodPost)

	log.Fatal(http.ListenAndServe(":8080", router))
}
```

with the results

```
$ curl http://localhost:8080/db/1
the item is 1
$ curl http://localhost:8080/db/1 -X POST
1 has been posted
```

We can also use URL arguments, e.g, `/db/XYZ?key=12` by reading a form value:

```go
func list(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	item := vars["item"]
	key := r.FormValue("key")

	if key != "" {
		fmt.Fprintln(w, "the item is", item, "and the key is", key)
	} else {
		fmt.Fprintln(w, "the item is", item, "(no key)")
	}
}
```

which we can test

```
$ curl http://localhost:8080/db/XYZ
the item is XYZ (no key)
$ curl http://localhost:8080/db/XYZ?key=1
the item is XYZ and the key is 1
```

### Middleware

_Middleware_ in the context of a web server is any function that's put between the router and the handlers. Any such function gets called on every request and gets a chance to modify how it's handled.

For example, we can log every request by adding logging middleware:

```go
func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

func main() {
	router := mux.NewRouter()

	router.Use(logRequest)  // installs middleware

	router.HandleFunc("/db/{item}", list).Methods(http.MethodGet)
	router.HandleFunc("/db/{item}", enter).Methods(http.MethodPost)

	log.Fatal(http.ListenAndServe(":8080", router))
}
```

There are some important details here:

- the middleware function is a handler, taking the same request and response parameters
- the middleware function is also a closure taking a "next" handler
- the middleware function does it's work before or after calling that "next" handler

If the next handler isn't called, the request won't be handled any further, which could be useful it the middleware is used to handle authentication.

Now it's worth pointing out that a "handler" in Go is really something that implements an interface:

```go
type Handler interface {
	ServeHTTP(ResponseWriter, *Request)
}

type HandlerFunc func(ResponseWriter, *Request)

func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
	f(w, r)
}
```

where the `HandlerFunc` type is just a way to take a regular function and allow that function to be called through the `ServeHTTP` method.

Anyway, the result of installing the `logRequest()` middleware is that we now see logs where we run the server:

```
$ go run main.go
2020/09/27 20:23:14 GET /db/XYZ?key=1
2020/09/27 20:23:19 GET /db/a
. . .
```

HTTP has a very simple name and password authentication method (that's not very secure) called [basic authentication](https://en.wikipedia.org/wiki/Basic_access_authentication). If we enable it, every request will need to have a special header with the name and password combined using base64 encoding.

The header looks like `Authorization: Basic YWRtaW46c2VjcmV0`. There's no cryptography involved, so in real life HTTPS is required to have any security with this method at all. It's trivial to "decode" this:

```
$ echo -n YWRtaW46c2VjcmV0 | base64 -D
admin:secret
```

We'll add another bit of middleware to check for basic auth:

```go
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
```

Now the `Request` object has a method that allows us to get the name and password (or tell that they weren't there). Also note that if the name and password are wrong, the handler stops the request and returns an error.

Now we'll need to have curl pass the secret handshake, or else:

```
$ curl http://localhost:8080/db/a -w "%{http_code}\n"
Unauthorized
401
$ curl http://localhost:8080/db/a -w "%{http_code}\n" --user admin:secret
the item is a (no key)
200
```

See how we've provided the `-w` argument to curl to print the HTTP status code. Alternatively, we could see all the response headers with

```
$ curl http://localhost:8080/db/a -i
HTTP/1.1 401 Unauthorized
Content-Type: text/plain; charset=utf-8
Www-Authenticate: Basic realm="Restricted"
X-Content-Type-Options: nosniff
Date: Mon, 28 Sep 2020 02:33:08 GMT
Content-Length: 13

Unauthorized
```

but that's not always as convenient.

### Improving the app structure

First, let's point out a pair of articles that are guiding us:

- [_How I Write HTTP Services After Eight Years_](https://pace.dev/blog/2018/05/09/how-I-write-http-services-after-eight-years.html) by Mat Ryer (which was also a talk at [GopherCon 2019](https://www.youtube.com/watch?v=rWBSMsLG8po))

- [_Writing Go CLIs With Just Enough Architecture_](https://blog.carlmjohnson.net/post/2020/go-cli-how-to-and-advice/) by Carl Johnson

The first discusses some thoughts about structuring handlers and middleware and how to make the web server testable.

The second is about how to structure the application that starts web service (the part that, for example, takes command-line arguments). Proper structure also helps make the server testable.

What we want is to build a `main()` function that does one thing:

```go
func main() {
	os.Exit(runApp(os.Args[1:]))
}
```

This is very reminiscent of the Python style of main program:

```python
if __name__ == "__main__":
    main()
```

which allows the real "main" function to be called from anywhere, for example, from a unit test.

We'd like to be able to write a test like this one:

```go
func TestServer(t *testing.T) {
	args := []string{"-addr", "localhost:8089"}

	go runApp(args)

	client := http.Client{}
	req, _ := http.NewRequest("GET", "http://localhost:8089/db/xyz", nil)

	req.SetBasicAuth("admin", "secret")

	resp, err := client.Do(req)

	if err != nil {
		t.Fatal(err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		t.Fatal(err)
	}

	result := strings.TrimSpace(string(body))

	if result != "the item is xyz (no key)" {
		t.Errorf("invalid response: %v", body)
	}
}
```

Here we run the "main" function `runApp()` with some parameters we pass in (without using the command line); it's in a goroutine so we can continue on with the client code to test the server.

We create an HTTP client instead of using `http.Get()` because we need to create a separate HTTP request in order to add the basic auth header. We get the response, read the entire body at once and make it into a string that we can compare to.

Note that we're fully starting up the server which will register itself on a network port we've picked. There's a risk to this; the test will fail if some other process on our machine is using port 8089 at the same time. Later we'll see a way to avoid this.

The next step is to create `runApp()`. For now, we'll do something like this:

```go
func runApp(args []string) int {
	var a app

	if err := a.fromArgs(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return -2
	}

	return a.runServer()
}
```

Now, most of our code will be methods on `app` such as `runServer()`, so we need to create it:

```go
type app struct {
	addr    string
	server  *http.Server
}
```

The `app` class is a great place to put all our dependencies as well as any parameters that we'll set up with flags. Let's go ahead and process those flags:

```go
func (a *app) fromArgs(args []string) error {
	fl := flag.NewFlagSet("service", flag.ContinueOnError)

	fl.StringVar(&a.addr, "addr", "localhost:8080", "server address")

	if err := fl.Parse(args); err != nil {
		return err
	}

	return nil
}
```

Later we'll see how to use a 3rd-party library to allow configuration by environment variables or flags. Using environment variables is part of building a good ["twelve-factor app"](https://12factor.net) (see also this [Wikipedia entry](https://en.wikipedia.org/wiki/Twelve-Factor_App_methodology)). For now, we'll use flags to make it easy to run the tutorial on the desktop.

Most of what went into `main()` before will now appear in `runServer()`.

```go
func (a *app) runServer() int {
	router := mux.NewRouter()

	router.Use(logRequest)
	router.Use(basicAuth)

	router.HandleFunc("/db/{item}", list).Methods(http.MethodGet)
	router.HandleFunc("/db/{item}", enter).Methods(http.MethodPost)

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
```

Note that we're also now following a best practice to set timeouts for the server as a whole.

Finally, we can start with a basic service function:

```go
func (a *app) serve() int {
	if err := a.server.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "listen: %s\n", err)
		return -1
	}
}
```

However, we can build a better function that allows the server to shut down gracefully (e.g., on receiving an interrupt). Here we'll also run the server in a goroutine while waiting on a channel to handle the signal. We allow 5 seconds for existing requests to finish.

```go
func (a *app) serve() int {
	done := make(chan os.Signal, 1)

	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		err := a.server.ListenAndServe()

		if err != nil && err != http.ErrServerClosed {
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
```

So now, the server output looks like:

```
2020/09/27 21:26:10 server started on localhost:8080
2020/09/27 21:26:16 GET /db/a
2020/09/27 21:26:25 GET /db/XYZ?key=1
. . .
2020/09/27 21:26:38 server stopping
2020/09/27 21:26:38 server stopped
```

assuming we did a `kill` on the server's process ID (PID), e.g.

```
$ ps -ef|grep go
  501 77510 35842   0  9:26PM ttys001    0:00.00 grep go
  501 77490 35950   0  9:26PM ttys002    0:00.38 go run main.go
  501 77506 77490   0  9:26PM ttys002    0:00.01 /var/folders/18/7d1ff7d550scc5v76sj1y0gh0000gn/T/go-build651932460/b001/exe/main
$ kill 77506
```

where we want to kill the _child_ of `go run` which is the actual server program.

Before we move on, there's one last thing we want to show, which is using middleware to provide a timeout on handlers.

Let's define a slow handler, e.g.

```go
func slow(w http.ResponseWriter, r *http.Request) {
	time.Sleep(6 * time.Second)
	fmt.Fprintln(w, "slow response")
}
```

and some middleware using another Go utility

```go
func (a *app) withDeadline(next http.Handler) http.Handler {
	return http.TimeoutHandler(next, a.timeout, "Timeout\n")
}
```

assuming we've added another parameter to the app

```go
type app struct {
	addr    string
	server  *http.Server
	timeout time.Duration
}
```

and in our startup, we're using that middleware

```go
	router.Use(a.withDeadline)
	router.HandleFunc("/slow", slow)
```

Now, if we try that slow route, we'll get an error

```
$ curl -s http://localhost:8080/slow -i --user admin:secret
HTTP/1.1 503 Service Unavailable
Date: Mon, 28 Sep 2020 03:31:17 GMT
Content-Length: 8
Content-Type: text/plain; charset=utf-8

Timeout
```

Note that we could also choose to apply the middleware only to a specific handler instead of the whole server:

```go
	router.Handle("/slow", a.withDeadline(http.HandlerFunc(slow)))
```

## Using Firestore

In this next section, we'll add support for reading and writing to Google's [Firestore](https://cloud.google.com/firestore) database. We'll also talk about how to build a "real" REST server.

### Installing the emulator
For the rest of this tutorial we'll be using Google's Firestore emulator and not the real database in the cloud. You still must have a Google account to use it, because it relies on the `gcloud` tool.

You’ll need Java 8+ in order to run Google’s Firestore emulator on your laptop.

Go to [https://www.oracle.com/java/technologies/javase/javase-jdk8-downloads.html](https://www.oracle.com/java/technologies/javase/javase-jdk8-downloads.html) and download the file `jdk-8u251-macosx-x64.dmg`. (You must create an account to do this.) Open the disk image and copy the installer to your actual disk so you can "fix" it for Catalina.

Once the installer is copied, run

```
$ xattr -d com.apple.quarantine <path/to/installer>
```

and then you should be able to run the installer. When complete, you should be able to check your work:

```
$ java -version
java version "1.8.0_251"
Java(TM) SE Runtime Environment (build 1.8.0_251-b08)
Java HotSpot(TM) 64-Bit Server VM (build 25.251-b08, mixed mode)
```

In order to run the emulator, Google will also want to download the "beta" command set as well as the emulator. Use

```
$ gcloud beta emulators firestore start --host-port localhost:8086
```

after which you’ll be prompted twice, once for the beta commands, and once for the emulator.

When the emulator runs, you'll see output like this

```
$ gcloud beta emulators firestore start --host-port=localhost:8086
Executing: /usr/local/Caskroom/google-cloud-sdk/latest/google-cloud-sdk/platform/cloud-firestore-emulator/cloud_firestore_emulator start --host=localhost --port=8086
[firestore] API endpoint: http://localhost:8086
[firestore] If you are using a library that supports the FIRESTORE_EMULATOR_HOST environment variable, run:
[firestore]
[firestore]    export FIRESTORE_EMULATOR_HOST=localhost:8086
[firestore]
[firestore] Dev App Server is now running.
[firestore]
. . .
```

In the shell where you'll run the server, you must do

```
$ export FIRESTORE_EMULATOR_HOST=localhost:8086
```

and then Google's Firestore library for Go will "magically" find the emulator and not try to contact a real database in the cloud. Later, we'll see how to make the emulator run as part of unit tests.

### Reading and writing
In this first section, we'll build out just enough to create and list some type of item in our database. To do that, we'll need to set up a DB client and make it available to our server.

We're going to define an interface for all our database functions so that later we can mock the entire database. For now, the Item we'll manage is going to be very simple: a name and some unique ID (a UUID).

```go
type Item struct {
	ID   string `json:"id" firestore:"id"`
	Name string `json:"name" firestore:"name"`
}

type DB interface {
	AddItem(context.Context, *Item) (string, error)
	GetItem(context.Context, string) (*Item, error)
	ListItems(context.Context) ([]*Item, error)
	UpdateItem(context.Context, *Item) error
	DeleteItem(context.Context, string) error
}
```

Note that we add struct tags for Firestore so that the database uses the same JSON keys as we use for actual JSON encoding.

Next, we're going to create the client we'll use to access the DB. Firestore is a document-based database with _collections_ as a tool to group like kinds of documents. Firestore's documents can be thought of as JSON objects; the properties are the fields of whatever struct (record) we're storing. Firestore is a schema-less database: there is no explicit schema for a collection, but rather documents in a collection need not have all the same properties. Schema-less is flexible but not always safe (in the sense of maintaining data integrity).

To access a database, we must name a GCP project (although with the emulator, it can be anything, and need not match any actual project in the cloud). We will also capture the collection name in which we'll store our items.

```go
import (
	"context"
	"errors"
	"fmt"
	"log"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct {
	fs   *firestore.Client
	data *firestore.CollectionRef
}
```
```go
func NewClient(project, collection string) (*Client, error) {
	if project == "" {
		return nil, errors.New("no projectID")
	}

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, project)

	if err != nil {
		return nil, fmt.Errorf("failed to create FS client: %w", err)
	}

	c := Client{
		fs:   client,
		data: client.Collection(collection),
	}

	return &c, nil
}

func (c *Client) Close() {
	c.fs.Close()
}
```

Once we've run `NewClient()` we're ready to access the database and the documents in our collection. Note that we don't "create" the collection; it's created implicitly when we store the first item. (Operations that actually communicate to the database all take a `Context` parameter; thus, "making" the collection object we add to our client object is not such an operation.)

We'll start just creating items and listing them. Feel free to remove some of the `DB` interface methods we won't implement just yet; we'll add them back in a bit.

First, we need to create items:

```go
func (c *Client) AddItem(ctx context.Context, i *Item) (string, error) {
	id := uuid.New().String()
	ref := c.data.Doc(id)

	i.ID = id // store it, so the caller can see it here too

	if _, err := ref.Create(ctx, i); err != nil {
		return "", err
	}

	return id, nil
}
```

Firestore requires the document's "ID" (which doesn't have to be a field, or be named "ID") to be unique. We create a UUID and use that to make a _document reference_ against our collection, and the use that reference to create the actual document, with the contents of our item. Firestore will use its own serialization of the object to send it to the database.

By using `Create()` we tell Firestore that we're making a new document; it's then an error if the ID (the UUID, here) already exists in that collection. (Later we'll use `Set()` to perform updates.) We could try again if we get a UUID we've already seen, with a simple `goto`:

```go
func (c *Client) AddItem(ctx context.Context, i *Item) (string, error) {
	var ref *firestore.DocumentRef

add:
	i.ID = uuid.New().String()
	ref = c.data.Doc(i.ID)

	if _, err := ref.Create(ctx, i); err != nil {
		// it's unlikely to happen once and virtually
		// impossible for it to happen twice in a row

		if status.Code(err) == codes.AlreadyExists {
			goto add
		}

		return "", err
	}

	return i.ID, nil
}
```

Again, an operation that communicates with the database takes a `Context` and returns a possible error; "making" the document ref is local to our app just as making the UUID value.

To list all the objects, we'll execute a Firestore query on the collection, asking it to order all documents by their ID. (Only when we fetch the documents is the query actually sent to the database.) Note that the query may return no documents without signaling an error.

```go
func (c *Client) ListItems(ctx context.Context) ([]*Item, error) {
	query := c.data.OrderBy(firestore.DocumentID, firestore.Asc)
	docs, err := query.Documents(ctx).GetAll()

	if err != nil {
		return nil, err
	}

	result := make([]*model.Item, 0, len(docs))

	for _, doc := range docs {
		var i Item

		if err = doc.DataTo(&i); err != nil {
			log.Printf("item %s decode: %s", doc.Ref.ID, err)
			continue
		}

		result = append(result, &i)
	}

	return result, nil
}
```

The "document" returned by the query hasn't yet been deserialized; that what the call to `DataTo()` is doing.

### Making handlers to create and list
Now that we've got a couple of database operations, let's add that functionality into our server. We're going to dispense with authentication for the rest of the tutorial for convenience, and remove the "slow" route we used to demonstrate timeouts.

We're also going to refactor up how we configure the app so that we can support another style of unit tests at the end of this section.

Let's start with the `list()` handler. But first, we're going to need to create the client on the app.


```go

type app struct {
	router     *mux.Router
	server     *http.Server
	db         DB
	addr       string
	project    string
	collection string
}
```

And then we'll need to set up the parameters

```go
func (a *app) fromArgs(args []string) error {
	fl := flag.NewFlagSet("service", flag.ContinueOnError)

	fl.StringVar(&a.addr, "addr", "localhost:8080", "server address")
	fl.StringVar(&a.project, "proj", "tutor-dev", "GCP project")
	fl.StringVar(&a.collection, "coll", "items", "FS collection")

	if err := fl.Parse(args); err != nil {
		return err
	}

	return nil
}
```

and create and install the client into the app

```go
func (a *app) createClient() (err error) {
	a.db, err = NewClient(a.project, a.collection)

	return
}
```
```go
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

	. . .
}
```

Now that we've done that, we can make the "list" handler a method and get access to dependencies such as the DB client through the app. So we'll right something like

```go
func (a *app) list(w http.ResponseWriter, r *http.Request) {
	. . .
}
```

and install it as

```go
	a.router.HandleFunc("/list", a.list)
```

Here `a.list` is a _method value_ (or _method closure_) which is the method with a receiver object already selected. As a result, it now has the function signature of a `HandlerFunc`, because `a` is already bound.

It's very important that `app.list()` take a **pointer** receiver. A method value that takes a pointer receiver keeps the address of the object, much like a normal closure: it will access the current value in memory. By contrast, if we had defined `app.list()` to take `a` by value, the method value would contain a **copy** of `a` (made when we created the server!) which might be bad.

Let's now fill in the body:

```go
func (a *app) list(w http.ResponseWriter, r *http.Request) {
	items, err := a.db.ListItems(r.Context())

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err = json.NewEncoder(w).Encode(items); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
}
```

All our handlers are going to be reading and writing JSON in the HTTP body; it's how we pass in data or return it to the client. In this case, if the list is empty, we'll return an empty JSON array `[]`.

Note that we create the JSON encoder on the `ResponseWriter` directly. Also note the "extra" return if JSON encoding fails. It's too easy to add more code to the handler later at the bottom of the method, and forget to come back and add a return to the error-handling block; just do it now.

To create objects, we'll add a `create` handler. It will decode JSON from the body of the request and use that to pass an item to the DB client's `AddItem()` method.

```go
func (a *app) add(w http.ResponseWriter, r *http.Request) {
	var item Item

	err := json.NewDecoder(r.Body).Decode(&item)

	if err != nil || item.Name == "" {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	id, err := a.db.AddItem(r.Context(), &item)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err = json.NewEncoder(w).Encode(item); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
}
```

We must also add these routes into our router:

```go
	a.router.HandleFunc("/list", a.list).Methods(http.MethodGet)
	a.router.HandleFunc("/create", a.add).Methods(http.MethodPost)
```

So now we're in a position to give it a test. We'll create a couple of items, and then list them out again. Note how we send data through `curl`; don't forget the single quotes around JSON on the command line. (To be completely correct, we should be setting `-H 'Content-Type: application/json'` but that's not critical here).

```
$ curl http://localhost:8080/create -X POST --data '{"name":"spoon"}'
{"id":"b34825af-bc2f-4ebb-84f8-0a1d082f9823","name":"spoon"}
$ curl http://localhost:8080/create -X POST --data '{"name":"spork"}'
{"id":"92d4a080-348e-4ef7-8ec3-80ba008ccd68","name":"spork"}
```
```js
$ curl -s http://localhost:8080/list | jq
[
  {
    "id": "92d4a080-348e-4ef7-8ec3-80ba008ccd68",
    "name": "spork"
  },
  {
    "id": "b34825af-bc2f-4ebb-84f8-0a1d082f9823",
    "name": "spoon"
  }
]
```

When listing, we pass the data to `jq` to pretty-print it (we could also use `jq` for various filtering or transforming operations). We pass the `-s` option to curl so that it doesn't print stats to `stderr` while piping the data to `jq`.

### REST for real
[REST](https://en.wikipedia.org/wiki/Representational_state_transfer) (Representational State Transfer) has a very specific meaning; it's not just handling any URLs from non-browser applications. For example, many people think the type of interface we've shown above (with `/list` and `/create` routes) is RESTful, but it's really not.

(By the way, Thomas Fielding's [PhD thesis](https://www.ics.uci.edu/~fielding/pubs/dissertation/fielding_dissertation.pdf), in which he invented REST, is one of a very few theses that have really changed the world.)

In REST, URLs are nouns and HTTP methods are the (only) verbs. That is, we use the URL to identify an object and the HTTP method to identify what we're going to do with it.

Pure REST is often tied to the acronym CRUD, which stands for Create, Retrieve, Update, and Delete, the four basic operations we can perform on database entries. REST is essentially stateless, and thus it's best suited for database operations.

GET is expected to return data without making any changes, thus, it is _idempotent_ (it can be run one or many times with the same result). POST creates data, PUT changes it, and DELETE removes it. DELETE should also be idempotent, but POST and PUT typically aren't.

Here's how they map:

|Method |Collection: /items  |Item:  /items/{id}           |
|:------|:-------------------|:----------------------------|
|GET    | retrieve all       | retrieve (200)              |
|POST   | create (201) or 409 (if ID exists) |             |
|PUT    |                    | update (200)                |
|DELETE |                    | delete (200)                |

where the item-based GET and PUT return 404 if the ID is not known, while POST returns 409 if it is already known. Ideally, there's no reason for DELETE to fail; if the ID isn't in the database, it still won't be (i.e., just like deleting keys from a Go map).

The main limitation of REST is that sometimes we don't just want to update data, but we want to make things happen. In a REST model, we can only do that by issuing PUT requests with data changes that imply a state change (think of setting a light's state to "on" or "off"). That doesn't work as well if we want our server to convert sound files on the fly, as opposed to store information about what CDs are in our library.

### Adding more DB methods
Before we can perform all these operations, we're going to need some additional DB client methods. Let's start with getting an item by its ID.

We again get a document reference on the collection and immediately get its data. We're going to add some special error-handling logic: if Firestore returns a specific status code, it means the document didn't exist, and we'll create a Go 1.13-style wrapped error to return (we'll see how to use this in just a bit). Otherwise, we deserialize the item and return it.

```go
var ErrNotFound = errors.New("not found")

func (c *Client) GetItem(ctx context.Context, id string) (*Item, error) {
	doc, err := c.data.Doc(id).Get(ctx)

	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, fmt.Errorf("item %s: %w", id, ErrNotFound)
		}

		return nil, fmt.Errorf("item %s: %w", id, err)
	}

	var i Item

	if err = doc.DataTo(&i); err != nil {
		return nil, fmt.Errorf("item %s decode: %w", id, err)
	}

	return &i, nil
}
```

Next, let's update an item. We need to try to get it to see if it actually exists in the collection, otherwise we have an error.

```go
func (c *Client) UpdateItem(ctx context.Context, i *Item) error {
	ref := c.data.Doc(i.ID)

	// set can create or overwrite existing data
	// so we need to see if it exists first

	if _, err := ref.Get(ctx); err != nil {
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("item %s: %w", i.ID, ErrNotFound)
		}

		return fmt.Errorf("item %s: %w", id, err)
	}

	if _, err := ref.Set(ctx, i); err != nil {
		return err
	}

	return nil
}
```

### Building the REST operations
So now that we've covered how REST should work, it's clear we need to change our routes and methods a bit. Let's build out all five operations, to match this router plan:

```go
	a.router.HandleFunc("/items", a.list).Methods(http.MethodGet)
	a.router.HandleFunc("/items", a.add).Methods(http.MethodPost)

	a.router.HandleFunc("/items/{id}", a.get).Methods(http.MethodGet)
	a.router.HandleFunc("/items/{id}", a.put).Methods(http.MethodPut)
	a.router.HandleFunc("/items/{id}", a.drop).Methods(http.MethodDelete)
```

We don't need to change `app.list()`; we can use it as-is. But we need to change `app.add()` to conform to REST correctly. 

When the object is created, we return a 201 status code (instead of 200), and attach a `Location` header that references the created object. However, if the object to be created (really, its ID) exists already, we should return a 409 "conflict" status.

```go
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
```

We're going to assume that if an ID was passed in, we'll have a conflict: the input data for a new object shouldn't have an ID assigned at all. 

We **must** set the status before we encode the output. It's a little-known detail about Go's HTTP package that if we write to the `ResponseWriter` before setting the status, it will be "200 OK" and we won't be able to change it (actually, we'll get a warning if we try to). It is safe to add headers before or after writing the status.

Here we create a location header using our server's URL and the created id, using a utility function.

```go
func (a *app) location(url *url.URL, host, id string) string {
	return fmt.Sprintf("http://%s%s/%s", host, url.String(), id)
}
```

The header will look like

```
Location: http://localhost:8080/items/92d4a080-348e-4ef7-8ec3-80ba008ccd68
```

when it's sent back to the client; the client must be able to use this URL as-is and immediately to fetch the object just created.

Since we're using the `Location` header, we don't really have to return the new object, so if its encoding fails, we won't override the status code because we have actually created the object and must report that.

Next we'll handle returning an individual item, given its ID. We must handle the case where the ID in fact doesn't correspond to an actual item in the database (for example, the item was already deleted).

```go
func (a *app) get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	item, err := a.db.GetItem(r.Context(), id)

	if err != nil {
		if errors.Is(err, ErrNotFound) {
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
		return
	}
}
```

This is one of three handlers that must get the ID from the URL given the route `/item/{id}`, that is, where the last part of the URL is matched as a variable. `mux.Vars()` returns a map of all such variables; in the unlikely event its missing (which as we saw before shouldn't happen), we'd look up an empty ID and fail in a "normal" way.

Note our use of `errors.Is()` to see if the underlying cause is a "not found" error: in our DB client method we deliberately wrapped such an error when Firestore told us the key didn't correspond to a document. We're using a new pattern of error handling established in Go 1.13; see the blog post [Working with Errors in Go 1.13](https://blog.golang.org/go1.13-errors).

Updating an item is similar; we must get the ID but also take data from the request body:

```go
func (a *app) put(w http.ResponseWriter, r *http.Request) {
	var item Item

	vars := mux.Vars(r)
	id := vars["id"]
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
```

We validate the incoming data (here validation is trivial: does it have a name?) and then call the update method, which will fail if the object doesn't exist already. We handle that just as for the `get` handler above. Note that we're not required to return data or a `Location` header.

Finally, deleting an item isn't too hard; all we need from the request is the ID, and deletion always succeeds unless there's an internal error.

```go
func (a *app) drop(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := a.db.DeleteItem(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
```

### Testing without a server
Our first test example ran pretty much the complete program. Here in a second example we'll show running a test using mocks that don't require network access at all.

#### DB mocks
We'll have to create a mock database which serves the same methods as the real Firestore client, with behavior as close as possible to the real thing.

For now, we'll assume we don't need thread safety in our DB mock, but we may want to induce deliberate failures.

```go
var (
	errShouldFail = errors.New("mock should fail")
	errInvalid    = errors.New("invalid operation")
)

type mockDB struct {
	data map[string]*Item
	fail bool
}
```

All our methods will mirror those of Firestore as well as how we wrap that logic in our real client. For example, the implementation of `AddItem()` should assign UUIDs in the same way.

```go
func (m *mockDB) AddItem(_ context.Context, i *Item) (string, error) {
	if m.fail {
		return "", errShouldFail
	}

	if m.data == nil {
		m.data = make(map[string]*Item)
	} else if i.ID != "" {
		return "", errInvalid
	}

add:
	i.ID = uuid.New().String()

	if _, ok := m.data[i.ID]; ok {
		goto add
	}

	m.data[i.ID] = i

	return i.ID, nil
}
```

Note how we first check whether the method should "just fail" before we do any real work; also, Go doesn't initialize maps for us, so we'll need to do that to avoid a panic the first time an object is created. We also fail if the item already has an ID, since it may already exist.

The other access methods are straightforward translations of database access to map access:

```go
func (m *mockDB) GetItem(_ context.Context, id string) (*Item, error) {
	if m.fail {
		return nil, errShouldFail
	}

	if i, ok := m.data[id]; ok {
		return i, nil
	}

	return nil, ErrNotFound
}
```
```go
func (m *mockDB) ListItems(_ context.Context) ([]*Item, error) {
	if m.fail {
		return nil, errShouldFail
	}

	result := make([]*Item, 0, len(m.data))

	for _, i := range m.data {
		result = append(result, i)
	}

	return result, nil
}
```
```go
func (m *mockDB) UpdateItem(_ context.Context, i *Item) error {
	if m.fail {
		return errShouldFail
	}

	if _, ok := m.data[i.ID]; !ok {
		return errInvalid
	}

	m.data[i.ID] = i

	return nil
}
```
```go
func (m *mockDB) DeleteItem(_ context.Context, id string) error {
	if m.fail {
		return errShouldFail
	}

	if m.data != nil {
		delete(m.data, id)
	}

	return nil
}
```

Finally, we'd like a way to preload some data for test purposes:

```go
func (m *mockDB) preload() {
	if m.data == nil {
		m.data = make(map[string]*Item)
	}

	for i := 1; i < 10; i++ {
		id := uuid.New().String()
		item := Item{ID: id, Name: fmt.Sprintf("item-%d", i)}

		m.data[id] = &item
	}
}
```

Again, we must pre-create the Go map for safety.

#### Restructuring the app
We also need to break up the app's setup routines a bit, since we won't actually start a server, but will need access to the router and mock DB.

Specifically, we need to break out the part where we set up routes, since we'll need to do that without creating an actual HTTP server.

```go
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
```
```go
func (a *app) addRoutes() {
	a.router.Use(logRequest)

	a.router.HandleFunc("/items", a.list).Methods(http.MethodGet)
	a.router.HandleFunc("/items", a.add).Methods(http.MethodPost)

	a.router.HandleFunc("/items/{id}", a.get).Methods(http.MethodGet)
	a.router.HandleFunc("/items/{id}", a.put).Methods(http.MethodPut)
	a.router.HandleFunc("/items/{id}", a.drop).Methods(http.MethodDelete)
}
```
```go
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

	return a.serve()
}
```

#### Running the test
Our test code will create a mock DB and then create an app with a router and that mock. We can then call the router with test requests and a special recorder that captures the handler's output. Note that the hostname part of the URL doesn't matter, since we're not actually connecting using a network.

```go
func TestWithMocks(t *testing.T) {
	d := &mockDB{}
	a := app{
		router: mux.NewRouter(),
		db:     d,
	}

	d.preload()
	a.addRoutes()

	r := httptest.NewRequest("GET", "http://who-cares/items", nil)
	w := httptest.NewRecorder()

	a.router.ServeHTTP(w, r)

	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("invalid response: %d", resp.StatusCode)
	}

	var result []Item

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	if len(result) != len(d.data) {
		t.Errorf("invalid result: %#v", result)
	}

	fmt.Println(result)
}
```

We can then decode the output and see if we got a list with items in it.

We can also test a couple of failure scenarios. First, we can actually make the mock DB fail and verify we get an "internal failure" error.

```go
func TestFailWithMocks(t *testing.T) {
	d := &mockDB{fail: true}
	a := app{
		router: mux.NewRouter(),
		db:     d,
		noAuth: true,
	}

	d.preload()
	a.addRoutes()

	r := httptest.NewRequest("GET", "http://who-cares/items", nil)
	w := httptest.NewRecorder()

	a.router.ServeHTTP(w, r)

	resp := w.Result()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("invalid response: %d", resp.StatusCode)
	}
}
```

Second, we can give the app an invalid item ID and see that we get a "not found" response.

```go
func TestNotFoundWithMocks(t *testing.T) {
	d := new(mockDB)
	a := app{
		router: mux.NewRouter(),
		db:     d,
		noAuth: true,
	}

	d.preload()
	a.addRoutes()

	r := httptest.NewRequest("GET", "http://who-cares/items/1", nil)
	w := httptest.NewRecorder()

	a.router.ServeHTTP(w, r)

	resp := w.Result()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("invalid response: %d", resp.StatusCode)
	}
}
```

Later we'll see how some of these tests could be combined into a table-driven unit test.

### Testing with a test server
This third test example fits between the first two; we'll mock the database but use Go's standard library test server, which will find a free port for us, so we're not at risk of having tests fail due to a port collision.

The key parts are

- we call `httptest.NewServer()` to make the test server
- we get the test server's URL which has a random port assignment

```go
func TestWithMockServer(t *testing.T) {
	d := &mockDB{}
	r := mux.NewRouter()
	s := httptest.NewServer(r)
	a := app{
		router: r,
		db:     d,
		noAuth: true,
	}

	d.preload()
	a.addRoutes()

	resp, err := http.Get(s.URL + "/items")

	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("invalid response: %d", resp.StatusCode)
	}

	var result []Item

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	if len(result) != len(d.data) {
		t.Errorf("invalid result: %#v", result)
	}

	fmt.Println(result)
}
```

There's not a lot of added benefit to this style of test over the second example, unless you have client code or utilities that expect to make a network connection in the test.

## Firestore transactions
In our next section, we're going to add an auto-incrementing ID to each item we store in the database. Here we'll call it a _SKU_ (which in retail means "stock keeping unit"). Firestore doesn't support this type of ID itself (as many SQL databases do), but we can fake it. Along the way we show how to run transactions in Firestore, because only in a transaction can we assign such an ID to database entries safely.

We'll change our `Item` definition with a new field

```go
type Item struct {
	ID   string `json:"id" firestore:"id"`
	Name string `json:"name" firestore:"name"`
	SKU  int    `json:"sku" firestore:"sku"`
}
```

### Creating the sequence
We need to extend our client with some private methods. First, when we create the client, we must see if the sequence number "document" exists and create it if it doesn't (which will always be true at least once).

(Note that when we use the Firestore emulator, it "forgets" all data any time we stop it. At the same time, if we leave it running with data when we start unit tests, we may get unexpected failures because the database doesn't start in a known state.)

Let's add a "startup" function into `NewClient()`, right after we make the client object:

```go
	if err = c.startSKU(ctx); err != nil {
		return nil, err
	}
```

Then we use this method to create a sequence if it's missing. Note that we can get a document ref outside the transaction since that's a local operation (no context, no error).

What's critically important is that within the closure we pass to `RunTransaction()`, all attempts to read or write use `tx` (the transaction passed in) instead of the client.

```go
const (
	skuDoc    = "Next$SKU"
	nextField = "next"
	startSKU  = 1000
)

func (c *Client) startSKU(ctx context.Context) error {
	ref := c.data.Doc(skuDoc)

	commit := func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(ref)

		if err != nil {
			if status.Code(err) == codes.NotFound {
				log.Println("no SKU doc, adding it")

				data := map[string]interface{}{
					nextField: startSKU,
				}

				if err := tx.Create(ref, data); err != nil {
					return fmt.Errorf("add SKU failed: %s", err)
				}

				return nil
			}

			return err
		}

		valRef, err := doc.DataAt(nextField)

		if err != nil {
			return err
		}

		if val, ok := valRef.(int64); ok {
			log.Printf("started SKU, %s = %v", nextField, val)
			return nil
		}

		return fmt.Errorf("can't read %s: %v", nextField, doc.Data())
	})
	
	return c.fs.RunTransaction(ctx, commit)
}
```

We didn't define a structure for our sequence "document"; instead we just use `map[string]{interface}` in a way similar to raw JSON.

If the document exists, we get the value of one field and use reflection to get its value as an integer.

Now, it turns out that the code above has a small problem, which we see immediately if we run the unit tests we already have (you always do that when changing code, don't you?). We put our sequence "document" into the same collection we use for items, and so when we list an "empty" database, we actually get an empty item back. (Please check this, and see if you can figure out why the item is empty.)

To avoid this problem, we'll use a separate collection just for keeping track of the sequence. First, we add a parameter to the app config

```go
type app struct {
	router  *mux.Router
	server  *http.Server
	db      DB
	addr    string
	project string
	data    string
	util    string
}
```

and add an option for it in `fromArgs()`

```go
	fl.StringVar(&a.data, "data", "items", "FS data collection")
	fl.StringVar(&a.util, "util", "util", "FS util collection")
```

then pass it to `NewClient`

```go

func (a *app) createClient() (err error) {
	a.db, err = NewClient(a.project, a.data, a.util)

	return
}
```

where it's used to make the client

```go
func NewClient(project, data, util string) (*Client, error) {
	. . .

	c := Client{
		fs:   client,
		data: client.Collection(data),
		util: client.Collection(util),
	}

	. . .
```

and finally we change the first line of `startSKU()` to use that "utility" collection and not the regular one:

```go
func (c *Client) startSKU(ctx context.Context) error {
	ref := c.util.Doc(skuDoc)
	. . .
```

We will also need a utility function we can run inside a transaction to get the next value in the sequence

```go
func getNext(seqRef *firestore.DocumentRef, tx *firestore.Transaction) (int, error) {
	seq, err := tx.Get(seqRef) // tx.Get, NOT ref.Get!

	if err != nil {
		return 0, err
	}

	valRef, err := seq.DataAt(nextField)

	if err != nil {
		return 0, err
	}

	val, ok := valRef.(int64)

	if !ok {
		return 0, fmt.Errorf("can't read %s", nextField)
	}

	return int(val), nil
}
```

Again, it's critically important that we get data through the transaction and not from the collection.

### Using the sequence to create items
The only DB client method we need to rewrite is `Client.AddItem()`. It will look almost the same, but we'll factor out our create logic into a function that replaces the usual Firestore `Create()` call.

```go
func (c *Client) AddItem(ctx context.Context, i *Item) (string, error) {
	var ref *firestore.DocumentRef

add:
	i.ID = uuid.New().String()
	ref = c.data.Doc(i.ID)

	if err := c.create(ctx, ref, i); err != nil {
		// it's unlikely to happen once and virtually
		// impossible for it to happen twice in a row

		if status.Code(err) == codes.AlreadyExists {
			goto add
		}

		return "", err
	}

	return i.ID, nil
}
```

Now our private client `Client.create()` method can run the transaction to get and update the sequence number while creating the item in Firestore.

One key detail about Firestore transactions is that _all reads must precede any writes_. In this case that's not a big deal, since we can't use the sequence number until we've read it. Again, note that we get the sequence document from the "utility" collection, but use the document ref passed in to `create` and captured in the closure to create the item.

```go
func (c *Client) create(ctx context.Context, ref *firestore.DocumentRef, item *Item) error {
	seqRef := c.util.Doc(skuDoc)

	commit := func(ctx context.Context, tx *firestore.Transaction) error {
		next, err := getNext(seqRef, tx)

		if err != nil {
			return err
		}

		item.SKU = next

		// if the transaction fails, this write will
		// also fail, so we shouldn't waste SKUs

		update := []firestore.Update{{
			Path: nextField, 
			Value: next + 1,
		}}

		if err := tx.Update(seqRef, update); err != nil {
			return err
		}

		// using Create here will prevent overwriting an
		// existing offer with the same UUID

		if err := tx.Create(ref, item); err != nil {
			return err
		}

		return nil
	})

	return c.fs.RunTransaction(ctx, commit)
}
```

In the event two transactions try to update the sequence number at the same time, one will fail and be retried by Firestore. Therefore, the closure we pass in to `RunTransaction()` must be safe to run more than once. If the closure must return data (by altering a variable local to `create()` it will need to (re-) create that data accordingly. However, if the closure returns an error of its own, Firestore will not run the closure again.

### Testing creation
Now that we've done this much, let's try to create some items and list them. Note that we don't have to change any of the web server handlers; they'll continue to work exactly as they are.

```
$ curl http://localhost:8080/items -X POST --data '{"name":"spoon"}' -i
HTTP/1.1 201 Created
Content-Type: application/json
Location: http://localhost:8080/items/b34825af-bc2f-4ebb-84f8-0a1d082f9823
Date: Sun, 27 Sep 2020 21:13:35 GMT
Content-Length: 72

{"id":"b34825af-bc2f-4ebb-84f8-0a1d082f9823","name":"spoon","sku":1000}
$ curl http://localhost:8080/items -X POST --data '{"name":"spork"}'
{"id":"92d4a080-348e-4ef7-8ec3-80ba008ccd68","name":"spork","sku":1001}
$ curl http://localhost:8080/items -X POST --data '{"name":"fork"}'
{"id":"e3828057-1373-4f98-ae08-e7ff2c7aa2ba","name":"fork","sku":1002}
```
```js
$ curl -s http://localhost:8080/items | jq
[
  {
    "id": "92d4a080-348e-4ef7-8ec3-80ba008ccd68",
    "name": "spork",
    "sku": 1001
  },
  {
    "id": "b34825af-bc2f-4ebb-84f8-0a1d082f9823",
    "name": "spoon",
    "sku": 1000
  },
  {
    "id": "e3828057-1373-4f98-ae08-e7ff2c7aa2ba",
    "name": "fork",
    "sku": 1002
  }
]
```

Note that as we created items, their SKUs were assigned in order, but when we list the items, they're ordered by UUID (go back and look at the logic in `Client.ListItems()`).

### Searching by SKU
Now that we have SKUs assigned to our items, we might like to list the SKUs or retrieve an item by SKU rather than by UUID. The "get by SKU" handler will allow us to show how to run a simple Firestore query.

Let's define a couple of new routes and then we'll fill in the handlers.

```go
	a.router.HandleFunc("/skus", a.listSKU).Methods(http.MethodGet)
	a.router.HandleFunc("/skus/{sku}", a.getSKU).Methods(http.MethodGet)
```

First, let's get a listing. We're going to use logic very similar to listing items, except we'll only return a map of SKU to item UUID. We need to add a new accessor to our DB client. In this case, our DB lookup will order by the "sku" field and not the document ID.

```go
func (c *Client) ListSKUs(ctx context.Context) (map[string]string, error) {
	query := c.data.OrderBy("sku", firestore.Asc)
	docs, err := query.Documents(ctx).GetAll()

	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(docs))

	for _, doc := range docs {
		var i Item

		if err = doc.DataTo(&i); err != nil {
			log.Printf("item %s decode: %s", doc.Ref.ID, err)
			continue
		}

		sku := strconv.Itoa(i.SKU)

		result[sku] = i.ID
	}

	return result, nil
}
```

While we're here, we'll add an accessor to get an item by SKU. Instead of creating a document ref using an ID, we create a query with a "where" clause to match any items with the correct value in the "sku" field. Note that the query may return no documents without an error, so we check the length of the return list.

```go
func (c *Client) GetItemBySKU(ctx context.Context, sku int) (*Item, error) {
	query := c.data.Where("sku", "==", sku)
	docs, err := query.Documents(ctx).GetAll()

	if err != nil {
		log.Printf("error finding sku %d: %s", sku, err)
		return nil, err
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("sku %d: %w", sku, ErrNotFound)
	}

	var i Item

	if err = docs[0].DataTo(&i); err != nil {
		log.Printf("item %s decode: %s", docs[0].Ref.ID, err)
		return nil, err
	}

	return &i, nil
}
```

In theory, we'll never have more than one document with the same SKU, so for now we won't handle that case. In real life, there are ways to duplicate a SKU in Firestore by accident, and we might want to be more aggressive about checking for that here and elsewhere.

Note that we'll also have to add these methods to the DB interface definition and to the `mockDB` type.

```go
type DB interface {
	AddItem(context.Context, *Item) (string, error)
	GetItem(context.Context, string) (*Item, error)
	GetItemBySKU(context.Context, int) (*Item, error)
	ListItems(context.Context) ([]*Item, error)
	ListSKUs(context.Context) (map[string]string, error)
	UpdateItem(context.Context, *Item) error
	DeleteItem(context.Context, string) error
}
```
```go
func (m *mockDB) GetItemBySKU(_ context.Context, sku int) (*Item, error) {
	if m.fail {
		return nil, errShouldFail
	}

	for _, v := range m.data {
		if v.SKU == sku {
			return v, nil
		}
	}

	return nil, ErrNotFound
}
```
```go
func (m *mockDB) ListSKUs(_ context.Context) (map[string]string, error) {
	if m.fail {
		return nil, errShouldFail
	}

	result := make(map[string]string, len(m.data))

	for _, i := range m.data {
		result[strconv.Itoa(i.SKU)] = i.ID
	}

	return result, nil
}
```

Now that we have DB accessors, we can write the handlers. Both of them look very much like the `/items` route handlers. To list SKUs, we just call the correct accessor and serialize the result.

```go
func (a *app) listSKU(w http.ResponseWriter, r *http.Request) {
	skus, err := a.db.ListSKUs(r.Context())

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err = json.NewEncoder(w).Encode(skus); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}
}
```

To find by SKU, we only need to get a different variable from the route and use a different DB accessor. We do need to handle validating that the "sku" variable is a valid integer.

```go
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
		return
	}
}
```

Let's try them out. Given the items we created above,

```js
$ curl -s http://localhost:8080/skus | jq
{
  "1000": "b34825af-bc2f-4ebb-84f8-0a1d082f9823",
  "1001": "92d4a080-348e-4ef7-8ec3-80ba008ccd68",
  "1002": "e3828057-1373-4f98-ae08-e7ff2c7aa2ba"
}
$ curl -s http://localhost:8080/skus/1001 | jq
{
  "id": "92d4a080-348e-4ef7-8ec3-80ba008ccd68",
  "name": "spork",
  "sku": 1001
}
$ curl -s http://localhost:8080/skus/1 -w "%{http_code}\n"
sku 1: not found
404
$ curl -s http://localhost:8080/skus/A -w "%{http_code}\n"
strconv.Atoi: parsing "A": invalid syntax
400
```

### A few router details
Gorilla mux allows us to specify regular expression patterns for variables in routes. For example, we can partly solve the problem of non-integer SKU values by changing the route to require a value using only digits:

```go
	a.router.HandleFunc("/skus/{sku:[0-9]+}", a.getSKU).Methods(http.MethodGet)
```

such that the error we see from that last example above becomes

```
$ curl -s http://localhost:8080/skus/A -w "%{http_code}\n"
404 page not found
```

Note that `[0-9]+` requires at least one digit due to the `+` sign.

Also note that Gorilla mux treats trailing slashes "strictly". That means a route `/users` cannot be accessed with `/users/` and vice versa. If you want to treat both `/users` and `/users/` (note: no variable) the same, you have a couple of options:

- register both routes with the same handler and method(s)
- "fix" the route before it's handed off to the router

For the second option, here's an interesting article [Dealing with Trailing Slashes on RequestURI in Go with Mux](https://natedenlinger.com/dealing-with-trailing-slashes-on-requesturi-in-go-with-mux/) that shows an example.

### Starting the emulator in tests
Here we'll adapt another good article [Unit Testing with Firestore Emulator and Go](https://www.captaincodeman.com/2020/03/04/unit-testing-with-firestore-emulator-and-go) so that our unit tests can always have an emulator.

In Go, if we define a function `TestMain` it will be run before all unit tests in our module. We then run the tests and report a final exit value.

```go
func TestMain(m *testing.M) {
	stop, err := startEmulator()

	if err != nil {
		log.Println("*** FAILED TO START EMULATOR ***")
		os.Exit(-1)
	}

	// it would be nice if we could catch a panic
	// and ensure we close the emulator properly,
	// but we can't because UTs are in goroutines

	result := m.Run()

	// don't defer, since we don't return
	// but rather exit with a code

	stop()
	os.Exit(result)
}
```

The `startEmulator()` function is a bit involved.

```go
func startEmulator() (func(), error) {
	const firestoreEmulatorHost = "FIRESTORE_EMULATOR_HOST"

	// we assume that if this env variable is already set,
	// there is an emulator running externally we can use

	if os.Getenv(firestoreEmulatorHost) != "" {
		return func() {}, nil
	}

	cmd := exec.Command("gcloud", "beta", "emulators", "firestore", "start", "--host-port=localhost:8086")

	// this makes it killable
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// capture its output so we can check it's started

	stderr, err := cmd.StderrPipe()

	if err != nil {
		log.Fatal(err)
	}

	defer stderr.Close()

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	timer := time.After(20 * time.Second)
	done := make(chan string)

	// we'll scrape the command's output in a separate
	// goroutine so we can timeout if it hangs up

	go func() {
		defer close(done)

		buf := make([]byte, 1024)

		for {
			n, err := stderr.Read(buf[:])

			if err != nil {
				if err == io.EOF {
					return
				}

				log.Fatalf("reading stderr: %v", err)
			}

			if n > 0 {
				d := string(buf[:n])

				// checking for an obvious failure
				if strings.Contains(d, "Exception") {
					done <- strings.TrimSuffix(d, "\n")
					return
				}

				// checking for the message that it's started
				if strings.Contains(d, "Dev App Server is now running") {
					done <- "running"
					return
				}

				// and capturing the FIRESTORE_EMULATOR_HOST value to set;
				// we will get this before we get the startup message above
				if pos := strings.Index(d, firestoreEmulatorHost+"="); pos > 0 {
					host := d[pos+len(firestoreEmulatorHost)+1 : len(d)-1]
					os.Setenv(firestoreEmulatorHost, host)
				}
			}
		}
	}()

	stop := func() {
		if err = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			log.Printf("failed to stop emulator pid=%d\n", cmd.Process.Pid)
			return
		}

		log.Printf("stopped emulator pid=%d\n", cmd.Process.Pid)
	}

	select {
	case result := <-done:
		if result == "running" {
			log.Printf("emulator running; pid=%d\n", cmd.Process.Pid)
			return stop, nil
		}

		// i.e., print something like "address in use" message
		log.Println(result)

	case <-timer:
		// fall through
	}

	stop()
	return func() {}, fmt.Errorf("failed to start emulator")
}
```

First, we check to see if the emulator is already running (which we assume if the environment variable. In Drone CI/CD, we typically run the Firestore emulator as a step from a Docker container that already has all the software installed, so we wouldn't want to start it here.

Next we start the emulator, and then we kick off a goroutine to scrape its output looking for either the magic phrase ("emulator running") or some type of Java exception (typically, port already in use). We also take actually set the environment variable in our process so when we try to make a Firestore client, it sees the emulator.

We don't want to wait for ever if something goes wrong, so we use `select` to choose between two alternatives: the scraper can tell us we've started or not, or we hit a timeout waiting for it (because somehow our command hung up).

Finally, we return a `stop()` function that can correctly stop the emulator by killing the correct process id (PID). Note that in some cases, if we already know we've failed, we just call stop() and don't return it.

Now, this is all somewhat complicated; if you're not familiar with Go's concurrency model, just use it, and then come back and study it later when you've gone further in the tutorial.

#### Utilities
There are also a couple of utility functions that might be useful, to preload and clean up the emulator. Ideally, each unit test should stand alone: add to the emulator any initial data, and remove **all** data from it, whether preloaded or added in the unit test.

We'll show the cleanup routine, since it's pretty generic, and leave preloading as an exercise. You could preload from files, or perhaps from a map literal in the code, for example.

The cleanup function makes no assumptions; given only the project, it will find all collections and remove all documents from all of them, using a batch write for the deletion. It avoids using the software under test.

```go
func clearEmulator(project string) error {
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, project)

	if err != nil {
		return fmt.Errorf("deleting: %s", err)
	}

	defer client.Close()

	batch := client.Batch()
	colls := client.Collections(ctx)

	for coll, err := colls.Next(); err != iterator.Done; coll, err = colls.Next() {
		if err != nil {
			return fmt.Errorf("clearing: %s", err)
		}

		iter := coll.Documents(ctx)

		defer iter.Stop()

		for doc, err := iter.Next(); err != iterator.Done; doc, err = iter.Next() {
			if err != nil {
				return fmt.Errorf("clearing: %s", err)
			}

			batch.Delete(doc.Ref)
			log.Println("unloaded " + doc.Ref.ID)
		}
	}

	if _, err := batch.Commit(ctx); err != nil {
		return fmt.Errorf("clearing: %s", err)
	}

	return nil
}
```

A typical unit test would then do something like this:

```go
func TestMeSport(t *testing.T) {
	if err := preloadEmulator(projectID, . . .); err != nil {
		t.Fatal(fmt.Errorf("couldn't preload: %w", err))
	}

	defer func() {
		if err := clearEmulator(projectID); err != nil {
			t.Errorf("failed to empty")
		}
	}()

	. . .
```

## GraphQL
In our last section, we'll add GraphQL support using the [gqlgen](https://gqlgen.com) library. This library provides the most complete support for GraphQL among the various choices for Go. It will also auto-generate much of our code for us in a way that promotes compile-time type safety.

By the way, GraphQL as a technology has the same limitation as REST, in that it's concerned with CRUD operations. It may not be well-suited for some  command-focused (as opposed to data-focused) APIs, such as the sound file conversion example we mentioned before.

### Setup
We're going to let `gqlgen` create a dummy project, and then we'll copy our existing tutorial into that new directory. This will be surprisingly easy, with just a few loose ends we need to tie up.

```
$ mkdir tutor4
$ cd tutor4
$ go mod init tutor4
$ go get github.com/99designs/gqlgen
$ go run github.com/99designs/gqlgen init
Exec "go run ./server.go" to start GraphQL server
```

Except we're not going to start it yet. We have several things we need to do first.

1. copy over the existing source files
2. edit the schema
3. remove our definition of Item
4. move db.go and db_test.go to its own package to avoid a cycle
5. inject the DB client into the Resolver type
6. regenerate
7. copy Item to new file and add firestore field names
8. edit the schema to point to model using @goModel directive
9. fix the use of Item in our existing code (use model.Item)
10. plug the graphQL handlers into our app setup
11. fill in the three resolvers we need
12. profit !!

In my case, I had the following files to copy:

```
$ ls
app.go		db.go		go.mod		model.go	serve_test.go
cmd			db_test.go	go.sum		serve.go
```

Note that `cmd/main.go` doesn't need to change. We're going to end up with this tree

```
$ tree
.
├── app.go
├── cmd
│   └── main.go
├── db
│   ├── db.go
│   └── db_test.go
├── go.mod
├── go.sum
├── gqlgen.yml
├── graph
│   ├── generated
│   │   └── generated.go
│   ├── model
│   │   ├── item.go
│   │   └── models_gen.go
│   ├── resolver.go
│   ├── schema.graphqls
│   └── schema.resolvers.go
├── serve.go
├── serve_test.go
└── server-gen.txt
```

The file `model.go` can go away for now. `server-gen.txt` is what the `init` command created as `server-gen.go`; we'll copy a couple of lines out of it but otherwise ignore it. (We must rename it so it doesn't create a problem when we build.)

Let's go ahead and take care of #4; if we wait it will just cause errors:

```
$ mkdir db
$ mv db*.go db
$ find ./db -name '*.go' -exec sed -i '' 's/tutor4/db/' {} +
```

That's because we need to open `graph/resolver.go` and make a change so that our resolvers have access to the DB client (#5):

```go
import "tutor4/db"

type Resolver struct {
	Client db.DB
}
```

and once we've done that, we'll have an eventual import cycle (once we reference the GraphQL stuff from our app) that we might as well break now.

Now that the DB code has moved, add `tutor4/db` everywhere the DB code is used, just as we did in `resolver.go`.

### Editing code
OK, back to #2; we still don't have a schema for _our_ app, just for the dummy app. Edit `graph/schema.graphqls`, remove everything that's there, and replace it with this:

```graphql
type Item {
	id: ID!
	name: String!
	sku: Int!
}

type Query {
	items: [Item!]!
}

input NewItem {
	name: String!
}

type Mutation {
	createItem(input: NewItem!): Item!
}
```

Also, remove the original `model.go` file with its definition of type `Item` (#3).

Let's jump over #4 and #5 (we just did them above), and regenerate code from our schema (#6). This will replace certain files, but shouldn't mess with our updated `resolver.go`:

```
go run github.com/99designs/gqlgen generate
```

So one small drawback to `gqlgen` is that we can't get custom struct tags for our model objects easily. Our choices are

- copy the model object, fix the tags, and don't regenerate it
- create a plugin that alters how `gqlgen` generates the model object
- live without struct tags for Firestore serialization

The second option isn't that hard, given some examples on the Internet, but we're not going down that road for now.

Model objects are created in `graph/models/model_gen.go`, so we'll place our "fixed" definition in the same directory (#7). Move the text for type `Item` from `model_gen.go` into a new file `graph/models/item.go`, and update it so it looks like

```go
type Item struct {
	ID   string `json:"id" firestore:"id"`
	Name string `json:"name" firestore:"name"`
	SKU  int    `json:"sku" firestore:"sku"`
}
```

which is pretty much what we had before; we're just adding back the Firestore tags (compared to the generated `Item`); if we don't, we'll have a problem with our queries that use the field name "sku" (capitalization).

Now, if we don't do one more step, the next time we regenerate we'll get a duplicate (and not quite desired) version of `Item` in `model_gen.go`. To avoid that, we're going to tell `gqlgen` to use our existing model object and not create one.

We need to re-edit our schema file to make it look like this (#8):

```graphql
directive @goModel(model:String,models:[String!]) on 
	OBJECT|INPUT_OBJECT|SCALAR|ENUM|INTERFACE|UNION

type Item @goModel(model: "tutor4/graph/model.Item") {
	id: ID!
	name: String!
	sku: Int!
}

. . .
```

The first line just activates a built-in directive specific to `gqlgen` that allows us to use an existing type; then we use it on `Item` to identify our type.

The next step is to carry out a couple a simple transformation: change `Item` to `model.Item` everywhere (#9).

Before we fill in the resolvers, let's add the GraphQL routes into our app. We need to add the GraphQL engine into our app type:

```go
import (
	. . .
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"

	"tutor4/db"
	"tutor4/graph"
	"tutor4/graph/generated"
)

type app struct {
	router  *mux.Router
	server  *http.Server
	graphql *handler.Server
	. . .
}
```

In `app.go`, we need to update `addRoutes()`. First, add these lines to the top of the function, which creates the server with our DB injected into the resolver (#10):

```go
	r := graph.Resolver{Client: a.db}
	c := generated.Config{Resolvers: &r}
	s := generated.NewExecutableSchema(c)

	a.graphql = handler.NewDefaultServer(s)
```

and then add two new routes below:

```go
	play := playground.Handler("GraphQL playground", "/graphql")

	a.router.Handle("/", play)
	a.router.Handle("/graphql", a.graphql)
```

### Adding resolvers
Finally, we're ready to add our custom resolver code (#11). Edit `graph/schema.resolvers.go` and we'll find two functions whose body is just a panic. Yes, just two functions that we need to provide; everything else has been generated for us (more than 2000 lines of code! compile-time safe code!).

First, we need to fill in `CreateItem`. It's going to look remarkably similar to our `add` handler, except that we don't need to deserialize any JSON (that's already done) and we don't need to write HTTP errors to the response, just return an error (if any) from the function.

```go
func (r *mutationResolver) CreateItem(ctx context.Context, input model.NewItem) (*model.Item, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("no name")
	}

	item := model.Item{
		Name: input.Name,
	}

	_, err := r.Client.AddItem(ctx, &item)

	if err != nil {
		return nil, err
	}

	return &item, nil
}
```

We do need to copy data from the input type `model.NewItem` (which was generated for us, and we'll keep) into our `Item` type. Then we just access the resolver's DB dependency and write it to the database.

The `Items` method is even easier; we just fetch data from the database and return it!

```go
func (r *queryResolver) Items(ctx context.Context) ([]*model.Item, error) {
	items, err := r.Client.ListItems(ctx)

	if err != nil {
		return nil, err
	}

	return items, nil
}
```

That's it! If we've done the steps correctly, we can start up our server and get to work.

One option is to open a browser window and navigate to `http://localhost:8080`. That will start a GraphQL playground where you can inspect the schema and execute requests (right now, only to create an item or list all items).

Note that we left all our REST-based methods in place, so you can still add or list data as before, in case you want to preload the database or need to remove an item.

Alternatively, we can run GraphQL queries with curl. Note that now we **must** include the content-type header or we'll get an error. Also, we must use POST. The GraphQL server will normally return a 200 response even if there's an error (see below for an error example, where it's part of the returned JSON).

```js
$ curl -s http://localhost:8080/graphql -X POST -H 'Content-Type: application/json' \
  --data '{"operationName":null,"variables":{},"query":"{items {name sku}}"}' | jq
{
  "data": {
    "items": []
  }
}
```

Actually, if we're not using them, we can omit the `operationName` and `variables` fields:

```js
$ curl -s http://localhost:8080/graphql -X POST -H 'Content-Type: application/json' \
  --data '{"query":"{items {name sku}}"}' | jq
{
  "data": {
    "items": []
  }
}
```

OK, we still have no data. So let's try to add some:

```js
$ curl -s http://localhost:8080/graphql -X POST -H 'Content-Type: application/json' \
  --data '{"query":"mutation {createItem(input:{name: \"knife\"}) {id sku}}"}' | jq
{
  "data": {
    "createItem": {
      "id": "45e8d301-d584-4c8d-be8d-40953b0c3f08",
      "sku": 1000
    }
  }
}
```
```js
$ curl -s http://localhost:8080/graphql -X POST -H 'Content-Type: application/json' \
  --data '{"query":"mutation {createItem(input:{name: \"spork\"}) {id sku}}"}' | jq
{
  "data": {
    "createItem": {
      "id": "55ae421d-5a43-486c-b3b4-db3eda664bde",
      "sku": 1001
    }
  }
}
```
```js
$ curl -s http://localhost:8080/graphql -X POST -H 'Content-Type: application/json' \
  --data '{"query":"{items {id name sku}}"}' | jq
{
  "data": {
    "items": [
      {
        "id": "45e8d301-d584-4c8d-be8d-40953b0c3f08",
        "name": "knife",
        "sku": 1000
      },
      {
        "id": "55ae421d-5a43-486c-b3b4-db3eda664bde",
        "name": "spork",
        "sku": 1001
      }
    ]
  }
}
```

### One more thing
Let's add the ability to get an Item by its SKU. We'll need to add to our schema, regenerate, and then fill in one more resolver.

First, change type `Query` in the schema:

```graphql
type Query {
	items: [Item!]!
    item(sku: Int!): Item
}
```

Note that the return type for the `item(sku)` lookup does not have a bang (exlamation point, `!`) at the end: if we can't find the SKU in the database, we'll return null.

Now regenerate the code. The file `graph/resolver.go` should be unchanged except that we'll now have one more resolver to fill in.

Again, that's pretty easy:

```go
func (r *queryResolver) Item(ctx context.Context, sku int) (*model.Item, error) {
	item, err := r.Client.GetItemBySKU(ctx, sku)

	if err != nil {
		return nil, err
	}

	return item, nil
}
```

Note that we no longer need to convert the incoming SKU to an integer, as we did with the REST handler.

Restart the server (we left the DB running, so it should still have data), and let's give it a try:

```js
$ curl -s http://localhost:8080/graphql -X POST -H 'Content-Type: application/json' \
  --data '{"query":"{item(sku:1001) {id name}}"}' -w "%{http_code}\n" | jq
{
  "data": {
    "item": {
      "id": "55ae421d-5a43-486c-b3b4-db3eda664bde",
      "name": "spork"
    }
  }
}
200
```
```js
$ curl -s http://localhost:8080/graphql -X POST -H 'Content-Type: application/json' \
  --data '{"query":"{item(sku:100) {id name}}"}' -w "%{http_code}\n" | jq
{
  "errors": [
    {
      "message": "sku 100: not found",
      "path": [
        "item"
      ]
    }
  ],
  "data": {
    "item": null
  }
}
200
```
```js
$ curl -s http://localhost:8080/graphql -X POST -H 'Content-Type: application/json' \
  --data '{"query":"{item(sku:A) {id name}}"}' -w "%{http_code}\n" | jq
{
  "errors": [
    {
      "message": "Expected type Int!, found A.",
      "locations": [
        {
          "line": 1,
          "column": 11
        }
      ],
      "extensions": {
        "code": "GRAPHQL_VALIDATION_FAILED"
      }
    }
  ],
  "data": null
}
200
```

Notice how the GraphQL handler returns errors in case we don't have that SKU, or if the SKU in our query is not an integer (validation was done for us).


Also notice that the logging middleware we added to our router works perfectly well with GraphQL queries, since from the router's perspective, the GraphQL handler is just another handler.

```
$ go run ./cmd
2020/09/28 17:28:47 no SKU doc, adding it
2020/09/28 17:28:47 server started on localhost:8080
2020/09/28 17:35:36 POST /graphql {"query":"{items {name sku}}"}
2020/09/28 17:38:12 POST /graphql {"query":"mutation {createItem(input:{name: \"knife\"}) {id sku}}"}
2020/09/28 17:39:58 POST /graphql {"query":"mutation {createItem(input:{name: \"spork\"}) {id sku}}"}
2020/09/28 17:40:13 POST /graphql {"query":"{items {id name sku}}"}
2020/09/28 17:50:42 POST /graphql {"query":"{item(sku:1001) {id name}}"}
2020/09/28 17:51:20 POST /graphql {"query":"{item(sku:100) {id name}}"}
2020/09/28 17:51:28 POST /graphql {"query":"{item(sku:A) {id name}}"}
```

If you're wondering where the GraphQL input came from, we added a bit to the logging middleware at some point, capturing the whole input and providing it back to the request in a replacecment `Body`:

```go
func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, err := ioutil.ReadAll(r.Body)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		r.Body = ioutil.NopCloser(bytes.NewBuffer(buf))

		log.Println(r.Method, r.RequestURI, bytes.NewBuffer(buf).String())
		next.ServeHTTP(w, r)
	})
}
```

It's possible to capture and log the output of from serving the request (you'd do that after calling `ServeHTTP`), but that involves replacing the `ResponseWriter`. It's not too hard, and there are examples on the Internet, so we'll leave that as an exercise.

### Summary
There's one important thing to note here: how little code we had to write in order to serve GraphQL, compared to REST. We wrote essentially three methods to wrap DB access functions, plus a couple of extra lines to configure the app at startup. That's in large part because we used `gqlgen` which generated lots of tedious boilerplate code for us. This is step #12: **profit!** Using auto-generated GraphQL takes less work for more functionality and safety.

An aside on safety: Go is a compiled language, and we'd like to validate at compile time as much of our code (and GraphQL model) as possible. Some other GraphQL libraries require us lots of tedious code that isn't checked until runtime (do you have enough unit tests? do you even know how many you need?). This isn't much better than building in Python or JS where you really don't know anything about your program until you run it. Compile-time safety is a very important benefit of `gqlgen`, just as much as the cost savings of all the code we didn't have to write.

## Moving on
That wraps up this tutorial on building REST and GraphQL web servers with Go. Feel free to expand on this and try some things.

For example, add more interesting data to `Item` and then add methods to query that data from Firestore and through GraphQL. Also, add GraphQL mutators to change and/or delete data. Note that there's not much point in adding the GraphQL equivalent of `/skus` routes; it's enough to use the existing `items` query and get a list which can specify what fields to return.
