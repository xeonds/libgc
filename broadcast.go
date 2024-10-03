package libgc

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

var (
	ClientID   string
	ClientPort int
	Clients    = make(map[string]string) // id -> ip:port
)

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatal(err)
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func RandPort() int {
	return 10000 + rand.Intn(10000)
}

func StartBroadcast() {
	conn, err := net.Dial("udp", "255.255.255.255:9876")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	for {
		message := fmt.Sprintf("%s:%d", ClientID, ClientPort)
		_, err = fmt.Fprintln(conn, message)
		if err != nil {
			log.Println("Error broadcasting:", err)
		}
		time.Sleep(2 * time.Second)
	}
}

func StartListening() {
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

		message := string(buf[:n])
		handleBroadcast(message)
	}
}

func handleBroadcast(message string) {
	parts := strings.Split(message, ":")
	if len(parts) != 2 {
		return
	}

	id := parts[0]
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return
	}

	if id != ClientID {
		Clients[id] = fmt.Sprintf("%s:%d", id, port)
		fmt.Printf("Discovered client: %s at %s\n", id, Clients[id])
	}
}
