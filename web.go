package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"sync"
	"time"
)

//go:embed index.html
var overlayHTML []byte

type DeathCounter struct {
	mu        sync.RWMutex
	deaths    int
	name      string
	listeners []chan struct{}
}

var counter = &DeathCounter{}

func (dc *DeathCounter) Update(deaths int, name string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.deaths = deaths
	dc.name = name
	for _, ch := range dc.listeners {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (dc *DeathCounter) Get() (int, string) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.deaths, dc.name
}

func (dc *DeathCounter) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	dc.mu.Lock()
	dc.listeners = append(dc.listeners, ch)
	dc.mu.Unlock()
	return ch
}

func (dc *DeathCounter) unsubscribe(ch chan struct{}) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if i := slices.Index(dc.listeners, ch); i >= 0 {
		dc.listeners = slices.Delete(dc.listeners, i, i+1)
	}
}

type apiResponse struct {
	Deaths int    `json:"deaths"`
	Name   string `json:"name"`
}

func startWebServer(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", overlayHandler)
	mux.HandleFunc("/api/deaths", deathsAPIHandler)
	mux.HandleFunc("/api/events", sseHandler)

	server := &http.Server{
		Addr:           fmt.Sprintf(":%d", port),
		Handler:        mux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   0,
		MaxHeaderBytes: 1 << 10,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func overlayHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(overlayHTML)
}

func deathsAPIHandler(w http.ResponseWriter, r *http.Request) {
	deaths, name := counter.Get()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiResponse{deaths, name})
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := counter.subscribe()
	defer counter.unsubscribe(ch)

	send := func() {
		deaths, name := counter.Get()
		data, _ := json.Marshal(apiResponse{deaths, name})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	send()

	for {
		select {
		case <-ch:
			send()
		case <-r.Context().Done():
			return
		}
	}
}
