package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

const (
	HTTP_PROTOCOL                     = "HTTP/1.1"
	HTTP_STATUS_OK                    = "HTTP/1.1 200 OK\r\n\r\n"
	HTTP_STATUS_NOT_FOUND             = "HTTP/1.1 404 Not Found\r\n\r\n"
	HTTP_STATUS_CREATED               = "HTTP/1.1 201 Created\r\n\r\n"
	HTTP_STATUS_INTERNAL_SERVER_ERROR = "HTTP/1.1 500 Internal Server Error\r\n\r\n"
	GET                               = "GET"
	POST                              = "POST"
)

type Request struct {
	Method    string
	Protocol  string
	Path      string
	Host      string
	UserAgent string
	Body      string
}

type Response struct {
	Protocol    string
	Status      string
	ContentType string
	Body        string
}

type Flags struct {
	ServerDirectory string
}

// Flag parsing function
func GetFlags() Flags {

	// define flags
	filePath := flag.String("directory", "", "Static site directory")

	// parse flage
	flag.Parse()

	// put flags into Flags struct
	flags := Flags{
		ServerDirectory: *filePath,
	}

	return flags
}

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Println("Logs from your program will appear here!")

	// get static site directory from flag
	flags := GetFlags()
	fileDir := flags.ServerDirectory

	// set up listener
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	defer l.Close()

	// accept connections and pass to handler
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		// handle requests in go routin
		go handleRequest(conn, fileDir)
	}
}

func handleRequest(conn net.Conn, fileDir string) {
	defer conn.Close()

	for {

		data := make([]byte, 1024)
		numBytes, err := conn.Read(data)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		var resp Response       // holds response data
		var req Request         // holds request data
		var fullResponse string // full response to send to client

		// parse request and store in Request truct
		parseRequest(string(data[:numBytes]), &req)

		// Test if site root directory or something else
		if req.Path == "/" {
			fullResponse = HTTP_STATUS_OK
			conn.Write([]byte(fullResponse))
			return
		} else {
			splitPath := parseRequestPath(req.Path)[1:]
			if len(splitPath) > 2 {
				conn.Write([]byte("HTTP/1.1 Invalid Request\r\n\r\n"))
				break
			}
			switch splitPath[0] {
			case "echo":
				resp.Protocol = HTTP_PROTOCOL
				resp.Status = "200 OK"
				resp.ContentType = "text/plain"
				resp.Body = splitPath[1]
			case "user-agent":
				resp.Protocol = HTTP_PROTOCOL
				resp.Status = "200 OK"
				resp.ContentType = "text/plain"
				resp.Body = strings.Split(req.UserAgent, " ")[1]
			case "files":
				fileName := splitPath[1]
				if req.Method == GET {
					respBody, err := os.ReadFile(fileDir + "/" + fileName)
					if err != nil {
						fullResponse = HTTP_STATUS_NOT_FOUND
						conn.Write([]byte(fullResponse))
						return
					}
					resp.Protocol = HTTP_PROTOCOL
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

	// put the rest into headers array
	headers := splitRequest[1:]

	// assign to struct
	r.Method = req[0]
	r.Protocol = req[2]
	r.Path = req[1]
	r.Host = headers[0]
	r.UserAgent = headers[1]
	r.Body = strings.Replace(headers[len(headers)-1], "\x00", "", -1)
}

// splits path on "/""
func parseRequestPath(request string) []string {
	return strings.Split(request, "/")
}

// constructs response
func createResponse(r Response) string {
	responseString := fmt.Sprintf("%s %s\r\nContent-Type: %s\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", r.Protocol, r.Status, r.ContentType, len(r.Body), r.Body)
	return responseString
}
