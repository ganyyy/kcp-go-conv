package kcp

import (
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
)

const (
	handshakePacketSize = 20 // 握手包大小
	handshakeMaxWaiter  = 20 // 握手包最多的等待数量

	maxConvRandCnt = 1000 // ?
)

const (
	codeConnect    = 255 // Connect
	codeConnectRsp = 325 // ConnectRsp
	codeDisconnect = 404 // Disconnect
)

const (
	numConnectRsp    = 0x14514545
	numDisconnectReq = 0x19419494
)

const (
	enetTimeout                = 0
	enetClientClose            = 1
	enetClientRebindFail       = 2
	enetClientShutdown         = 3
	enetServerRelogin          = 4
	enetServerKick             = 5
	enetServerShutdown         = 6
	enetNotFoundSession        = 7
	enetLoginUnfinished        = 8
	enetPacketFreqTooHigh      = 9
	enetPingTimeout            = 10
	enetTranferFailed          = 11
	enetServerKillClient       = 12
	enetCheckMoveSpeed         = 13
	enetAccountPasswordChange  = 14
	enetClientEditorConnectKey = 987654321
	enetClientConnectKey       = 1234567890
)

const (
	debugLog = false
)

type handshakeWaiter []*waiter

type waitFilter func(*waiter) bool

func dLog(format string, args ...any) {
	if !debugLog {
		return
	}
	log.Println("[KCP]", fmt.Sprintf(format, args...))
}

func (h *handshakeWaiter) getWait(filter waitFilter) *waiter {
	waiter := *h
	for _, w := range waiter {
		if filter(w) {
			return w
		}
	}
	return nil
}

func (h *handshakeWaiter) removeWaitConv(conv uint64) {
	waiter := *h
	for i, w := range waiter {
		if w.conv == conv {
			waiter = append(waiter[:i], waiter[i+1:]...)
		}
	}
	*h = waiter
}

func (h *handshakeWaiter) addNewWait(w waiter) {
	waiter := *h
	if len(waiter) >= handshakeMaxWaiter {
		copy(waiter, waiter[1:])
		waiter = waiter[:handshakeMaxWaiter-1]
	}
	waiter = append(waiter, &w)
	*h = waiter
}

func (h *handshakeWaiter) getWaitAddr(addr string) *waiter {
	return h.getWait(func(w *waiter) bool {
		return w.addr == addr
	})
}

func (h *handshakeWaiter) getWaitConv(conv uint64) *waiter {
	return h.getWait(func(w *waiter) bool {
		return w.conv == conv
	})
}

func generateConvId() uint64 {
	return rand.Uint64()
}

var (
	endian = binary.LittleEndian
)

type waiter struct {
	conv uint64
	addr string
}

type handshakePkt struct {
	code uint32
	enet uint32
	num  uint32
	conv uint64
}

func (h *handshakePkt) send(conn net.PacketConn, addr net.Addr) {
	dLog("%v Pkt send to %v: %+v", conn.LocalAddr(), addr, h)
	// TODO 异步化?
	buf := xmitBuf.Get().([]byte)[:handshakePacketSize]
	h.marshal(buf)
	_, err := conn.WriteTo(buf, addr)
	xmitBuf.Put(buf)
	if err != nil {
		//TODO 错误处理
		_ = err
	}
}

type handshaleData [handshakePacketSize]byte

func (h *handshaleData) code() []byte { return h[:4] }
func (h *handshaleData) conv() []byte { return h[4:12] }
func (h *handshaleData) enet() []byte { return h[12:16] }
func (h *handshaleData) num() []byte  { return h[16:] }

func (h *handshakePkt) marshal(bs []byte) {
	d := (*handshaleData)(bs)
	endian.PutUint32(d.code(), h.code)
	endian.PutUint64(d.conv(), h.conv)
	endian.PutUint32(d.enet(), h.enet)
	endian.PutUint32(d.num(), h.num)
}

func (pkt *handshakePkt) unmarshal(bs []byte) {
	data := (*(*[handshakePacketSize]byte)(bs))
	d := handshaleData(data)
	pkt.code = endian.Uint32(d.code())
	pkt.conv = endian.Uint64(d.conv())
	pkt.enet = endian.Uint32(d.enet())
	pkt.num = endian.Uint32(d.num())
}

func (l *Listener) handshake(data []byte, addr net.Addr) (ret bool) {
	if len(data) != handshakePacketSize {
		return false
	}
	// 不管怎么说, 这一定是握手包了
	ret = true

	var pkt handshakePkt
	pkt.unmarshal(data)

	dLog("%v Listener.handshake %v Input:%+v", l.conn.LocalAddr(), addr, pkt)

	if pkt.code == codeConnect {
		// 链接code需要使用服务器创建的conv Id
		conv, valid := l.getConv(addr)
		if !valid {
			//TODO 错误日志
			return
		}
		pkt.conv = conv
	}

	l.handlePkt(pkt, addr)
	return
}

func (l *Listener) getConv(addr net.Addr) (conv uint64, valid bool) {
	sAddr := addr.String()
	w := l.getWaitAddr(sAddr)
	if w == nil {
		// 创建新的
		n := maxConvRandCnt
		for n > 0 {
			conv = generateConvId()
			if _, ok := l.getSession(conv); !ok && l.getWaitConv(conv) == nil {
				break
			}
			n--
		}
		if n == 0 {
			return // 找不到合适的conv(?)
		}
		l.addNewWait(waiter{conv: conv, addr: sAddr})
	} else {
		// 使用旧的
		conv = w.conv
	}
	valid = true
	return
}

func (l *Listener) handlePkt(pkt handshakePkt, addr net.Addr) {
	switch pkt.code {
	case codeConnect:
		pkt.code = codeConnectRsp
		pkt.num = numConnectRsp
		pkt.send(l.conn, addr)
	case codeDisconnect:
		ses, ok := l.getSession(pkt.conv)
		if pkt.num != numDisconnectReq {
			return
		}
		if ok {
			dLog("%v close %v conn", l.conn.LocalAddr(), addr)
			ses.Close()
		}
	default:
		//TODO 错误日志
	}
}

func (s *UDPSession) handshake(data []byte) (ret bool) {
	if len(data) != handshakePacketSize {
		return false
	}
	// 不管怎么说, 这一定是握手包了
	ret = true
	var pkt handshakePkt
	pkt.unmarshal(data)
	dLog("UDPSession.handshake Input:%+v", pkt)
	if pkt.code != codeConnectRsp {
		return
	}
	if pkt.num != numConnectRsp {
		return
	}
	s.kcp.conv = pkt.conv
	return
}

func (s *UDPSession) handshakeSendConnect() {
	var pkt handshakePkt
	pkt.code = codeConnect
	pkt.enet = enetClientConnectKey
	pkt.send(s.conn, s.RemoteAddr())
}

func (s *UDPSession) handshakeSendDisconnect() {
	var pkt handshakePkt
	pkt.code = codeDisconnect
	pkt.conv = uint64(s.GetConv())
	pkt.enet = enetServerKick
	pkt.num = numDisconnectReq
	pkt.send(s.conn, s.RemoteAddr())
}
