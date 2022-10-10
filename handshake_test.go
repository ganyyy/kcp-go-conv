package kcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
