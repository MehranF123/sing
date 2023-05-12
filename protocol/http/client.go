package http

import (
	std_bufio "bufio"
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/MehranF123/sing/common/buf"
	"github.com/MehranF123/sing/common/bufio"
	E "github.com/MehranF123/sing/common/exceptions"
	M "github.com/MehranF123/sing/common/metadata"
	N "github.com/MehranF123/sing/common/network"
)

var _ N.Dialer = (*Client)(nil)

type Client struct {
	dialer     N.Dialer
	serverAddr M.Socksaddr
	username   string
	password   string
}

func NewClient(dialer N.Dialer, serverAddr M.Socksaddr, username string, password string) *Client {
	return &Client{
		dialer,
		serverAddr,
		username,
		password,
	}
}

func (c *Client) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if !strings.HasPrefix(network, "tcp") {
		return nil, os.ErrInvalid
	}
	var conn net.Conn
	conn, err := c.dialer.DialContext(ctx, "tcp", c.serverAddr)
	if err != nil {
		return nil, err
	}
	destinationAddress := destination.String()
	request := &http.Request{
		Method: http.MethodConnect,
		URL: &url.URL{
			Host: destinationAddress,
		},
		Host: destinationAddress,
		Header: http.Header{
			"Proxy-Connection": []string{"Keep-Alive"},
		},
	}
	if c.username != "" {
		auth := c.username + ":" + c.password
		request.Header.Add("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
	}
	err = request.Write(conn)
	if err != nil {
		return nil, err
	}
	reader := std_bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, request)
	if err != nil {
		return nil, err
	}
	switch response.StatusCode {
	case http.StatusOK:
		if reader.Buffered() > 0 {
			buffer := buf.NewSize(reader.Buffered())
			_, err = buffer.ReadFullFrom(reader, buffer.FreeLen())
			if err != nil {
				return nil, err
			}
			conn = bufio.NewCachedConn(conn, buffer)
		}
		return conn, nil
	case http.StatusProxyAuthRequired:
		return nil, E.New("authentication required")
	case http.StatusMethodNotAllowed:
		return nil, E.New("method not allowed")
	default:
		return nil, E.New("unexpected status: ", response.Status)
	}
}

func (c *Client) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}
