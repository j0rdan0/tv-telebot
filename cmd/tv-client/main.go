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
		fmt.Printf("\n❌ Error: Failed to connect to RPC server at %s\n", *serverAddr)
		fmt.Println("Possible reasons:")
		fmt.Println("  1. The TV Bot/RPC server is not running on the Raspberry Pi.")
		fmt.Println("  2. A firewall is blocking port 9090 on the Raspberry Pi.")
		fmt.Println("  3. 'raspberry.local' is not resolving correctly to the current IP.")
		fmt.Printf("\nTry using the IP address directly: ./tv-client -server 192.168.0.234:9090 %s\n", command)
		os.Exit(1)
	}

	var reply string
	fmt.Printf("Sending %s command...\n", command)
	err = client.Call(rpcMethod, &struct{}{}, &reply)
	if err != nil {
		log.Fatalf("RPC error: %v", err)
	}

	fmt.Printf("Success: %s\n", reply)
}
