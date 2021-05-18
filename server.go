package main

import (
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"net"
	"reactor_network/poll"
)

type Server struct {
	ListenerPoll *poll.Poll
	// todo 做成数组
	WPoll          *poll.Poll
	ConnectionPoll map[int]*Connection
}

func New(network string, addr string) *Server {
	var (
		listener net.Listener
		err      error
		s        *Server
	)
	s = &Server{
		ListenerPoll:   poll.Create(),
		WPoll:          poll.Create(),
		ConnectionPoll: make(map[int]*Connection),
	}
	listener, err = net.Listen(network, addr)
	l, _ := listener.(*net.TCPListener)

	file, err := l.File()
	if err != nil {
		log.Fatal(err)
	}
	fd := int(file.Fd())
	if err = unix.SetNonblock(fd, true); err != nil {
		log.Fatal(err)
	}
	s.ListenerPoll.AddReadEvent(fd)

	return s
}

func (s *Server) Start() {
	go s.ListenerPoll.RunLoop(func(fd int, events poll.Event) {
		if events&poll.EventRead != 0 {
			nfd, _, err := unix.Accept(fd)
			if err != nil {
				if err != unix.EAGAIN {
					log.Fatal("accept:", err)
				}
				return
			}
			if err := unix.SetNonblock(nfd, true); err != nil {
				_ = unix.Close(nfd)
				log.Fatal("set nonblock:", err)
				return
			}
			conn := NewConn(nfd)
			s.ConnectionPoll[nfd] = conn
			log.Println(fmt.Sprintf("%d 三次握手完成", nfd))
			s.WPoll.AddReadEvent(nfd)
		}
	})

	// connection 对数据进行读写
	go s.WPoll.RunLoop(func(fd int, events poll.Event) {
		log.Println("sub reactor receive data", fd, events&poll.EventRead, events&poll.EventWrite)

		if events&poll.EventRead != 0 {
			if conn, ok := s.ConnectionPoll[fd]; ok {
				conn.handleRead()
				if conn.writeBuf.Len() > 0 {
					s.WPoll.EnableWriteEvent(fd)
				}
			}
		}

		if events&poll.EventErr != 0 {
			// todo conn closed
			err := unix.Close(fd)
			if err != nil {
				log.Fatal(err)
			}
			log.Println(fmt.Sprintf("%d 关闭了链接", fd))
		}

		if events&poll.EventWrite != 0 {
			if conn, ok := s.ConnectionPoll[fd]; ok {
				if conn.writeBuf.Len() > 0 {
					conn.handleWrite()
				}
				s.WPoll.EnableRead(fd)
			}
		}

	})
}
