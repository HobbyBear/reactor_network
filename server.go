package main

import (
	"golang.org/x/sys/unix"
	"log"
	"net"
	"reactor_network/poll"
)

type Server struct {
	ListenerPoll *poll.Poll
	// todo 做成数组
	WPoll *poll.Poll
}

func New(network string, addr string) *Server {
	var (
		listener net.Listener
		err      error
		s        *Server
	)
	s = &Server{
		ListenerPoll: poll.Create(),
		WPoll:        poll.Create(),
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

			s.WPoll.AddReadEvent(nfd)
		}
	})

	// connection 对数据进行读写
	go s.WPoll.RunLoop(func(fd int, events poll.Event) {
		if events&poll.EventRead != 0 {
			buf := make([]byte, 1024)
			// todo 拆包
			n, err := unix.Read(fd, buf)
			if err != nil {
				log.Fatal(err)
			}

			// todo 粘包
			_, err = unix.Write(fd, buf[:n])
			if err != nil {
				log.Fatal(err)
			}
		}

		if events&poll.EventErr != 0 {
			err := unix.Close(fd)
			if err != nil {
				log.Fatal(err)
			}
		}

		if events&poll.EventWrite != 0 {

		}

	})
}
