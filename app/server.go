package main

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

const (
	HTTP_STATUS_OK                    = "HTTP/1.1 200 OK\r\n\r\n"
	HTTP_STATUS_NOT_FOUND             = "HTTP/1.1 404 Not Found\r\n\r\n"
	HTTP_STATUS_CREATED               = "HTTP/1.1 201 Created\r\n\r\n"
	HTTP_STATUS_INTERNAL_SERVER_ERROR = "HTTP/1.1 500 Internal Server Error\r\n\r\n"
	GET                               = "GET"
	POST                              = "POST"
)

type Request struct {
	Method   string
	Protocol string
	Path     string
	Body     string
	Headers  map[string]string
}

type Response struct {
	Protocol    string
	Status      string
	ContentType string
	Body        string
	Encoding    string
}

type Flags struct {
	ServerDirectory string
}

// Flag parsing function
func GetFlags() Flags {
	// define flags
	filePath := flag.String("directory", "", "Static site directory")

	// parse flags
	flag.Parse()

	// put flags into Flags struct
	flags := Flags{
		ServerDirectory: *filePath,
	}

	return flags
}

func main() {
	// get flags
	flags := GetFlags()
	fileDir := flags.ServerDirectory

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		go handleRequest(conn, fileDir)
	}
}

func handleRequest(conn net.Conn, fileDir string) {
	defer conn.Close()

	for {
		data := make([]byte, 1024)
		numBytes, err := conn.Read(data)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Connection closed by client")
				return
			}
			log.Fatal(err)
		}

		var resp Response       // holds response data
		var req Request         // holds request data
		var fullResponse string // full response to be sent to client

		parseRequest(string(data[:numBytes]), &req)

		// Test if site root directory or something else
		if req.Path == "/" {
			fullResponse = HTTP_STATUS_OK
			conn.Write([]byte(fullResponse))
		} else {
			splitPath := parseRequestPath(req.Path)[1:]
			if len(splitPath) > 2 {
				conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
				return
			}
			switch splitPath[0] {
			case "echo":
				encoding, ok := req.Headers["Accept-Encoding"]
				if ok && strings.Contains(encoding, "gzip") {
					resp.Encoding = "gzip"
				}

				resp.Body = splitPath[1]
				resp.Protocol = "HTTP/1.1"
				resp.Status = "200 OK"
				resp.ContentType = "text/plain"
			case "user-agent":
				resp.Protocol = "HTTP/1.1"
				resp.Status = "200 OK"
				resp.ContentType = "text/plain"
				resp.Body = req.Headers["User-Agent"]
			case "files":
				fileName := splitPath[1]
				if req.Method == GET {
					respBody, err := os.ReadFile(fileDir + "/" + fileName)
					if err != nil {
						fullResponse = HTTP_STATUS_NOT_FOUND
						conn.Write([]byte(fullResponse))
						return
					}
					resp.Protocol = "HTTP/1.1"
					resp.Status = "200 OK"
					resp.ContentType = "application/octet-stream"
					resp.Body = string(respBody)
				} else if req.Method == POST {
					err := os.WriteFile(fileDir+"/"+fileName, []byte(req.Body), 0664)
					if err != nil {
						fullResponse = HTTP_STATUS_INTERNAL_SERVER_ERROR
						conn.Write([]byte(fullResponse))
						return
					}
					fullResponse = HTTP_STATUS_CREATED
					conn.Write([]byte(fullResponse))
					return
				} else {
					fullResponse = HTTP_STATUS_NOT_FOUND
					conn.Write([]byte(fullResponse))
					return
				}
			default:
				fullResponse = HTTP_STATUS_NOT_FOUND
				conn.Write([]byte(fullResponse))
				return
			}
		}

		// send response to client
		fullResponse = createResponse(resp)
		conn.Write([]byte(fullResponse))
	}
}

// parses full request and
func parseRequest(fullReq string, r *Request) {
	// split full request on "\r\n"
	splitRequest := strings.Split(fullReq, "\r\n")

	// grab the request itself
	req := strings.Split(splitRequest[0], " ")

	// map to hold header values
	headers := make(map[string]string)

	// grab the body
	body := splitRequest[len(splitRequest)-1]

	// parse headers into map[string]string
	for _, header := range splitRequest[1 : len(splitRequest[1:])-1] {
		line := strings.Split(header, ": ")
		headers[line[0]] = line[1]
	}

	// assign parsed values to Response struct
	r.Method = req[0]
	r.Protocol = req[2]
	r.Path = req[1]
	r.Body = strings.Replace(body, "\x00", "", -1)
	r.Headers = headers
}

// split path on "/""
func parseRequestPath(request string) []string {
	return strings.Split(request, "/")
}

// construct response
func createResponse(r Response) string {
	if r.Encoding == "gzip" {
		body, err := gzipBody(r.Body)
		if err != nil {
			log.Fatal(err)
		}

		responseString := fmt.Sprintf("%s %s\r\nContent-Encoding: %s\r\nContent-Type: %s\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", r.Protocol, r.Status, r.Encoding, r.ContentType, len(body), body)
		return responseString
	}
	responseString := fmt.Sprintf("%s %s\r\nContent-Type: %s\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", r.Protocol, r.Status, r.ContentType, len(r.Body), r.Body)
	return responseString
}

func gzipBody(body string) (string, error) {
	var bodyBuffer bytes.Buffer
	zipper := gzip.NewWriter(&bodyBuffer)

	_, err := zipper.Write([]byte(body))
	if err != nil {
		return "", err
	}

	err = zipper.Close()
	if err != nil {
		return "", err
	}

	return bodyBuffer.String(), nil
}
