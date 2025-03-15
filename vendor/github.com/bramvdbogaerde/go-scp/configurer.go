/* Copyright (c) 2020 Bram Vandenbogaerde
 * You may use, distribute or modify this code under the
 * terms of the Mozilla Public License 2.0, which is distributed
 * along with the source code.
 */

package scp

import (
	"time"

	"golang.org/x/crypto/ssh"
)

// ClientConfigurer a struct containing all the configuration options
// used by an scp client.
type ClientConfigurer struct {
	host         string
	clientConfig *ssh.ClientConfig
	session      *ssh.Session
	timeout      time.Duration
	remoteBinary string
	sshClient    *ssh.Client
}

// NewConfigurer creates a new client configurer.
// It takes the required parameters: the host and the ssh.ClientConfig and
// returns a configurer populated with the default values for the optional
// parameters.
//
// These optional parameters can be set by using the methods provided on the
// ClientConfigurer struct.
func NewConfigurer(host string, config *ssh.ClientConfig) *ClientConfigurer {
	return &ClientConfigurer{
		host:         host,
		clientConfig: config,
		timeout:      0, // no timeout by default
		remoteBinary: "scp",
	}
}

// RemoteBinary sets the path of the location of the remote scp binary
// Defaults to: /usr/bin/scp.
func (c *ClientConfigurer) RemoteBinary(path string) *ClientConfigurer {
	c.remoteBinary = path
	return c
}

// Host alters the host of the client connects to.
func (c *ClientConfigurer) Host(host string) *ClientConfigurer {
	c.host = host
	return c
}

// Timeout Changes the connection timeout.
// Defaults to one minute.
func (c *ClientConfigurer) Timeout(timeout time.Duration) *ClientConfigurer {
	c.timeout = timeout
	return c
}

// ClientConfig alters the ssh.ClientConfig.
func (c *ClientConfigurer) ClientConfig(config *ssh.ClientConfig) *ClientConfigurer {
	c.clientConfig = config
	return c
}

func (c *ClientConfigurer) SSHClient(sshClient *ssh.Client) *ClientConfigurer {
	c.sshClient = sshClient
	return c
}

// Create builds a client with the configuration stored within the ClientConfigurer.
func (c *ClientConfigurer) Create() Client {
	return Client{
		Host:         c.host,
		ClientConfig: c.clientConfig,
		Timeout:      c.timeout,
		RemoteBinary: c.remoteBinary,
		sshClient:    c.sshClient,
		closeHandler: EmptyHandler{},
	}
}
