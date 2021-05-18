package main

import (
	"fmt"
	"github.com/Allenxuxu/ringbuffer"
	"golang.org/x/sys/unix"
	"log"
)

type Connection struct {
	fd      int
	readBuf *ringbuffer.RingBuffer
}

func OnMsg(c *Connection, data []byte) []byte {
	fmt.Println("accept 消息:" + string(data))
	return nil
}

func NewConn(fd int) *Connection {
	return &Connection{
		fd:      fd,
		readBuf: ringbuffer.New(1024),
	}
}

func (c *Connection) handleRead() {
	tmpBuf := make([]byte, 1024)
	n, err := unix.Read(c.fd, tmpBuf)
	if err != nil {
		log.Fatal(err)
	}
	c.readBuf.Write(tmpBuf[:n])
	for c.readBuf.Length() > 4 {
		len := c.readBuf.PeekUint32()
		cmdData := make([]byte, len)
		_, err = c.readBuf.Read(cmdData)
		if err != nil {
			log.Fatal(err)
		}
		OnMsg(c, cmdData)
	}
}

func (c *Connection) handleWrite() {

}
