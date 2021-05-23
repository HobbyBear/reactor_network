
## reacotor简述
在go里，传统的io模型是一个链接由一个协程去处理，但是epoll，kqueue出现后，可以通过一个协程通过epoll.wait ,Kevent 方法轮询获取客户端消息进行处理。

![截屏2021-05-23 上午2.40.33.png](https://p6-juejin.byteimg.com/tos-cn-i-k3u1fbpfcp/344ac0b223614a58a3d246daa9df62e5~tplv-k3u1fbpfcp-watermark.image)

reactor 模型就是两个fd ，一个main reactor ，一个sub reactor，main reactor 负责监听客户端的链接，然后收到后，将客户端的链接注册到sub reactor ，之后由sub reactor进行读写的处理。

## 启动示例
```go
type example struct {

}

func (e example) MsgCallBack(c *Connection, data []byte) []byte {
	fmt.Println(string(data))
	return []byte("收到了消息")
}


func main() {
	ch := make(chan int)
	s := New("tcp", ":8080",example{})
	s.Start()

	<-ch
}
```

## 设计思路
### server
我们的服务端程序，抽象为server，里面有main reactor ，和一个sub reactor集合， reactor包含一个epoll 或者kqueue的实例，用于监听读写事件。我在server里启动时会启动两个协程去分别对两个reactor的操作进行处理。


![截屏2021-05-23 上午3.05.58.png](https://p9-juejin.byteimg.com/tos-cn-i-k3u1fbpfcp/0e40f8dbfa854489bb48fa152c247241~tplv-k3u1fbpfcp-watermark.image)



```go
type Server struct {
	ListenerPoll *poll.Poll // 对kqueue的封装，用于监听链接
	// todo 做成数组
	WPoll          *poll.Poll  // subreactor封装，用于读写请求
	ConnectionPoll map[int]*Connection
	Handler        Handler  // 对消息进行处理的handler
}
func New(network string, addr string, h Handler) *Server {
	var (
		listener net.Listener
		err      error
		s        *Server
	)
	s = &Server{
		ListenerPoll:   poll.Create(),
		WPoll:          poll.Create(),
		ConnectionPoll: make(map[int]*Connection),
		Handler:        h,
	}
        // 监听事件
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
        // listend fd监听读事件
	s.ListenerPoll.AddReadEvent(fd)

	return s
}
func (s *Server) Start() {
        // 进行监听
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
                        
                       // 抽象一个链接
			conn := NewConn(nfd, s.Handler.MsgCallBack)
			s.ConnectionPoll[nfd] = conn
			log.Println(fmt.Sprintf("%d 三次握手完成", nfd))
                        // 注册读事件
			s.WPoll.AddReadEvent(nfd)
		}
	})

	// connection 对数据进行读写
	go s.WPoll.RunLoop(func(fd int, events poll.Event) {
		log.Println("sub reactor receive data", fd, events&poll.EventRead, events&poll.EventWrite)

		if events&poll.EventRead != 0 {
			if connection, ok := s.ConnectionPoll[fd]; ok {
				connection.HandleRead()
				if connection.WriteBuf.Len() > 0 {
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
				if conn.WriteBuf.Len() > 0 {
					conn.HandleWrite()
				}
                                // 写完buf数据后，删除写事件监听，监听读事件
				s.WPoll.EnableRead(fd)
			}
		}

	})
}

```

### connection

tcp流存在粘包，拆包问题，我是这样解决的，数据流的前4个字节代表长度，然后先读取一个4字节长度的数据，然后再读取对应长度的数据。存在以下两种情况分析。

- 文件描述符读取len 小于 剩余的数据长度
先查看数据长度，如果小于剩余数据长度，因为没法形成一个完整的消息体，所以停止读取操作。
- 读取的len 大于等于 剩余数据长度
能形成完整的消息体，循环读取，直至最后的数据达不到完整消息体的要求。

```go
type Connection struct {
	fd       int
	ReadBuf  *bytes.Buffer
	WriteBuf *bytes.Buffer
	proto    *DefaultProtocol  // 用于粘包拆包的协议
	callFunc MsgCallBack
}


func NewConn(fd int,c MsgCallBack) *Connection {
	return &Connection{
		fd:       fd,
		ReadBuf:  bytes.NewBuffer([]byte{}),
		WriteBuf: bytes.NewBuffer([]byte{}),
		proto:    &DefaultProtocol{},
		callFunc: c,
	}
}
```
connection 处理读操作，
```go
func (c *Connection) HandleRead() {
        //  用一个临时缓冲区从文件描述符读取数据，缓冲区过小可能造成读取的消息题不完整
	tmpBuf := make([]byte, 1024)
	n, err := unix.Read(c.fd, tmpBuf)
	if err != nil {
		if err != unix.EAGAIN{
			log.Fatal(c.fd, " conn read: ",err)
		}
		return
	}
	log.Println("fd收到了可读请求", string(tmpBuf[:n]), n)
	if n == 0 {
		return
	}
        // 将临时缓冲区数据写入conn readbuf里
	c.ReadBuf.Write(tmpBuf[:n]）
        // 拆包操作
	c.proto.Unpack(c, c.callFunc)
}

// 连接触发写事件时调用
func (c *Connection) HandleWrite() {
	if c.WriteBuf.Len() == 0 {
		return
	}
	log.Println("写入了数据：", c.WriteBuf.String())
	_, err := unix.Write(c.fd, c.WriteBuf.Bytes())
	if err != nil {
		log.Fatal(c.fd, " write ", err)
	}
	c.WriteBuf.Reset()
}
```
看下拆包如何做的
```go
func (d *DefaultProtocol) Unpack(c *Connection, msgCall MsgCallBack) {
        // 保证第一个代表长度的4个字节存在 循环处理消息
	for c.ReadBuf.Len() > 4 {
               // 获取数据包的长度
		var length = binary.BigEndian.Uint32(c.ReadBuf.Bytes()[:4])
               // 剩余数据包长度不够，等待下次读操作触发时从conn的readbuf里继续读取
		if c.ReadBuf.Len() < int(length)+4 {
			log.Println("read bug len little ,wait next time")
			return
		}

		err := binary.Read(c.ReadBuf, binary.BigEndian, &length)

		if err != nil {
			log.Fatal(err)
		}
		log.Println("获取到的数据长度是", length)
		cmdData := make([]byte, length)
		n, err := c.ReadBuf.Read(cmdData)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("读取的数据长度是", n)
		out := msgCall(c, cmdData)
                // 经过回调函数处理消息后，将返回的数据写入writebuf里，由server判断conn 的writebuf 长度，如果长度大于0 ，则同时监听连接的读写事件
		c.WriteBuf.Write(d.Pack(out))
	}
}
```

## 待完善的几个点
- connection的缓冲区目前用bytes.Buffer 会存在内存泄漏的问题，因为每次从fd读取的数据都会写到缓冲区内，如果持续读取bytes.Buffer会越来越大
- 项目目前是针对mac环境的kqueue设计，还未用linux的epoll
- 每次读取操作发生时申请临时缓冲区的操作太过频繁会造成gc过高


