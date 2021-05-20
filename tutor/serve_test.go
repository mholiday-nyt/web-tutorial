package tutor

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

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
