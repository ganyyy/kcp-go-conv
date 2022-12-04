package main

import (
	"io"
	"log"
	"math/rand"
	"time"

	"github.com/xtaci/kcp-go/v5"
)

func main() {
	if listener, err := kcp.ListenWithOptions("127.0.0.1:12345", nil, 0, 0); err == nil {
		// spin-up the client
		go func() {
			for i := 0; i < 5; i++ {
				go client()
			}
		}()
		for {
			s, err := listener.AcceptKCP()
			if err != nil {
				log.Fatal(err)
			}
			go handleEcho(s)
		}
	} else {
		log.Fatal(err)
	}
}

// handleEcho send back everything it received
func handleEcho(conn *kcp.UDPSession) {
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Println("Read", conn.RemoteAddr(), err)
			return
		}

		_, err = conn.Write(buf[:n])
		if err != nil {
			log.Println("Write", conn.RemoteAddr(), err)
			return
		}
	}
}

func client() {

	// wait for server to become ready
	time.Sleep(time.Second)

	// dial to the echo server
	if sess, err := kcp.Dial("127.0.0.1:12345", kcp.WithDialTimeout(time.Second*3)); err == nil {
		for {
			data := time.Now().String()
			buf := make([]byte, len(data))
			log.Println(sess.LocalAddr(), "sent:", data)
			if _, err := sess.Write([]byte(data)); err == nil {
				// read back the data
				if _, err := io.ReadFull(sess, buf); err == nil {
					log.Println(sess.LocalAddr(), "recv:", string(buf))
				} else {
					log.Fatal(err)
				}
			} else {
				log.Fatal(err)
			}
			if rand.Intn(100) > 80 {
				sess.Close()
				break
			}
			time.Sleep(time.Second)
		}
	} else {
		log.Fatal(err)
	}
}
