//go:build !solution

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"net/http"
	"sync"
)

type Store struct {
	mu       sync.Mutex
	keyToURL map[string]string
}

func (s *Store) Set(url string) string {
	sum := crc32.ChecksumIEEE([]byte(url))
	key := fmt.Sprintf("%08x", sum)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.keyToURL[key] = url
	return key
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	url, ok := s.keyToURL[key]
	return url, ok
}

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	flag.Parse()

	store := &Store{keyToURL: make(map[string]string)}
	mux := http.NewServeMux()

	mux.HandleFunc("POST /shorten", func(w http.ResponseWriter, r *http.Request) {
		type RequestBody struct {
			URL string `json:"url"`
		}

		var body RequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		key := store.Set(body.URL)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"url": body.URL,
			"key": key,
		})
	})

	mux.HandleFunc("GET /go/{key}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		url, ok := store.Get(key)
		if !ok {
			http.Error(w, "key not found", http.StatusNotFound)
			return
		}
		http.Redirect(w, r, url, http.StatusFound)
	})

	addr := fmt.Sprintf(":%d", *port)
	http.ListenAndServe(addr, mux)
}
