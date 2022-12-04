package kcp

import (
	"crypto/rand"
	rand2 "math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func init() {
	rand2.Seed(time.Now().UnixNano())
}

func TestHandshake(t *testing.T) {
	var pkt handshakePkt
	pkt.code = 100
	pkt.conv = 1000
	pkt.enet = 10000
	pkt.num = 100000

	bs := make([]byte, handshakePacketSize)
	pkt.marshal(bs)

	var newPkt handshakePkt
	newPkt.unmarshal(bs)

	assert.Equal(t, pkt, newPkt)
}

func TestHandleShakeWaiter(t *testing.T) {
	var w handshakeWaiter
	var buf [4]byte
	for i := 0; i < 100; i++ {
		for {
			uid := generateConvId()
			if w.getWaitConv(uid) != nil {
				continue
			}
			_, _ = rand.Read(buf[:])
			w.addNewWait(waiter{
				conv: uid,
				addr: string(buf[:]),
			})
			wait := w.getWaitConv(uid)
			assert.NotNil(t, wait)
			assert.Equal(t, w.getWaitAddr(wait.addr), wait)
			break
		}
	}
	assert.Equal(t, handshakeMaxWaiter, len(w))

	for len(w) > 0 {
		l := len(w)
		wait := w[rand2.Intn(len(w))]
		w.removeWaitConv(wait.conv)
		assert.Nil(t, w.getWaitConv(wait.conv))
		assert.Nil(t, w.getWaitAddr(wait.addr))
		assert.Equal(t, l-1, len(w))
	}
}
