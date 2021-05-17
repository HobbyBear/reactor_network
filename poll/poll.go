package poll

import (
	"fmt"
	"github.com/Allenxuxu/gev/log"
	"golang.org/x/sys/unix"
	"runtime"
)

// Event poller 返回事件
type Event uint32

// Event poller 返回事件值
const (
	EventRead  Event = 0x1
	EventWrite Event = 0x2
	EventErr   Event = 0x80
	EventNone  Event = 0
)

type SocketHandler interface {
	Handle(fd int, event Event) error
}

type Poll struct {
	fd               int
}

// init epoll
func Create() *Poll {
	fd, err := unix.Kqueue()
	if err != nil {
		log.Fatal(err)
	}

	return &Poll{
		fd: fd,
	}

}

func (p *Poll) RunLoop(handler func(fd int, event Event)) error {
	events := make([]unix.Kevent_t, 1024)
	var (
		ts  unix.Timespec
		tsp *unix.Timespec
	)
	for {
		n, err := unix.Kevent(p.fd, nil, events, tsp)
		if err != nil && err != unix.EINTR {
			log.Error("EpollWait: ", err)
			continue
		}
		if n <= 0 {
			tsp = nil
			runtime.Gosched()
			continue
		}
		tsp = &ts

		for i := 0; i < n; i++ {
			fd := int(events[i].Ident)
			if fd != 0 {
				//  如何保证read 是数据的到来，而不是握手信息的到来
				var rEvents Event
				if (events[i].Flags&unix.EV_ERROR != 0) || (events[i].Flags&unix.EV_EOF != 0) {
					rEvents |= EventErr
					fmt.Println(rEvents)
				}
				if events[i].Filter == unix.EVFILT_WRITE && events[i].Flags&unix.EV_ENABLE != 0 {
					rEvents |= EventWrite
				}
				if events[i].Filter == unix.EVFILT_READ && events[i].Flags&unix.EV_ENABLE != 0 {
					rEvents |= EventRead
				}
				handler(fd, rEvents)
			}
		}

		if n == len(events) {
			events = make([]unix.Kevent_t, n*2)
		}
	}

}

// add read
func (p *Poll) AddReadEvent(fd int) {
	kEvents := p.kEvents(EventNone, EventRead, fd)
	_, err := unix.Kevent(p.fd, kEvents, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func (p *Poll) kEvents(old Event, new Event, fd int) (ret []unix.Kevent_t) {
	if new&EventRead != 0 {
		if old&EventRead == 0 {
			ret = append(ret, unix.Kevent_t{Ident: uint64(fd), Flags: unix.EV_ADD | unix.EV_ENABLE, Filter: unix.EVFILT_READ})
		}
	} else {
		if old&EventRead != 0 {
			ret = append(ret, unix.Kevent_t{Ident: uint64(fd), Flags: unix.EV_DELETE | unix.EV_ONESHOT, Filter: unix.EVFILT_READ})
		}
	}

	if new&EventWrite != 0 {
		if old&EventWrite == 0 {
			ret = append(ret, unix.Kevent_t{Ident: uint64(fd), Flags: unix.EV_ADD | unix.EV_ENABLE, Filter: unix.EVFILT_WRITE})
		}
	} else {
		if old&EventWrite != 0 {
			ret = append(ret, unix.Kevent_t{Ident: uint64(fd), Flags: unix.EV_DELETE | unix.EV_ONESHOT, Filter: unix.EVFILT_WRITE})
		}
	}
	return
}
