//go:build !solution

package httpgauge

import (
	"fmt"
	"net/http"
	"sort"
	"sync"

	"github.com/go-chi/chi/v5"
)

type Gauge struct {
	mu     sync.Mutex
	counts map[string]int
}

func New() *Gauge {
	return &Gauge{counts: make(map[string]int)}
}

// Snapshot возвращает копию текущих счётчиков
func (g *Gauge) Snapshot() map[string]int {
	g.mu.Lock()
	defer g.mu.Unlock()

	copy := make(map[string]int, len(g.counts))
	for k, v := range g.counts {
		copy[k] = v
	}
	return copy
}

// ServeHTTP отдаёт статистику в текстовом виде, отсортированную по паттерну
func (g *Gauge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	snapshot := g.Snapshot()

	keys := make([]string, 0, len(snapshot))
	for k := range snapshot {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Fprintf(w, "%s %d\n", k, snapshot[k])
	}
}

// Wrap — middleware, которая считает запросы по паттерну маршрута
func (g *Gauge) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Сначала пропускаем запрос дальше — chi заполнит RouteContext
		next.ServeHTTP(w, r)

		// Только после этого паттерн известен, например "/user/{userID}"
		pattern := chi.RouteContext(r.Context()).RoutePattern()
		if pattern == "" {
			return
		}

		g.mu.Lock()
		g.counts[pattern]++
		g.mu.Unlock()
	})
}
