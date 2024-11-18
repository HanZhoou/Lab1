package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const maxClients = 10

var mimeTypes = map[string]string{
	".html": "text/html",
	".txt":  "text/plain",
	".gif":  "image/gif",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".css":  "text/css",
}

func main() {
	//get listen port from terminal input
	if len(os.Args) != 2 {
		fmt.Println("Usage:./http_server <port>")
	}
	var port = os.Args[1]

	//Start listening
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Starting server failed:", err)
		return
	}
	defer listener.Close()
	fmt.Println("Server is running on port:", port)

	//Concurrency control
	semaphore := make(chan struct{}, maxClients)
	var wg sync.WaitGroup

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		//Acquire a semaphore slot
		semaphore <- struct{}{}
		wg.Add(1)

		go func(conn net.Conn) {
			defer conn.Close()
			//Release semaphore
			defer func() { <-semaphore; wg.Done() }()

			handleConn(conn)
		}(conn)
	}
	//wg.Wait()
}
func parseRequestLine(requestLine string) (string, string, string) {
	parts := strings.Fields(requestLine)
	if len(parts) != 3 {
		return "", "", ""
	}
	return parts[0], parts[1], parts[2]
}

// EnsureDir ensures that the directory exists, creating it if necessary.
func ensureDir(filePath string) error {
	dir := filepath.Dir(filePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		//if directory not exists,create a new directory
		err = os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}
func parseHeaders(reader *bufio.Reader) (map[string]string, error) {
	headers := make(map[string]string)

	for {
		// read headers info by line
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		// header ended
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		// parse header value
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed header: %s", line)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		headers[key] = value
	}

	return headers, nil
}
func handleGET(conn net.Conn, path string) {
	// cwd, err := os.Getwd()
	// if err != nil {
	// 	fmt.Println("Error getting current working directory:", err)
	// 	return
	// }
	//filePath := filepath.Join(cwd, path)
	filePath := filepath.Join(".", path)
	ext := filepath.Ext(filePath)
	mimeType, supported := mimeTypes[ext]
	//return 400 for not supported filetypes
	if !supported {
		writeErrorResponse(conn, 400, "Bad Request")
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		writeErrorResponse(conn, 404, "Not Found")
		return
	}
	defer file.Close()

	//Write successful response
	conn.Write([]byte("HTTP/1.1 200 OK\r\n"))
	conn.Write([]byte("Content-Type: " + mimeType + "\r\n\r\n"))
	io.Copy(conn, file)
}

func handlePOST(conn net.Conn, reader *bufio.Reader, path string) {
	filePath := filepath.Join(".", path)
	ext := filepath.Ext(filePath)
	_, supported := mimeTypes[ext]
	//return 400 for not supported filetypes
	if !supported {
		writeErrorResponse(conn, 400, "Bad Request")
		return
	}
	err := ensureDir(filePath)
	if err != nil {
		fmt.Println("Error creating directory:", err) // Print error
		writeErrorResponse(conn, 500, "Internal Server Error")
		return
	}
	//parse requests headers
	header, err := parseHeaders(reader)
	if err != nil {
		fmt.Println("Error parsing headers:", err)
		writeErrorResponse(conn, 400, "Bad Request")
		return
	}
	contentLength, err := strconv.ParseInt(header["Content-Length"], 10, 64)
	if err != nil {
		fmt.Println("Error contentLength:", err)
		writeErrorResponse(conn, 500, "Internal Server Error")
	}
	//Limit the size of the reader to Content-Length
	limitedReader := &io.LimitedReader{
		R: reader,
		N: int64(contentLength), // limit the byte size of reader
	}

	fmt.Println("Content-Length:", contentLength)
	//write request body into file
	file, err := os.Create(filePath)
	if err != nil {
		//debug messages
		fmt.Println("Error copying data:", err)
		writeErrorResponse(conn, 500, "Internal Server Error")
	}
	defer file.Close()

	//write request body into file
	_, err = io.Copy(file, limitedReader)
	if err != nil {
		//debug messages
		fmt.Println("Error copying data:", err)
		writeErrorResponse(conn, 500, "Internal Server Error")
		return
	}

	writeSuccessResponse(conn, "File uploaded successfully")
	//debug messages
	fmt.Println("File uploaded successfully:", filePath)
}
func handleConn(conn net.Conn) {
	//read the request by line
	reader := bufio.NewReader(conn)
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		writeErrorResponse(conn, 400, "Bad Request")
		return
	}

	//Parse request
	method, path, version := parseRequestLine(requestLine)
	if method == "" || path == "" || version == "" {
		writeErrorResponse(conn, 400, "Bad Request")
		return
	}

	//handle Get and Post request
	switch method {
	case "GET":
		handleGET(conn, path)
	case "POST":
		handlePOST(conn, reader, path)
	default:
		writeErrorResponse(conn, 501, "Not Implemented") //return 501 for other methods
	}

}
func writeErrorResponse(conn net.Conn, statusCode int, message string) {
	response := fmt.Sprintf("HTTP/1.1 %d %s\r\n\r\n%s\r\n", statusCode, message, message)
	conn.Write([]byte(response))
}

func writeSuccessResponse(conn net.Conn, message string) {
	response := fmt.Sprintf("HTTP/1.1 200 OK\r\n\r\n%s\r\n", message)
	conn.Write([]byte(response))
}
