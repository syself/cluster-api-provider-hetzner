// Code generated by ifacemaker; DO NOT EDIT.

package hcloud

import (
	"context"
	"net"
)

// IRDNSClient ...
type IRDNSClient interface {
	// ChangeDNSPtr changes or resets the reverse DNS pointer for a IP address.
	// Pass a nil ptr to reset the reverse DNS pointer to its default value.
	ChangeDNSPtr(ctx context.Context, rdns RDNSSupporter, ip net.IP, ptr *string) (*Action, *Response, error)
}
