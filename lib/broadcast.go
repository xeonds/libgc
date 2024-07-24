package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Client struct {
	ID     string
	IP     string
	Port   int
	Status string
	Peers  map[string]*Client
	server *gin.Engine
}

func NewClient(ip string, port int) *Client {
	return &Client{
		ID:     fmt.Sprintf("%s:%d", ip, port),
		IP:     ip,
		Port:   port,
		Status: "",
		Peers:  make(map[string]*Client),
		server: gin.Default(),
	}
}

func (c *Client) StartDiscover() {
	// broadcast self's address every 2 seconds
	go func() {
		conn, err := net.Dial("udp", "255.255.255.255:9876")
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()

		for {
			if _, err := fmt.Fprintln(conn, c.ID); err != nil {
				log.Println("Error broadcasting:", err)
			}
			time.Sleep(2 * time.Second)
		}
	}()

	// listen to clients and add them to pool
	go func() {
		addr, err := net.ResolveUDPAddr("udp", ":9876")
		if err != nil {
			log.Fatal(err)
		}
		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		buf := make([]byte, 1024)
		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				log.Println("Error receiving:", err)
				continue
			}
			id := string(buf[:n])
			parts := strings.Split(id, ":")
			if len(parts) != 2 {
				continue
			}
			ip := parts[0]
			port, err := strconv.Atoi(parts[1])
			if err != nil {
				continue
			}
			// in case of mark self as new client
			if id == c.ID {
				continue
			}
			c.Peers[id] = &Client{
				ID:   id,
				IP:   ip,
				Port: port,
			}
		}
	}()
}

func (c *Client) Listen(path string, handler func(ctx *gin.Context, src *Client)) {
	c.server.POST(path, func(ctx *gin.Context) {
		handler(ctx, c)
	})
}

// send message to client, path should start with '/'
func (c *Client) Send(path string, content gin.H) (gin.H, error) {
	jsonData, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}
	url := fmt.Sprintf("http://%s%s", c.ID, path)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-OK response status: %s", resp.Status)
	}

	// 解析响应体为 gin.H
	var responseBody gin.H
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return responseBody, nil
}
