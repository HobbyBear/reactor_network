package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"golang.org/x/sys/unix"
	"log"
)

type Connection struct {
	fd       int
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
}

func OnMsg(c *Connection, data []byte) []byte {
	fmt.Println("accept 消息:" + string(data))
	return []byte("发送给客户端的消息")
}

func NewConn(fd int) *Connection {
	return &Connection{
		fd:       fd,
		readBuf:  bytes.NewBuffer([]byte{}),
		writeBuf: bytes.NewBuffer([]byte{}),
	}
}

func (c *Connection) handleRead() {
	tmpBuf := make([]byte, 1024)
	n, err := unix.Read(c.fd, tmpBuf)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("fd收到了可读请求", string(tmpBuf[:n]), n)
	if n == 0 {
		return
	}
	c.readBuf.Write(tmpBuf[:n])
	for c.readBuf.Len() > 4 {
		var length uint32
		err = binary.Read(c.readBuf, binary.BigEndian, &length)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("获取到的数据长度是", length)
		cmdData := make([]byte, length)
		n, err = c.readBuf.Read(cmdData)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("读取的数据长度是", n)
		out := OnMsg(c, cmdData)
		c.writeBuf.Write(out)
	}
}

func (c *Connection) handleWrite() {
	if c.writeBuf.Len() == 0 {
		return
	}
	log.Println("写入了数据：", c.writeBuf.String())
	_, err := unix.Write(c.fd, c.writeBuf.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	c.writeBuf.Reset()
}
