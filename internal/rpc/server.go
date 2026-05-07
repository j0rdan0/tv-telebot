package rpc

import (
	"log"
	"net"
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

func StartServer(controller *tv.Controller, port string) {
	server := rpc.NewServer()
	service := &TVService{controller: controller}
	err := server.Register(service)
	if err != nil {
		log.Fatalf("Error registering RPC service: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle(rpc.DefaultRPCPath, server)
	mux.Handle(rpc.DefaultDebugPath, server)

	// CORS middleware to allow requests from any origin
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		mux.ServeHTTP(w, r)
	})

	l, err := net.Listen("tcp4", ":"+port)
	if err != nil {
		log.Fatalf("RPC listen error: %v", err)
	}
	log.Printf("RPC server listening on port %s (CORS enabled)", port)
	go http.Serve(l, handler)
}
