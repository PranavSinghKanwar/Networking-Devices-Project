package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

type Server interface {
	Address() string
	IsAlive() bool
	Serve(rw http.ResponseWriter, req *http.Request)
}

type simpleServer struct {
	address string
	proxy   *httputil.ReverseProxy
}

type LoadBalancer struct {
	port            string
	roundRobinCount int
	servers         []Server
}

func NewLoadBalancer(port string, servers []Server) *LoadBalancer {
	return &LoadBalancer{
		port:            port,
		roundRobinCount: 0,
		servers:         servers,
	}
}

func newSimpleServer(address string) *simpleServer {
	serverUrl, err := url.Parse(address)
	handlerErr(err)
	return &simpleServer{
		address: address,
		proxy:   httputil.NewSingleHostReverseProxy(serverUrl),
	}
}

func handlerErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func (s *simpleServer) Address() string {
	return s.address
}

func (s *simpleServer) IsAlive() bool {
	resp, err := http.Get(s.address)
	if err != nil {
		fmt.Printf("[Health Check] Server %s - Error: %s\n", s.address, err.Error())
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true
	}

	fmt.Printf("[Health Check] Server %s - Status Code: %d\n", s.address, resp.StatusCode)
	return false
}

func (s *simpleServer) Serve(rw http.ResponseWriter, req *http.Request) {
	s.proxy.ServeHTTP(rw, req)
}

func (lb *LoadBalancer) getNextAvailableServer() Server {
	server := lb.servers[lb.roundRobinCount%len(lb.servers)]
	fmt.Printf("[Server Check] Checking server %s\n", server.Address())

	for !server.IsAlive() {
		fmt.Printf("[Server Check] Server %s is not alive\n", server.Address())
		lb.roundRobinCount++
		server = lb.servers[lb.roundRobinCount%len(lb.servers)]
	}
	fmt.Printf("[Server Check] Server %s is alive\n", server.Address())
	lb.roundRobinCount++
	return server
}

func (lb *LoadBalancer) serveProxy(rw http.ResponseWriter, req *http.Request) {
	lb.roundRobinCount++
	server := lb.getNextAvailableServer()
	fmt.Printf("[Request Forwarding] Forwarding request to %s\n", server.Address())
	if server != nil {
		server.Serve(rw, req)
		return
	}
	http.Error(rw, "Service not available", http.StatusServiceUnavailable)
}

// func (lb *LoadBalancer) displayServerHealthAndOrder() {
// 	fmt.Println("[Server Health Status]")
// 	for _, server := range lb.servers {
// 		if server.IsAlive() {
// 			fmt.Printf("Server %s is alive\n", server.Address())
// 		} else {
// 			fmt.Printf("Server %s is not alive\n", server.Address())
// 		}
// 	}

// 	fmt.Println("\nList of available servers along with their healths:")
// 	for i := 0; i < len(lb.servers); i++ {
// 		fmt.Printf("%d. %s\n", i+1, lb.servers[(lb.roundRobinCount+i)%len(lb.servers)].Address())
// 	}
// }
func (lb *LoadBalancer) displayServerHealthAndOrder() {
	fmt.Println("[Server Health Status]")
	for _, server := range lb.servers {
		status := "not alive"
		if server.IsAlive() {
			status = "alive"
		}
		fmt.Printf("Server %s is %s\n", server.Address(), status)
	}
}

func main() {
	servers := []Server{
		newSimpleServer("https://example.com"),
		newSimpleServer("https://jsonplaceholder.typicode.com"),
		newSimpleServer("https://api.publicapis.org"),
		newSimpleServer("https://dog.ceo/api/breeds/list/all"),
		newSimpleServer("https://nonexistentwebsite123.com"), // Non-existent server
	}

	lb := NewLoadBalancer(":8081", servers)
	handleRedirect := func(rw http.ResponseWriter, req *http.Request) {
		lb.serveProxy(rw, req)
	}

	http.HandleFunc("/", handleRedirect)
	fmt.Printf("Server started at port %s\n", lb.port)
	lb.displayServerHealthAndOrder()
	http.ListenAndServe(lb.port, nil)
}
