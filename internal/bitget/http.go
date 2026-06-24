package bitget

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
)

const bitgetAPIBase = "https://api.bitget.com"

var bitgetAPIIPs = []string{"104.18.14.166", "104.18.15.166"}

func newHTTPClient(timeout time.Duration) *http.Client {
	ipIndex := 0
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialTLS: func(network, addr string) (net.Conn, error) {
				host := bitgetAPIIPs[ipIndex%len(bitgetAPIIPs)]
				ipIndex++
				d := net.Dialer{Timeout: 5 * time.Second}
				conn, err := d.DialContext(context.Background(), network, host+":443")
				if err != nil {
					return nil, fmt.Errorf("bitget direct dial: %w", err)
				}
				tlsConn := tls.Client(conn, &tls.Config{
					ServerName: "api.bitget.com",
				})
				if err := tlsConn.Handshake(); err != nil {
					conn.Close()
					return nil, fmt.Errorf("bitget tls handshake: %w", err)
				}
				return tlsConn, nil
			},
		},
	}
}
