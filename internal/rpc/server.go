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
	service := &TVService{controller: controller}
	err := rpc.Register(service)
	if err != nil {
		log.Fatalf("Error registering RPC service: %v", err)
	}
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("RPC listen error: %v", err)
	}
	log.Printf("RPC server listening on port %s", port)
	go http.Serve(l, nil)
}
