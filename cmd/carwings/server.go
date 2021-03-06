package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/karora/carwings"
)

func updateLoop(ctx context.Context, s *carwings.Session) {
	_, err := s.UpdateStatus()
	if err != nil {
		fmt.Printf("Error updating status: %s\n", err)
	}

	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-t.C:
			_, err := s.UpdateStatus()
			if err != nil {
				fmt.Printf("Error updating status: %s\n", err)
			}
		}
	}
}

func runServer(s *carwings.Session, args []string) error {
	var srv http.Server

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-ch
		cancel()
		srv.Shutdown(context.Background())
	}()

	go updateLoop(ctx, s)

	const timeout = 5 * time.Second

	http.HandleFunc("/charging/on", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			fmt.Println("Charging request")

			ch := make(chan error, 1)
			go func() {
				ch <- s.ChargingRequest()
			}()

			select {
			case err := <-ch:
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}

			case <-time.After(timeout):
				w.WriteHeader(http.StatusAccepted)
			}

		default:
			http.NotFound(w, r)
			return
		}
	})

	http.HandleFunc("/climate/on", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			fmt.Println("Climate control on request")

			ch := make(chan error, 1)
			go func() {
				_, err := s.ClimateOnRequest()
				ch <- err
			}()

			select {
			case err := <-ch:
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}

			case <-time.After(timeout):
				w.WriteHeader(http.StatusAccepted)
			}

		default:
			http.NotFound(w, r)
			return
		}
	})

	http.HandleFunc("/climate/off", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			fmt.Println("Climate control off request")

			ch := make(chan error, 1)
			go func() {
				_, err := s.ClimateOffRequest()
				ch <- err
			}()

			select {
			case err := <-ch:
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}

			case <-time.After(timeout):
				w.WriteHeader(http.StatusAccepted)
			}

		default:
			http.NotFound(w, r)
			return
		}
	})

	srv.Addr = ":8040"
	srv.Handler = nil
	fmt.Printf("Starting HTTP server on %s...\n", srv.Addr)
	return srv.ListenAndServe()
}
