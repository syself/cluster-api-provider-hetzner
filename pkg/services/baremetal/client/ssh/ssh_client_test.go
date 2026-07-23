/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package sshclient contains the interface to speak to bare metal servers with ssh.
package sshclient

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func Test_removeUselessLinesFromCloudInitOutput(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "ignore: 10000K ...",
			s:    "foo\n 10000K .......... .......... .......... .......... ..........  6%!M(MISSING) 1s\nbar",
			want: "foo\nbar",
		},
		{
			name: "ignore: ^10000K ...2",
			s:    "foo\n10000K .......... .......... .......... .......... ..........  6%!M(MISSING) 1s\nbar",
			want: "foo\nbar",
		},
		{
			name: "ignore: Get:17 http://...",
			s:    "foo\nGet:17 http://archive.ubuntu.com/ubuntu focal/universe Translation-en [5,124 kB[]\nbar",
			want: "foo\nbar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeUselessLinesFromCloudInitOutput(tt.s); got != tt.want {
				t.Errorf("removeUselessLinesFromCloudInitOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOutput_String(t *testing.T) {
	f := func(o Output, wantString string) {
		require.Equal(t, wantString, o.String())
	}
	f(Output{
		StdOut: "",
		StdErr: "",
		Err:    nil,
	}, "")

	f(Output{
		StdOut: " mystdout ",
		StdErr: "",
		Err:    nil,
	}, "mystdout")

	f(Output{
		StdOut: "",
		StdErr: " mystderr ",
		Err:    nil,
	}, "mystderr")

	f(Output{
		StdOut: "",
		StdErr: "",
		Err:    fmt.Errorf(" some err "),
	}, "some err")

	f(Output{
		StdOut: "mystdout",
		StdErr: "",
		Err:    fmt.Errorf("some err"),
	}, "mystdout. Err: some err")

	f(Output{
		StdOut: "",
		StdErr: "mystderr",
		Err:    fmt.Errorf("some err"),
	}, "mystderr. Err: some err")

	f(Output{
		StdOut: "mystdout",
		StdErr: "mystderr",
		Err:    fmt.Errorf("some err"),
	}, "mystdout. Stderr: mystderr. Err: some err")
}

func Test_isTransportError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"non-zero exit status", &ssh.ExitError{}, false},
		{
			"session torn down without exit status (ambiguous: reboot or a dead transport, treated as transport)",
			&ssh.ExitMissingError{},
			true,
		},
		{"genuine transport failure", fmt.Errorf("broken pipe"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isTransportError(tt.err))
		})
	}
}

// fakeSSHServer is a minimal in-process SSH server used to test the
// connection pool without a real rescue-system host. It accepts any
// public key and answers every "exec" request with a trivial exit-0
// command, tracking how many SSH handshakes it has completed.
type fakeSSHServer struct {
	addr string

	mu             sync.Mutex
	handshakeCount int
	rawConns       []net.Conn
}

func newFakeSSHServer(t *testing.T) *fakeSSHServer {
	t.Helper()

	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(hostKey)
	require.NoError(t, err)

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, _ ssh.PublicKey) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
	}
	config.AddHostKey(signer)

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	s := &fakeSSHServer{addr: ln.Addr().String()}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.rawConns = append(s.rawConns, conn)
			s.handshakeCount++
			s.mu.Unlock()
			go s.handleConn(conn, config)
		}
	}()

	return s
}

func (s *fakeSSHServer) handleConn(conn net.Conn, config *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}
		channel, requests, err := newChan.Accept()
		if err != nil {
			continue
		}
		go func() {
			for req := range requests {
				if req.Type != "exec" {
					if req.WantReply {
						_ = req.Reply(false, nil)
					}
					continue
				}
				if req.WantReply {
					_ = req.Reply(true, nil)
				}
				_, _ = channel.Write([]byte("ok\n"))
				_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
				_ = channel.Close()
			}
		}()
	}
}

// handshakeCount returns how many TCP connections the server has accepted
// and begun an SSH handshake on so far.
func (s *fakeSSHServer) handshakes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.handshakeCount
}

// dropConnections forcibly closes every raw connection accepted so far, to
// simulate the remote end resetting the connection (reboot, sshd restart, ...).
func (s *fakeSSHServer) dropConnections() {
	s.mu.Lock()
	conns := s.rawConns
	s.rawConns = nil
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

func generateTestClientKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return string(pem.EncodeToMemory(block))
}

