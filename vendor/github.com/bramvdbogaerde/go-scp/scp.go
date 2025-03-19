/* Copyright (c) 2021 Bram Vandenbogaerde
 * You may use, distribute or modify this code under the
 * terms of the Mozilla Public License 2.0, which is distributed
 * along with the source code.
 */

// Package scp.
// Simple scp package to copy files over SSH.
package scp

import (
	"time"

	"golang.org/x/crypto/ssh"
)

// NewClient returns a new scp.Client with provided host and ssh.clientConfig.
func NewClient(host string, config *ssh.ClientConfig) Client {
	return NewConfigurer(host, config).Create()
}

// NewClientWithTimeout returns a new scp.Client with provides host, ssh.ClientConfig and timeout.
// Deprecated: provide meaningful context to each "Copy*" function instead.
func NewClientWithTimeout(host string, config *ssh.ClientConfig, timeout time.Duration) Client {
	return NewConfigurer(host, config).Timeout(timeout).Create()
}

// NewClientBySSH returns a new scp.Client using an already existing established SSH connection.
func NewClientBySSH(ssh *ssh.Client) (Client, error) {
	return NewConfigurer("", nil).SSHClient(ssh).Create(), nil
}

// NewClientBySSHWithTimeout same as NewClientWithTimeout but uses an existing SSH client.
// Deprecated: provide meaningful context to each "Copy*" function instead.
func NewClientBySSHWithTimeout(ssh *ssh.Client, timeout time.Duration) (Client, error) {
	return NewConfigurer("", nil).SSHClient(ssh).Timeout(timeout).Create(), nil
}
