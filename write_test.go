package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"testing"
	"time"
)

func TestNewConn(t *testing.T) {
	go data()
	time.Sleep(1 * time.Second)
	go data()

	time.Sleep(10 * time.Second)
}

func data() {
	conn, err := net.Dial("tcp", "127.0.0.1:8080")
	if err != nil {
		log.Fatal(err)
	}
	data := new(bytes.Buffer)
	str := "hello world"
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(len(str)))
	data.Write(append(buf, []byte(str)...))

	_, err = conn.Write(data.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	receiveData := make([]byte, 100)
	time.Sleep(500 * time.Millisecond)
	n, err := conn.Read(receiveData)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("收到的数据为：", string(receiveData[:n]))
	conn.Close()
	time.Sleep(5 * time.Second)
}