func newTestFactory(ctx context.Context) *sshFactory {
	// Short idle timeout/sweep interval so idle-eviction tests don't have to
	// wait for the real connIdleTimeout/connSweepInterval.
	return newFactory(ctx, 150*time.Millisecond, 20*time.Millisecond)
}

func Test_ConnectionPool_ReusesConnection(t *testing.T) {
	server := newFakeSSHServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	factory := newTestFactory(ctx)
	host, portStr, err := net.SplitHostPort(server.addr)
	require.NoError(t, err)
	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	require.NoError(t, err)

	client := factory.NewClient(Input{IP: host, Port: port, PrivateKey: generateTestClientKeyPEM(t)})

	out := client.GetHostName(ctx)
	require.NoError(t, out.Err)
	out = client.GetHostName(ctx)
	require.NoError(t, out.Err)

	require.Equal(t, 1, server.handshakes(), "two sequential calls should reuse one SSH connection")
}

func Test_ConnectionPool_ReconnectsAfterRemoteCloses(t *testing.T) {
	server := newFakeSSHServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	factory := newTestFactory(ctx)
	host, portStr, err := net.SplitHostPort(server.addr)
	require.NoError(t, err)
	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	require.NoError(t, err)

	client := factory.NewClient(Input{IP: host, Port: port, PrivateKey: generateTestClientKeyPEM(t)})

	out := client.GetHostName(ctx)
	require.NoError(t, out.Err)
	require.Equal(t, 1, server.handshakes())

	server.dropConnections()
	// Give the client's transport goroutine a moment to notice the closed
	// connection so the next call's liveness probe observes it as dead.
	time.Sleep(100 * time.Millisecond)

	out = client.GetHostName(ctx)
	require.NoError(t, out.Err, "call after remote reset should transparently reconnect")
	require.Equal(t, 2, server.handshakes(), "a dead pooled connection must be replaced by a fresh dial")
}

func Test_ConnectionPool_EvictsIdleConnection(t *testing.T) {
	server := newFakeSSHServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	factory := newTestFactory(ctx)
	host, portStr, err := net.SplitHostPort(server.addr)
	require.NoError(t, err)
	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	require.NoError(t, err)

	client := factory.NewClient(Input{IP: host, Port: port, PrivateKey: generateTestClientKeyPEM(t)})

	out := client.GetHostName(ctx)
	require.NoError(t, out.Err)
	require.Equal(t, 1, server.handshakes())

	// Wait past the (test-shortened) idle timeout and sweep interval so the
	// background sweep evicts the pooled connection.
	require.Eventually(t, func() bool {
		factory.mu.RLock()
		defer factory.mu.RUnlock()
		return len(factory.conns) == 0
	}, time.Second, 10*time.Millisecond, "idle sweep should have evicted the pooled connection")

	out = client.GetHostName(ctx)
	require.NoError(t, out.Err)
	require.Equal(t, 2, server.handshakes(), "a call after the idle eviction should dial a fresh connection")
}

func Test_ConnectionPool_DifferentPrivateKeysDoNotShareConnection(t *testing.T) {
	server := newFakeSSHServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	factory := newTestFactory(ctx)
	host, portStr, err := net.SplitHostPort(server.addr)
	require.NoError(t, err)
	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	require.NoError(t, err)

	client1 := factory.NewClient(Input{IP: host, Port: port, PrivateKey: generateTestClientKeyPEM(t)})
	client2 := factory.NewClient(Input{IP: host, Port: port, PrivateKey: generateTestClientKeyPEM(t)})

	out := client1.GetHostName(ctx)
	require.NoError(t, out.Err)
	out = client2.GetHostName(ctx)
	require.NoError(t, out.Err)

	require.Equal(t, 2, server.handshakes(), "different private keys must never share a pooled connection")
}

func Test_ConnectionPool_EvictClosesConnectionForIP(t *testing.T) {
	server := newFakeSSHServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	factory := newTestFactory(ctx)
	host, portStr, err := net.SplitHostPort(server.addr)
	require.NoError(t, err)
	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	require.NoError(t, err)

	client := factory.NewClient(Input{IP: host, Port: port, PrivateKey: generateTestClientKeyPEM(t)})

	out := client.GetHostName(ctx)
	require.NoError(t, out.Err)
	require.Equal(t, 1, server.handshakes())

	factory.EvictConnectionsForIP(host)

	out = client.GetHostName(ctx)
	require.NoError(t, out.Err)
	require.Equal(t, 2, server.handshakes(), "EvictConnectionsForIP should force the next call to dial a fresh connection")
}

