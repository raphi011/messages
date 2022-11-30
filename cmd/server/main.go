package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go"
)

const (
	StateFollower  = "follower"
	StateCandidate = "candidate"
	StateLeader    = "leader"
)

type config struct {
	peers       []string
	listenAddr  string
	tlsCertPath string
	tlsKeyPath  string
}

type peer struct {
	address  string
	conState string
}

type server struct {
	state       string
	currentTerm int
	votedFor    int
	activePeers []peer

	config config

	l sync.Mutex
}

func main() {
	peers := flag.String("peers", "", "comma separated ip addresses of peers")
	certPath := flag.String("cert", "", "path to tls certificate")
	keyPath := flag.String("key", "", "path to tls key")
	listenAddr := flag.String("listen", "", "server address")

	flag.Parse()

	c := config{
		peers:       strings.Split(*peers, ","),
		listenAddr:  *listenAddr,
		tlsCertPath: *certPath,
		tlsKeyPath:  *keyPath,
	}

	s := &server{config: c}

	err := s.run()
	if err != nil {
		log.Fatal(err)
	}
}

const alpnQuicTransport = "test"

func (s *server) run() error {
	cert, err := tls.LoadX509KeyPair(s.config.tlsCertPath, s.config.tlsKeyPath)
	if err != nil {
		return err
	}

	for _, address := range s.config.peers {
		go s.connectPeer(address)
	}

	return s.listenToPeers(cert)
}

func (s *server) connectPeer(address string) {
	var con quic.Connection
	var err error
	var stream quic.Stream

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		con, err = quic.DialAddrContext(ctx, address, &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"test"},
		}, &quic.Config{KeepAlivePeriod: 15 * time.Second})
		if err == nil {
			break
		}

		log.Printf("could not connect: %v, retrying\n", err)
		time.Sleep(time.Second)
	}

	log.Printf("connected to %s\n", address)

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		stream, err = con.OpenStreamSync(ctx)
		if err == nil {
			break
		}

		log.Printf("could not open stream: %v, retrying\n", err)
		time.Sleep(time.Second)
	}

	scanner := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	scanner.WriteString("connect\n")
	err = scanner.Flush()
	if err != nil {
		log.Printf("error flushing %v\n", err)
	}
	line, err := scanner.ReadString('\n')
	if err != nil {
		log.Printf("error reading from con: %v\n", err)
	}

	s.l.Lock()
	s.activePeers = append(s.activePeers, peer{address: address, conState: "connected"})
	s.l.Unlock()

	fmt.Printf("success: %s\n", line)
}

func (s *server) listenToPeers(cert tls.Certificate) error {
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{alpnQuicTransport},
	}

	listener, err := quic.ListenAddr(s.config.listenAddr, tlsConfig, &quic.Config{
		EnableDatagrams: true,
	})
	if err != nil {
		return err
	}

	for {
		sess, err := listener.Accept(context.Background())
		if err != nil {
			log.Printf("accepting conn: %v", err)
		}

		if err = s.handleSession(sess); err != nil {
			log.Printf("handling session: %v", err)
		}
	}
}

func (s *server) handleSession(session quic.Connection) error {
	stream, err := session.AcceptStream(context.Background())
	if err != nil {
		return err
	}

	scanner := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	if err != nil {
		return err
	}

	for {
		cmd, err := scanner.ReadString('\n')
		if err != nil {
			log.Printf("reading from peer: %v\n", err)
			return err
		}

		fmt.Printf("received command %s from %s", cmd, session.RemoteAddr().String())
	}
}
