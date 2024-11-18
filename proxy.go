package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:./proxy<port>")
	}
	port := os.Args[1]
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Error starting proxy:", err)
	}
	defer listener.Close()
	fmt.Println("Proxy is running on port:", port)

	for {
		conn, err := listener.Accept() //accept connections from clients
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go handleConn(conn) //use goroutine handle connections
	}
}
func parseRequestLine(requestLine string) (string, string, string) {
	parts := strings.Fields(requestLine)
	if len(parts) != 3 {
		return "", "", ""
	}
	return parts[0], parts[1], parts[2]
}
func handleConn(conn net.Conn) {
	defer conn.Close()              //ensure connection close when function end accidentally
	reader := bufio.NewReader(conn) //read request body
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading request line:", err)
		return
	}
	method, url, protocol := parseRequestLine(requestLine)
	if method == "" || url == "" || protocol == "" {
		writeErrorResponse(conn, 400, "Bad Request")
		return
	}
	if method != "GET" {
		writeErrorResponse(conn, 501, "Not Implemented")
		return
	}

	//foward requesr to remote server
	resp, err := forwardRequest(method, url, reader)
	if err != nil {
		fmt.Println("Error forwarding request", err)
		writeErrorResponse(conn, 500, "Internal Server Error")
	}
	defer resp.Body.Close()

	//write the response back to the client
	writeResponse(conn, resp)
}
func forwardRequest(method string, url string, reader *bufio.Reader) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			//the flag EOF means end
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if line == "\r\n" {
			break
		}
		headerParts := strings.SplitN(line, ":", 2)
		if len(headerParts) == 2 {
			req.Header.Add(strings.TrimSpace(headerParts[0]), strings.TrimSpace(headerParts[1]))
		}

	}
	//forward request and return response
	return client.Do(req)
}
func writeResponse(conn net.Conn, resp *http.Response) {
	//write status line
	fmt.Fprintf(conn, "%s %d %s\r\n", resp.Proto, resp.StatusCode, resp.Status)

	//write headers
	for key, values := range resp.Header {
		for _, value := range values {
			fmt.Fprintf(conn, "%s:%s\r\n", key, value)
		}
	}
	fmt.Fprint(conn, "\r\n")
	//write body
	io.Copy(conn, resp.Body)
}
func writeErrorResponse(conn net.Conn, statusCode int, message string) {
	fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\n\r\n%s\r\n", statusCode, message, message)
}
