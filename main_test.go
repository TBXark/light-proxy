package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"testing"
)

func TestProxy(test *testing.T) {
	config := Config{
		Address: ":8080",
		Locations: []*LocationConfig{
			{
				Path:   "/v1",
				Target: "http://localhost:8081",
				Rewrite: &PathRwConfig{
					From: "^/v1",
					To:   "",
				},
			},
			{
				Path:   "/v2",
				Target: "http://localhost:8082",
			},
		},
	}
	startDemoAPI := func(addr string) {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			body := map[string]string{
				"method": r.Method,
				"uri":    r.RequestURI,
				"query":  r.URL.RawQuery,
			}
			if r.Body != nil {
				defer r.Body.Close()
				buf := new(bytes.Buffer)
				buf.ReadFrom(r.Body)
				body["body"] = buf.String()
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			bits, err := json.Marshal(body)
			if err != nil {
				w.Write([]byte(err.Error()))
			} else {
				w.Write(bits)
			}
		})
		log.Fatal(http.ListenAndServe(addr, mux))
	}
	go startDemoAPI(":8081")
	go startDemoAPI(":8082")
	go createProxyServer(&config)

	// test proxy
	testProxy := func(path, method, body string) map[string]any {
		client := &http.Client{}
		req, err := http.NewRequest(method, "http://localhost:8080"+path, bytes.NewBufferString(body))
		if err != nil {
			test.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			test.Fatal(err)
		}
		defer resp.Body.Close()
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(resp.Body)
		if err != nil {
			test.Fatal(err)
		}
		var data map[string]any
		err = json.Unmarshal(buf.Bytes(), &data)
		if err != nil {
			test.Fatal(err)
		}
		return data
	}

	t1 := testProxy("/v1/callback?hello=1&world=apple", "GET", "")
	if t1["method"] != "GET" {
		test.Fatal("method mismatch")
	}
	if t1["uri"] != "/callback?hello=1&world=apple" {
		test.Fatal("uri mismatch")
	}

	t2 := testProxy("/v1/status", "POST", "{\"hello\": \"world\"}")
	if t2["method"] != "POST" {
		test.Fatal("method mismatch")
	}
	if t2["uri"] != "/status" {
		test.Fatal("uri mismatch")
	}
	if t2["body"] != "{\"hello\": \"world\"}" {
		test.Fatal("body mismatch")
	}

	t3 := testProxy("/v2/callback?hello=2", "GET", "")
	if t3["method"] != "GET" {
		test.Fatal("method mismatch")
	}
	if t3["uri"] != "/v2/callback?hello=2" {
		test.Fatal("uri mismatch")
	}

	t4 := testProxy("/v2/status#test", "POST", "hello")
	if t4["method"] != "POST" {
		test.Fatal("method mismatch")
	}
	if t4["uri"] != "/v2/status" {
		test.Fatal("uri mismatch")
	}
	if t4["body"] != "hello" {
		test.Fatal("body mismatch")
	}
}
