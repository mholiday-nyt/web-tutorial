package tutor4

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

// TestWithApp **MUST** have a Firestore emulator
// running and also uses the network; it may be
// fragile if the port is already in use
//
// but it's a full door-to-door test of the app
func TestWithApp(t *testing.T) {
	args := []string{"-addr", "localhost:8089"}

	go RunApp(args)

	time.Sleep(2 * time.Second)

	client := http.Client{}
	req, _ := http.NewRequest("GET", "http://localhost:8089/items", nil)

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

	if result != "[]" {
		t.Errorf("invalid response: %q: %[1]v", body)
	}
}

// TestWithMocks requires no network at all, so
// the URL host doesn't really matter
func TestWithMocks(t *testing.T) {
	d := new(mockDB)
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

	for i := range result {
		if result[i].SKU < 1000 || result[i].SKU > 1009 {
			t.Errorf("invalid SKU: %#v", result[i])
		}
	}
}

// TestWithMockServer uses only the loopback
// connection with a random port
func TestWithMockServer(t *testing.T) {
	d := new(mockDB)
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

// TestFailWithMocks creates a DB that won't work
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

// TestNotFoundWithMocks tries to get an item that doesn't exist
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
