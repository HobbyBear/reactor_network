package main

import (
	"bytes"
)

type Connection struct {
	readBuf bytes.Buffer
}

func OnMsg(c Connection, data []byte) []byte {
	return nil
}

func NewConn() *Connection {

	return nil
}

func (c *Connection) handleRead(fd int) {
}

func (c *Connection) handleWrite() {

}
