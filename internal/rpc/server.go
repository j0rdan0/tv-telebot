package rpc

import (
	"log"
	"net/http"
	"net/rpc"
	"telegram-bot/internal/tv"
)

type TVService struct {
	controller *tv.Controller
}

func (s *TVService) Start(args *struct{}, reply *string) error {
	log.Println("RPC: Received Start request")
	err := s.controller.Start()
	if err != nil {
		return err
	}
	*reply = "TV started successfully"
	return nil
}

func (s *TVService) Stop(args *struct{}, reply *string) error {
	log.Println("RPC: Received Stop request")
	err := s.controller.Stop()
	if err != nil {
		return err
	}
	*reply = "TV stopped successfully"
	return nil
}

func RegisterRPC(controller *tv.Controller, mux *http.ServeMux) {
	server := rpc.NewServer()
	service := &TVService{controller: controller}
	err := server.Register(service)
	if err != nil {
		log.Fatalf("Error registering RPC service: %v", err)
	}

	// Use HandleFunc to ensure we catch the requests
	mux.HandleFunc(rpc.DefaultRPCPath, func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r)
	})
	mux.Handle(rpc.DefaultDebugPath, server)
	
	log.Printf("RPC service registered on /_goRPC_ (port 8080)")
}
