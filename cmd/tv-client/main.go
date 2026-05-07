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

	var conn net.Conn
	var err error

	// Helper to dial with retries
	dialWithRetry := func(network, addr string, attempts int) (net.Conn, error) {
		var lastErr error
		for i := 0; i < attempts; i++ {
			if i > 0 {
				fmt.Printf("  (Retry %d/%d after error: %v)\n", i, attempts-1, lastErr)
				time.Sleep(1 * time.Second) // Longer backoff
			}
			c, err := net.DialTimeout(network, addr, 2*time.Second)
			if err == nil {
				return c, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}

	// 1. Try the primary address
	conn, err = dialWithRetry("tcp", *serverAddr, 3)
	
	// 2. Fallback to direct IP if primary (raspberry.local) fails
	if err != nil && *serverAddr == "raspberry.local:8080" {
		fallbackIP := "192.168.0.234:8080"
		fmt.Printf("Warning: Connection to %s failed. Trying fallback IP %s...\n", *serverAddr, fallbackIP)
		conn, err = dialWithRetry("tcp4", fallbackIP, 5) // More attempts and forced tcp4
	}

	if err != nil {
		fmt.Printf("\nError: Failed to connect to %s\n", *serverAddr)
		fmt.Printf("Underlying error: %v\n", err)
		
		// Debug: check for proxy environment variables
		for _, env := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
			if val := os.Getenv(env); val != "" {
				fmt.Printf("Note: Proxy detected: %s=%s\n", env, val)
			}
		}
		
		fmt.Println("\nPossible reasons:")
		fmt.Println("  1. The Raspberry Pi is offline or has a new IP.")
		fmt.Println("  2. A firewall (Mac or Pi) is blocking port 8080.")
		fmt.Println("  3. The tv-bot is not running on the Pi.")
		os.Exit(1)
	}
	
	if localAddr := conn.LocalAddr(); localAddr != nil {
		fmt.Printf("Connected using local address: %s\n", localAddr.String())
	}

	// Handshake for DialHTTP: send "CONNECT /_goRPC_ HTTP/1.0\n\n"
	io.WriteString(conn, "CONNECT " + rpc.DefaultRPCPath + " HTTP/1.0\n\n")

	// Read the response "HTTP/1.0 200 Connected to Go RPC\n\n"
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err != nil || resp.Status != "200 Connected to Go RPC" {
		conn.Close()
		fmt.Printf("\nError: Failed to establish RPC handshake over HTTP\n")
		if err != nil {
			fmt.Printf("Underlying error: %v\n", err)
		} else {
			fmt.Printf("Unexpected HTTP response: %s\n", resp.Status)
		}
		os.Exit(1)
	}

	client := rpc.NewClient(conn)


	var reply string
	fmt.Printf("Sending %s command...\n", command)
	err = client.Call(rpcMethod, &struct{}{}, &reply)
	if err != nil {
		log.Fatalf("RPC error: %v", err)
	}

	fmt.Printf("Success: %s\n", reply)
}
