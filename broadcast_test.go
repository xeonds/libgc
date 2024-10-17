package libgc_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xeonds/libgc"
)

func TestBroadcast(t *testing.T) {
	IP := libgc.GetLocalIP()
	Port1, Port2 := libgc.RandPort(), libgc.RandPort()
	fmt.Printf("Client IP: %s\n", IP)
	fmt.Printf("Client Port: %d, %d\n", Port1, Port2)

	c1, c2 := libgc.NewClient(IP, Port1), libgc.NewClient(IP, Port2)
	c1.Listen("/ping", func(ctx *gin.Context, src *libgc.Client) {
		ctx.JSON(200, gin.H{"port": src.ID})
	})
	c2.Listen("/ping", func(ctx *gin.Context, src *libgc.Client) {
		ctx.JSON(200, gin.H{"port": src.ID})
	})
	go c1.StartDiscover()
	go c2.StartDiscover()

	time.Sleep(time.Second * 5)
	if c1.Peers[c2.ID] == nil {
		t.Errorf("c1 should have c2 as a peer")
	}
	resp, err := c1.Peers[c2.ID].Send("/ping", nil)
	if err != nil {
		t.Errorf("c1 should be able to send to c2")
	}
	if resp["port"] != c2.ID {
		t.Errorf("c1 should receive c2's port")
	}
}
