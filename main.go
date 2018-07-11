package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/oklog/oklog/pkg/group"
)

var (
	cert tls.Certificate
)

func init() {
	var err error
	cert, err = tls.LoadX509KeyPair("certs/server.crt", "certs/server.key")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func logmessage(c net.Conn) {
	r := bufio.NewReader(c)
	msg, err := r.ReadString('\n')
	if err != nil {
		fmt.Println(err)
	}

	println(msg)
	return
}

func handleSecureConnection(l net.Listener) error {
	for {
		c, err := l.Accept()
		if err != nil {
			return err
		}
		defer c.Close()
		// Doesn't break if you put the logger right in here
		logmessage(c)
	}

}

func newHandler(logger log.Logger) http.Handler {
	options := []httptransport.ServerOption{
		httptransport.ServerErrorLogger(logger),
	}

	handler := httptransport.NewServer(
		handleBase,
		decodeHTTPRequest,
		encodeHTTPGenericResponse,
		options...,
	)
	m := http.NewServeMux()
	m.Handle("/", handler)

	return m
}

func handleBase(ctx context.Context, request interface{}) (interface{}, error) {
	return nil, nil
}

func decodeHTTPRequest(ctx context.Context, req *http.Request) (request interface{}, err error) {
	return nil, nil
}

func encodeHTTPGenericResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}

func main() {
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}

	httpListener, err := net.Listen("tcp", "localhost:10443")
	if err != nil {
		os.Exit(1)
	}
	httpHandler := newHandler(logger)

	var g group.Group
	{
		g.Add(func() error {
			return http.Serve(httpListener, httpHandler)
		}, func(error) {
			httpListener.Close()
		})
		{
			tlsListener := tls.NewListener(
				httpListener,
				&tls.Config{Certificates: []tls.Certificate{cert}},
			)

			g.Add(func() error {
				return handleSecureConnection(tlsListener)
			}, func(error) {
				tlsListener.Close()
			})
		}
		{
			// This function just sits and waits for ctrl-C.
			cancelInterrupt := make(chan struct{})
			g.Add(func() error {
				c := make(chan os.Signal, 1)
				signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
				select {
				case sig := <-c:
					return fmt.Errorf("received signal %s", sig)
				case <-cancelInterrupt:
					return nil
				}
			}, func(error) {
				close(cancelInterrupt)
			})
		}
		logger.Log("exit", g.Run())
	}
}
