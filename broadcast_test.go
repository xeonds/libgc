package libgc_test

import (
	"fmt"
	"libgc/lib"
	"testing"
)

func TestBroadcast(t *testing.T) {
	ClientID := lib.GetLocalIP()
	ClientPort := lib.RandPort()
	fmt.Printf("Client ID: %s\n", ClientID)
	fmt.Printf("Client Port: %d\n", ClientPort)

	go lib.StartBroadcast()
	lib.StartListening()
}
