package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"time"
)

func main() {
	serverAddr := flag.String("server", "raspberry.local:8080", "Address of the TV RPC server")
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

	// Try to resolve the address to see what we are hitting
	host, _, _ := net.SplitHostPort(*serverAddr)
	ips, _ := net.LookupIP(host)
	if len(ips) > 0 {
		fmt.Printf("Resolved %s to %v\n", host, ips)
	}

	var client *rpc.Client
	var err error

	// Attempt to connect with a custom dialer to get better control
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	conn, err := dialer.Dial("tcp4", *serverAddr)
	if err != nil {
		fmt.Printf("\n❌ Error: Failed to dial %s\n", *serverAddr)
		fmt.Printf("Underlying error: %v\n", err)
		fmt.Println("\nPossible reasons:")
		fmt.Println("  1. The TV Bot/RPC server is not running on the Raspberry Pi.")
		fmt.Println("  2. A firewall is blocking port 8080 on the Raspberry Pi.")
		fmt.Println("  3. The Raspberry Pi is on a different network or the IP is wrong.")
		os.Exit(1)
	}

	// Handshake for DialHTTP: send "CONNECT /_goRPC_ HTTP/1.0\n\n"
	io.WriteString(conn, "CONNECT " + rpc.DefaultRPCPath + " HTTP/1.0\n\n")

	// Read the response "HTTP/1.0 200 Connected to Go RPC\n\n"
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err != nil || resp.Status != "200 Connected to Go RPC" {
		conn.Close()
		fmt.Printf("\n❌ Error: Failed to establish RPC handshake over HTTP\n")
		if err != nil {
			fmt.Printf("Underlying error: %v\n", err)
		} else {
			fmt.Printf("Unexpected HTTP response: %s\n", resp.Status)
		}
		os.Exit(1)
	}

	client = rpc.NewClient(conn)


	var reply string
	fmt.Printf("Sending %s command...\n", command)
	err = client.Call(rpcMethod, &struct{}{}, &reply)
	if err != nil {
		log.Fatalf("RPC error: %v", err)
	}

	fmt.Printf("Success: %s\n", reply)
}
