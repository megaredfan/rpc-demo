package transport

import (
	"io"
	"net"
	"time"
)

type transportMaker func() Transport

type serverTransportMaker func() ServerTransport

type TransportType byte

const TCPTransport TransportType = iota

var makeTransport = map[TransportType]transportMaker{
	TCPTransport: func() Transport {
		return new(Socket)
	},
}

var makeServerTransport = map[TransportType]serverTransportMaker{
	TCPTransport: func() ServerTransport {
		return new(ServerSocket)
	},
}

type Transport interface {
	Dial(network, addr string, option DialOption) error
	io.ReadWriteCloser
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
}

type Socket struct {
	conn net.Conn
}

func NewTransport(t TransportType) Transport {
	return makeTransport[t]()
}

type DialOption struct {
	Timeout time.Duration
}

func (s *Socket) Dial(network, addr string, option DialOption) error {
	var dialer net.Dialer
	if option.Timeout > time.Duration(0) {
		dialer.Timeout = option.Timeout
	}
	conn, err := dialer.Dial(network, addr)

	s.conn = conn
	return err
}

func (s *Socket) Read(p []byte) (n int, err error) {
	return s.conn.Read(p)
}

func (s *Socket) Write(p []byte) (n int, err error) {
	return s.conn.Write(p)
}

func (s *Socket) Close() error {
	return s.conn.Close()
}

func (s *Socket) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s Socket) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

type ServerTransport interface {
	Listen(network, addr string) error
	Accept() (Transport, error)
	io.Closer
}

type ServerSocket struct {
	ln net.Listener
}

func NewServerTransport(t TransportType) ServerTransport {
	return makeServerTransport[t]()
}

func (s *ServerSocket) Listen(network, addr string) error {
	ln, err := net.Listen(network, addr)
	s.ln = ln
	return err
}

func (s *ServerSocket) Accept() (Transport, error) {
	conn, err := s.ln.Accept()
	return &Socket{conn: conn}, err
}

func (s *ServerSocket) Close() error {
	return s.ln.Close()
}
