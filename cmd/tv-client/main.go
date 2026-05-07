package main

import (
	"flag"
	"fmt"
	"log"
	"net/rpc"
	"os"
)

func main() {
	serverAddr := flag.String("server", "raspberry.local:9090", "Address of the TV RPC server")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: tv-client [-server address] <start|stop>")
		os.Exit(1)
	}

	command := args[0]
	var rpcMethod string

	switch command {
	case "start":
		rpcMethod = "TVService.Start"
	case "stop":
		rpcMethod = "TVService.Stop"
	default:
		fmt.Printf("Unknown command: %s. Use 'start' or 'stop'.\n", command)
		os.Exit(1)
	}

	fmt.Printf("Connecting to RPC server at %s...\n", *serverAddr)
	client, err := rpc.DialHTTP("tcp", *serverAddr)
	if err != nil {
		log.Fatalf("Failed to connect to RPC server: %v", err)
	}

	var reply string
	fmt.Printf("Sending %s command...\n", command)
	err = client.Call(rpcMethod, &struct{}{}, &reply)
	if err != nil {
		log.Fatalf("RPC error: %v", err)
	}

	fmt.Printf("Success: %s\n", reply)
}
