package soapcalls

import (
	"fmt"
	"github.com/huin/goupnp/dcps/av1"
	"net/url"
)

func NewAVTransportClient(controlURL string) (*av1.AVTransport1, error) {
	u, err := url.Parse(controlURL)
	if err != nil {
		return nil, err
	}
	clients, err := av1.NewAVTransport1ClientsByURL(u)
	if err != nil {
		return nil, err
	}
	if len(clients) == 1 {
		return clients[0], nil
	}
	return nil, fmt.Errorf("not found AVTransport1")
}
