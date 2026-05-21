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
	"encoding/binary"
	"encoding/pem"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// fakeSSHResponse defines what a fake SSH server returns for a given command.
type fakeSSHResponse struct {
	stdout   string
	exitCode uint32
}

// startFakeSSHServer starts an in-process SSH server that maps exact command strings
// to fixed responses. Each runSSH call opens a new TCP connection, so the server
// handles concurrent connections. Returns a PEM-encoded RSA private key the sshClient
// can use for authentication (server accepts any key) and the listener port.
func startFakeSSHServer(t *testing.T, responses map[string]fakeSSHResponse) (clientPrivKeyPEM string, port int) {
	t.Helper()

	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	hostSigner, err := ssh.NewSignerFromKey(hostKey)
	require.NoError(t, err)

	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	clientPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
	}))

	serverConfig := &ssh.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go fakeSSHHandleConn(conn, serverConfig, responses)
		}
	}()

	return clientPEM, ln.Addr().(*net.TCPAddr).Port
}

func fakeSSHHandleConn(conn net.Conn, config *ssh.ServerConfig, responses map[string]fakeSSHResponse) {
	srvConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer srvConn.Close()
	go ssh.DiscardRequests(reqs)
	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		ch, requests, err := newChan.Accept()
		if err != nil {
			return
		}
		go fakeSSHHandleSession(ch, requests, responses)
	}
}

func fakeSSHHandleSession(ch ssh.Channel, requests <-chan *ssh.Request, responses map[string]fakeSSHResponse) {
	defer ch.Close()
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
		// SSH exec payload: 4-byte big-endian length followed by the command string.
		if len(req.Payload) < 4 {
			fakeSSHSendExitStatus(ch, 1)
			return
		}
		cmdLen := binary.BigEndian.Uint32(req.Payload[:4])
		cmd := string(req.Payload[4 : 4+cmdLen])

		resp, ok := responses[cmd]
		if !ok {
			fakeSSHSendExitStatus(ch, 127)
			return
		}
		if resp.stdout != "" {
			_, _ = ch.Write([]byte(resp.stdout))
		}
		fakeSSHSendExitStatus(ch, resp.exitCode)
		return
	}
}

func fakeSSHSendExitStatus(ch ssh.Channel, code uint32) {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, code)
	_, _ = ch.SendRequest("exit-status", false, payload)
}

// TestStateOfImageURLCommandV2_ProcessExitedWithoutOutputJSON verifies that
// StateOfImageURLCommandV2 returns ImageURLCommandStateFailed immediately when
// the image-url-command process has exited but output.json is missing.
//
// Without the fix this test exposes, the function returned ImageURLCommandStateRunning,
// causing the controller to wait up to 7 minutes for a timeout that was never needed.
func TestStateOfImageURLCommandV2_ProcessExitedWithoutOutputJSON(t *testing.T) {
	privKeyPEM, port := startFakeSSHServer(t, map[string]fakeSSHResponse{
		`[ -e /root/image-url-command.pid ]`: {exitCode: 0}, // PID file exists
		`ps -p "$(cat /root/image-url-command.pid)" -o args= | grep -q image-url-command`: {exitCode: 1}, // process gone
		`cat ` + outputJSONPath: {exitCode: 1}, // output.json missing
	})

	c := sshClient{ip: "127.0.0.1", privateSSHKey: privKeyPEM, port: port}

	state, detail, err := c.StateOfImageURLCommandV2(context.Background())
	require.NoError(t, err)
	require.Equal(t, ImageURLCommandStateFailed, state,
		"expected immediate failure when process is gone without output.json, not ImageURLCommandStateRunning")
	require.Contains(t, detail, "process exited")
}

// TestStateOfImageURLCommandV2_ProcessExitedWithEmptyStatus verifies that
// StateOfImageURLCommandV2 returns ImageURLCommandStateFailed when the process
// has exited and output.json exists but has an empty Status field (e.g. the
// binary wrote a partial file before crashing).
func TestStateOfImageURLCommandV2_ProcessExitedWithEmptyStatus(t *testing.T) {
	privKeyPEM, port := startFakeSSHServer(t, map[string]fakeSSHResponse{
		`[ -e /root/image-url-command.pid ]`: {exitCode: 0},
		`ps -p "$(cat /root/image-url-command.pid)" -o args= | grep -q image-url-command`: {exitCode: 1},
		`cat ` + outputJSONPath: {exitCode: 0, stdout: `{"apiVersion":"v2"}`}, // Status field missing
	})

	c := sshClient{ip: "127.0.0.1", privateSSHKey: privKeyPEM, port: port}

	state, _, err := c.StateOfImageURLCommandV2(context.Background())
	require.NoError(t, err)
	require.Equal(t, ImageURLCommandStateFailed, state,
		"expected failure when output.json has no Status field")
}

// TestStateOfImageURLCommandV2_ProcessStillRunning verifies that
// StateOfImageURLCommandV2 returns ImageURLCommandStateRunning while the
// process is still alive (the normal in-progress case).
func TestStateOfImageURLCommandV2_ProcessStillRunning(t *testing.T) {
	privKeyPEM, port := startFakeSSHServer(t, map[string]fakeSSHResponse{
		`[ -e /root/image-url-command.pid ]`: {exitCode: 0},
		`ps -p "$(cat /root/image-url-command.pid)" -o args= | grep -q image-url-command`: {exitCode: 0}, // still running
	})

	c := sshClient{ip: "127.0.0.1", privateSSHKey: privKeyPEM, port: port}

	state, _, err := c.StateOfImageURLCommandV2(context.Background())
	require.NoError(t, err)
	require.Equal(t, ImageURLCommandStateRunning, state)
}

// TestStateOfImageURLCommandV2_FinishedSuccessfully verifies that
// StateOfImageURLCommandV2 returns ImageURLCommandStateFinishedSuccessfully
// when the process has exited and output.json contains a non-empty Status.
func TestStateOfImageURLCommandV2_FinishedSuccessfully(t *testing.T) {
	outputJSON := `{"apiVersion":"v2","status":"Succeeded","phases":{}}`
	privKeyPEM, port := startFakeSSHServer(t, map[string]fakeSSHResponse{
		`[ -e /root/image-url-command.pid ]`: {exitCode: 0},
		`ps -p "$(cat /root/image-url-command.pid)" -o args= | grep -q image-url-command`: {exitCode: 1},
		`cat ` + outputJSONPath: {exitCode: 0, stdout: outputJSON},
	})

	c := sshClient{ip: "127.0.0.1", privateSSHKey: privKeyPEM, port: port}

	state, content, err := c.StateOfImageURLCommandV2(context.Background())
	require.NoError(t, err)
	require.Equal(t, ImageURLCommandStateFinishedSuccessfully, state)
	require.Equal(t, outputJSON, content)
}
