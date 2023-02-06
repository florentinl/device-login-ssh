package server

import (
	"log"
	"net"

	"golang.org/x/crypto/ssh"
)

var id = 0

type Server struct {
	Config  *ssh.ServerConfig
	Handler func(sshConn *ssh.ServerConn, channel ssh.Channel)
}

func NewServer(config *ssh.ServerConfig, handler func(sshConn *ssh.ServerConn, channel ssh.Channel)) *Server {
	return &Server{
		Config:  config,
		Handler: handler,
	}
}

func (s *Server) ListenAndServe(port string) error {
	log.Println("Listening on port", port)
	listener, err := net.Listen("tcp", port)
	if err != nil {
		return err
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting connection", err)
		}
		go s.handleConnection(conn)
	}

}

func (s *Server) handleConnection(conn net.Conn) {
	localId := id
	id++
	log.Println("Handling connection ", localId)
	defer conn.Close()
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.Config)
	if err != nil {
		log.Println(localId, ":", "Error creating server connection", err)
		return
	}
	log.Println("Logged in")
	go ssh.DiscardRequests(reqs)
	for newChannel := range chans {
		log.Println(localId, ":", "New channel")
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Println(localId, ":", "Error accepting channel", err)
			continue
		}
		go s.handleChannel(sshConn, channel, requests)
	}
	log.Println(localId, ":", "Connection closed")
}

func (s *Server) handleChannel(sshConn *ssh.ServerConn, channel ssh.Channel, requests <-chan *ssh.Request) {
	log.Println("Handling channel")
	defer channel.Close()
	for req := range requests {
		if req.Type == "shell" {
			req.Reply(true, nil)
			s.Handler(sshConn, channel)
			break
		}
	}
}