// midCommandDropServer is a minimal in-process SSH server that accepts an
// "exec" request and then immediately drops the raw TCP connection without
// ever sending output or an exit status -- unlike a clean session teardown
// (e.g. "reboot" closing its own session), this simulates the transport
// itself dying while a command is in flight.
type midCommandDropServer struct {
	addr string
}

func newMidCommandDropServer(t *testing.T) *midCommandDropServer {
	t.Helper()

	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(hostKey)
	require.NoError(t, err)

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, _ ssh.PublicKey) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
	}
	config.AddHostKey(signer)

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	s := &midCommandDropServer{addr: ln.Addr().String()}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handleConn(conn, config)
		}
	}()
	return s
}

func (s *midCommandDropServer) handleConn(conn net.Conn, config *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}
		_, requests, err := newChan.Accept()
		if err != nil {
			continue
		}
		go func() {
			for req := range requests {
				if req.Type != "exec" {
					if req.WantReply {
						_ = req.Reply(false, nil)
					}
					continue
				}
				if req.WantReply {
					_ = req.Reply(true, nil)
				}
				// Drop the raw connection now, before any output or
				// exit-status is sent: the command is "in flight" from the
				// client's point of view.
				_ = conn.Close()
				return
			}
		}()
	}
}

func Test_ConnectionPool_EvictsOnMidCommandTransportError(t *testing.T) {
	server := newMidCommandDropServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Long idle timeout/sweep interval: this test asserts that runSSH itself
	// evicts the pooled entry on a transport error, so the idle sweep must
	// not be able to interfere with that assertion.
	factory := newFactory(ctx, time.Hour, time.Hour)
	host, portStr, err := net.SplitHostPort(server.addr)
	require.NoError(t, err)
	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	require.NoError(t, err)

	client := factory.NewClient(Input{IP: host, Port: port, PrivateKey: generateTestClientKeyPEM(t)})

	out := client.GetHostName(ctx)
	require.Error(t, out.Err, "a connection dropped mid-command must surface as an error")
	t.Logf("runSSH returned: %v", out.Err)

	factory.mu.RLock()
	defer factory.mu.RUnlock()
	require.Empty(t, factory.conns, "runSSH must evict the pooled entry after a mid-command transport failure")
}

// Test_Reboot_TreatsSessionTornDownWithoutExitStatusAsSuccess guards the
// specific behavior that made isTransportError's classification tricky:
// "reboot" tears down its own session the same way an unrelated dead
// transport would (see midCommandDropServer), and Reboot() must still report
// success -- while the pooled connection is evicted regardless, proving that
// the eviction itself does not alter the returned Output.
func Test_Reboot_TreatsSessionTornDownWithoutExitStatusAsSuccess(t *testing.T) {
	server := newMidCommandDropServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	factory := newFactory(ctx, time.Hour, time.Hour)
	host, portStr, err := net.SplitHostPort(server.addr)
	require.NoError(t, err)
	var port int
	_, err = fmt.Sscanf(portStr, "%d", &port)
	require.NoError(t, err)

	client := factory.NewClient(Input{IP: host, Port: port, PrivateKey: generateTestClientKeyPEM(t)})

	out := client.Reboot(ctx)
	require.NoError(t, out.Err, "Reboot() must treat a session torn down without an exit status as success")

	factory.mu.RLock()
	defer factory.mu.RUnlock()
	require.Empty(t, factory.conns, "the pooled connection should still be evicted even though Reboot() reports success")
}

func Test_ExecutePreProvisionCommand_withRealServer(t *testing.T) {
	// This test is disabled by default because it requires a real server to be up and running.
	// It is useful to enable it when debugging scp issues.
	t.SkipNow()

	ctx := context.Background()
	pk, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa"))
	require.NoError(t, err)

	c := sshClient{
		privateSSHKey: string(pk),
		ip:            "178.63.61.147",
		port:          22,
	}

	exitStatus, output, err := c.ExecutePreProvisionCommand(ctx, "/usr/bin/hostname")
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
	require.Equal(t, "hz1", output)
}
