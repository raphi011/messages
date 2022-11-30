package main

import (
	"context"
	"crypto/tls"
	"io"
	"log"

	"github.com/lucas-clemente/quic-go"
)

func main() {
	con, err := quic.DialAddr("localhost:9000", &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"test"},
	}, nil)
	if err != nil {
		log.Fatalf("could not connect: %v", err)
	}

	stream, err := con.OpenStreamSync(context.Background())
	if err != nil {
		log.Fatalf("unable to open stream: %v", err)
	}

	_, err = stream.Write([]byte("hello world\n"))

	if err != nil {
		log.Fatalf("unable to write string: %v", err)
	}

	buf := make([]byte, 20)
	_, err = io.ReadFull(stream, buf)
	if err != nil {
		log.Fatalf("unable to close stream: %v", err)
	}
}
