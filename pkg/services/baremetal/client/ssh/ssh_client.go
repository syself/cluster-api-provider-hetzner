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
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	scp "github.com/bramvdbogaerde/go-scp"
	"golang.org/x/crypto/ssh"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	sshTimeOut time.Duration = 5 * time.Second
	sshUser                  = "root"

	imageURLCommandLog   = "/root/image-url-command.log"
	outputJSONPath       = "/root/output.json"
	outputJSONMaxRetries = 10

	// connIdleTimeout is how long a pooled connection may sit unused before the
	// idle sweep closes it. It is comfortably above the ~10s poll interval used
	// while a host is in the rescue system, but short enough that an
	// abandoned/failed/deleted host's connection doesn't linger.
	connIdleTimeout = 2 * time.Minute

	// connSweepInterval is how often the idle sweep runs.
	connSweepInterval = 30 * time.Second

	// connKeepAliveRequest is used to cheaply check whether a pooled connection
	// is still alive before handing it out. It is not a real SSH protocol
	// feature: the "name@domain" form (RFC 4251 4.6.1) marks it as a
	// vendor-specific global request, so it can never collide with a name an
	// RFC or another implementation defines. OpenSSH originated this
	// particular name for exactly this liveness-probing purpose, and other
	// implementations copied the convention; the peer is never expected to
	// recognize it. See isConnAlive for why that's fine.
	connKeepAliveRequest = "keepalive@openssh.com"
)

//go:embed detect-linux-on-another-disk.sh
var detectLinuxOnAnotherDiskShellScript string

//go:embed wipe-disk.sh
var wipeDiskShellScript string

//go:embed check-disk.sh
var checkDiskShellScript string

//go:embed nic-info.sh
var nicInfoShellScript string

//go:embed download-from-oci.sh
var downloadFromOciShellScript string

var (
	// ErrCommandExitedWithoutExitSignal means the ssh command exited unplanned.
	ErrCommandExitedWithoutExitSignal = errors.New("wait: remote command exited without exit status or exit signal")

	// ErrAuthenticationFailed means ssh was unable to authenticate.
	ErrAuthenticationFailed = errors.New("ssh: unable to authenticate")
	// ErrEmptyStdOut means that StdOut equals empty string.
	ErrEmptyStdOut = errors.New("unexpected empty output in stdout")
	// ErrTimeout means that there is a timeout error.
	ErrTimeout = errors.New("i/o timeout")
	// ErrCheckDiskBrokenDisk means that a disk seams broken.
	ErrCheckDiskBrokenDisk = errors.New("CheckDisk failed")
)

// Input defines an SSH input.
type Input struct {
	IP         string
	PrivateKey string
	Port       int
}

// Output defines the SSH output.
type Output struct {
	StdOut string
	StdErr string
	Err    error
}

// InstallImageState defines three states of the process.
type InstallImageState string

const (
	// InstallImageStateNotStartedYet means the process has not started yet.
	InstallImageStateNotStartedYet InstallImageState = "not-started-yet"
	// InstallImageStateRunning means the process is still running.
	InstallImageStateRunning InstallImageState = "running"
	// InstallImageStateFinished has finished.
	InstallImageStateFinished InstallImageState = "finished"
)

// ImageURLCommandState is the command which reads the imageURL of and provisions the machine accordingly. It gets copied to the server running in the rescue system.
type ImageURLCommandState string

const (
	// ImageURLCommandStateNotStarted indicates that the command was not started yet.
	ImageURLCommandStateNotStarted ImageURLCommandState = "ImageURLCommandStateNotStarted"

	// ImageURLCommandStateRunning indicates that the command is running.
	ImageURLCommandStateRunning ImageURLCommandState = "ImageURLCommandStateRunning"

	// ImageURLCommandStateFinishedSuccessfully indicates that the command is finished with IMAGE_URL_DONE in
	// stdout.
	ImageURLCommandStateFinishedSuccessfully ImageURLCommandState = "ImageURLCommandStateFinishedSuccessfully"

	// ImageURLCommandStateFailed indicates that the command is finished, but failed.
	ImageURLCommandStateFailed ImageURLCommandState = "ImageURLCommandStateFailed"
)

func (o Output) String() string {
	s := make([]string, 0, 3)
	stdout := strings.TrimSpace(o.StdOut)
	if stdout != "" {
		s = append(s, stdout)
	}
	stderr := strings.TrimSpace(o.StdErr)
	if stderr != "" {
		if len(s) > 0 {
			stderr = "Stderr: " + stderr
		}
		s = append(s, stderr)
	}
	if o.Err != nil {
		e := o.Err.Error()
		e = strings.TrimSpace(e)
		if len(s) > 0 {
			e = "Err: " + e
		}
		s = append(s, e)
	}
	return strings.Join(s, ". ")
}

// ExitStatus returns the exit status of the remote shell command.
// There are three case:
// First case: Remote command finished with exit 0: 0, nil.
// Second case: Remote command finished with non zero: N, nil.
// Third case: Remote command was not called successfully (like "host not reachable"): 0, err.
func (o Output) ExitStatus() (int, error) {
	var exitError *ssh.ExitError
	if errors.As(o.Err, &exitError) {
		return exitError.ExitStatus(), nil
	}
	return 0, o.Err
}

// Client is the interface defining all functions necessary to talk to a bare metal server via SSH.
type Client interface {
	GetHostName(ctx context.Context) Output
	GetHardwareDetailsRAM(ctx context.Context) Output
	GetHardwareDetailsNics(ctx context.Context) Output
	GetHardwareDetailsStorage(ctx context.Context) Output
	GetHardwareDetailsCPUArch(ctx context.Context) Output
	GetHardwareDetailsCPUModel(ctx context.Context) Output
	GetHardwareDetailsCPUClockGigahertz(ctx context.Context) Output
	GetHardwareDetailsCPUFlags(ctx context.Context) Output
	GetHardwareDetailsCPUThreads(ctx context.Context) Output
	GetHardwareDetailsCPUCores(ctx context.Context) Output
	GetHardwareDetailsDebug(ctx context.Context) Output
	GetInstallImageState(ctx context.Context) (InstallImageState, error)
	GetResultOfInstallImage(ctx context.Context) (string, error)
	GetCloudInitOutput(ctx context.Context) Output
	CreateAutoSetup(ctx context.Context, data string) Output

	// DownloadImage is a synchronous process. This means the controller waits until the
	// download is finished. Note: We should use StartImageURLCommand(), similar to the handling
	// of ImageURLCommand.
	DownloadImage(ctx context.Context, path, url string) Output

	CreatePostInstallScript(ctx context.Context, data string) Output
	ExecuteInstallImage(ctx context.Context, hasPostInstallScript bool) Output
	Reboot(ctx context.Context) Output
	CloudInitStatus(ctx context.Context) Output
	CheckCloudInitLogsForSigTerm(ctx context.Context) Output
	CleanCloudInitLogs(ctx context.Context) Output
	CleanCloudInitInstances(ctx context.Context) Output
	ResetKubeadm(ctx context.Context) Output
	UntarTGZ(ctx context.Context) Output
	DetectLinuxOnAnotherDisk(ctx context.Context, sliceOfWwns []string) Output

	// Erase filesystem, raid and partition-table signatures.
	// String "all" will wipe all disks.
	WipeDisk(ctx context.Context, sliceOfWwns []string) (string, error)

	// CheckDisk checks the given disks via smartctl.
	// ErrCheckDiskBrokenDisk gets returned, if a disk is broken.
	CheckDisk(ctx context.Context, sliceOfWwns []string) (info string, err error)

	// ExecutePreProvisionCommand executes a command before the provision process starts.
	// A non-zero exit status will indicate that provisioning should not start.
	ExecutePreProvisionCommand(ctx context.Context, preProvisionCommand string) (exitStatus int, stdoutAndStderr string, err error)

	// StartImageURLCommand calls the command provided via image-url-command.
	// It gets called by the controller after the rescue system of the new machine
	// is reachable. The env var `OCI_REGISTRY_AUTH_TOKEN` gets set to the same value of the
	// corresponding env var of the controller.
	// This gets used when imageURL set.
	// For hcloud deviceNames is always {"sda"}. For baremetal it corresponds to the WWNs
	// of RootDeviceHints.
	StartImageURLCommand(ctx context.Context, command, imageURL string, bootstrapData []byte, machineName string, deviceNames []string) (exitStatus int, stdoutAndStderr string, err error)

	// StateOfImageURLCommand returns the current states of the ImageURLCommand. States can
	// be: NotStarted, Running, Failed, FinishedSuccesfully.
	StateOfImageURLCommand(ctx context.Context) (state ImageURLCommandState, logFile string, err error)

	// ReadOutputJSON reads /root/output.json from the rescue system. It retries up to
	// outputJSONMaxRetries times when the content does not end with '}', which guards against
	// reading a partially-written file. An empty or not existing file returns an empty string and
	// error is nil.
	ReadOutputJSON(ctx context.Context) (string, error)
}

// Factory is the interface for creating new Client objects.
type Factory interface {
	NewClient(Input) Client

	// EvictConnectionsForIP closes and removes any pooled connection to the
	// given IP, regardless of port or private key. Callers should call this
	// once a host leaves the rescue-related states, so the connection cache
	// doesn't linger beyond the window where reuse is actually useful.
	EvictConnectionsForIP(ip string)
}

// connKey identifies a pooled connection. The private key is hashed (not
// stored/compared as plain text) so that a rotated SSH secret (rescue key
// changed, or the same IP now needs the OS key after installimage) naturally
// lands on a different pool entry instead of accidentally reusing a
// connection authenticated with the wrong key.
type connKey struct {
	ip      string
	port    int
	keyHash [sha256.Size]byte
}

func newConnKey(ip string, port int, privateKey string) connKey {
	return connKey{
		ip:      ip,
		port:    port,
		keyHash: sha256.Sum256([]byte(privateKey)),
	}
}

// pooledConn wraps a shared *ssh.Client. Its mutex serializes get-or-create
// and evict operations for this one entry, so concurrent callers for the same
// connKey neither dial twice nor race on lastUsed or client. See getSSHClient
// for why the lock is held across the liveness probe and dial, not just the
// map/field access.
type pooledConn struct {
	mu       sync.Mutex
	client   *ssh.Client
	lastUsed time.Time
}

type sshFactory struct {
	mu    sync.RWMutex
	conns map[connKey]*pooledConn

	idleTimeout   time.Duration
	sweepInterval time.Duration
}

// NewFactory creates a new factory for SSH clients. The idle-connection sweep
// it starts runs until ctx is done, at which point all pooled connections are
// closed. ctx should be the controller manager's long-lived context, not a
// per-Reconcile context: the factory and its pooled connections must outlive
// any single Reconcile call.
func NewFactory(ctx context.Context) Factory {
	return newFactory(ctx, connIdleTimeout, connSweepInterval)
}

// newFactory is the actual constructor; tests use it with a shorter idle
// timeout/sweep interval so they don't have to wait connIdleTimeout for real.
func newFactory(ctx context.Context, idleTimeout, sweepInterval time.Duration) *sshFactory {
	f := &sshFactory{
		conns:         make(map[connKey]*pooledConn),
		idleTimeout:   idleTimeout,
		sweepInterval: sweepInterval,
	}
	go f.sweepIdleConns(ctx)
	return f
}

var _ Factory = (*sshFactory)(nil)

// NewClient implements the NewClient method of the factory interface.
func (f *sshFactory) NewClient(in Input) Client {
	return &sshClient{
		privateSSHKey: in.PrivateKey,
		ip:            in.IP,
		port:          in.Port,
		factory:       f,
	}
}

// entry returns the pooled entry for key, creating an empty one if necessary.
// The returned entry's client may be nil, meaning no connection is cached yet.
func (f *sshFactory) entry(key connKey) *pooledConn {
	f.mu.RLock()
	pc, ok := f.conns[key]
	f.mu.RUnlock()
	if ok {
		return pc
	}

	// Before inserting a new connection for the key we need to check again
	// that it is really not set yet. In theory, some other goroutine could
	// have inserted it in the mean time.
	f.mu.Lock()
	defer f.mu.Unlock()
	if pc, ok := f.conns[key]; ok {
		return pc
	}
	pc = &pooledConn{}
	f.conns[key] = pc
	return pc
}

// evict removes the pooled entry for key from the map first and closes the
// connection afterwards, outside of f.mu.
//
// Deleting from the map does not destroy the pooledConn: pc is a pointer, and
// the local variable (plus any goroutine that already looked the entry up)
// keeps the object alive, so it is still safe to use after the delete. What
// the delete does achieve is that no *new* caller can find this entry; anyone
// arriving from now on creates a fresh entry and dials a new connection.
//
// Closing outside of f.mu matters because Close() writes to the network and
// can block. Holding the map lock across it would stall every other pool
// operation for the duration.
func (f *sshFactory) evict(key connKey) {
	f.mu.Lock()
	pc, ok := f.conns[key]
	if ok {
		delete(f.conns, key)
	}
	f.mu.Unlock()
	if !ok {
		return
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()
	if pc.client != nil {
		_ = pc.client.Close()
		pc.client = nil
	}
}

// EvictConnectionsForIP implements the EvictConnectionsForIP method of the
// factory interface. It is a no-op if no pooled connection exists for ip:
// callers use it as best-effort cleanup after a state transition, without
// checking beforehand whether a connection is actually pooled.
//
// It removes the entries from the map before closing them, for the same
// reasons as evict() above.
func (f *sshFactory) EvictConnectionsForIP(ip string) {
	f.mu.Lock()
	var toClose []*pooledConn
	for key, pc := range f.conns {
		if key.ip == ip {
			toClose = append(toClose, pc)
			delete(f.conns, key)
		}
	}
	f.mu.Unlock()

	for _, pc := range toClose {
		pc.mu.Lock()
		if pc.client != nil {
			_ = pc.client.Close()
			pc.client = nil
		}
		pc.mu.Unlock()
	}
}

// sweepIdleConns periodically closes pooled connections that have been idle
// for longer than connIdleTimeout. It runs until ctx is done, at which point
// it closes every remaining pooled connection.
func (f *sshFactory) sweepIdleConns(ctx context.Context) {
	ticker := time.NewTicker(f.sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			f.closeAll()
			return
		case <-ticker.C:
			f.evictIdle()
		}
	}
}

func (f *sshFactory) evictIdle() {
	now := time.Now()

	f.mu.Lock()
	var toClose []*pooledConn
	for key, pc := range f.conns {
		pc.mu.Lock()
		idle := pc.client != nil && now.Sub(pc.lastUsed) > f.idleTimeout
		pc.mu.Unlock()
		if idle {
			toClose = append(toClose, pc)
			delete(f.conns, key)
		}
	}
	f.mu.Unlock()

	for _, pc := range toClose {
		pc.mu.Lock()
		if pc.client != nil {
			_ = pc.client.Close()
			pc.client = nil
		}
		pc.mu.Unlock()
	}
}

func (f *sshFactory) closeAll() {
	f.mu.Lock()
	conns := f.conns
	f.conns = make(map[connKey]*pooledConn)
	f.mu.Unlock()

	for _, pc := range conns {
		pc.mu.Lock()
		if pc.client != nil {
			_ = pc.client.Close()
			pc.client = nil
		}
		pc.mu.Unlock()
	}
}

type sshClient struct {
	ip            string
	privateSSHKey string
	port          int
	factory       *sshFactory
}

func (c *sshClient) connKey() connKey {
	return newConnKey(c.ip, c.port, c.privateSSHKey)
}

// isTransportError reports whether err indicates a problem with the
// underlying SSH transport (dead connection, broken session) rather than a
// remote command that simply exited non-zero. Only transport errors should
// evict a pooled connection; a remote command exiting non-zero is a normal,
// expected outcome that must not throw away a healthy connection.
//
// Note: *ssh.ExitMissingError (session torn down without an exit status) is
// treated as a transport error here, even though it is also the expected
// outcome for a command like "reboot" that kills its own session: the
// golang.org/x/crypto/ssh session layer returns this exact error both when a
// session is closed cleanly without an exit status and when the underlying
// connection dies mid-command, so the two cannot be told apart at this
// level. That's fine because evicting only ever removes the pooled entry; it
// does not alter or retry the command, so it cannot corrupt the returned
// Output the way a retry-and-replace would.
func isTransportError(err error) bool {
	if err == nil {
		return false
	}
	var exitErr *ssh.ExitError
	return !errors.As(err, &exitErr)
}

// evictUnlessCanceled evicts the pooled connection for this client, unless
// ctx was canceled. The scp-based methods below evict on any copy failure,
// since go-scp does not expose a way to distinguish a genuine transport
// failure from a remote-side protocol error (e.g. "no such file"). But a
// canceled ctx says nothing about the connection's health -- it only means
// the caller stopped waiting -- so it must never trigger an eviction.
func (c *sshClient) evictUnlessCanceled(ctx context.Context) {
	if ctx.Err() == nil {
		c.factory.evict(c.connKey())
	}
}

var _ = Client(&sshClient{})

// GetHostName implements the GetHostName method of the SSHClient interface.
func (c *sshClient) GetHostName(ctx context.Context) Output {
	return c.runSSH(ctx, "hostname")
}

// GetHardwareDetailsRAM implements the GetHardwareDetailsRAM method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsRAM(ctx context.Context) Output {
	return c.runSSH(ctx, "grep MemTotal /proc/meminfo | awk '{print $2}'")
}

// GetHardwareDetailsNics implements the GetHardwareDetailsNics method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsNics(ctx context.Context) Output {
	return c.runSSH(ctx, fmt.Sprintf(`cat >/root/nic-info.sh <<'EOF_VIA_SSH'
%s
EOF_VIA_SSH
chmod a+rx /root/nic-info.sh
/root/nic-info.sh
`, nicInfoShellScript))
}

// GetHardwareDetailsStorage implements the GetHardwareDetailsStorage method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsStorage(ctx context.Context) Output {
	return c.runSSH(ctx, `lsblk -b -P -o "NAME,TYPE,SIZE,VENDOR,MODEL,SERIAL,WWN,HCTL,ROTA"`)
}

// GetHardwareDetailsCPUArch implements the GetHardwareDetailsCPUArch method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsCPUArch(ctx context.Context) Output {
	return c.runSSH(ctx, `lscpu | grep "Architecture:" | awk '{print $2}'`)
}

// GetHardwareDetailsCPUModel implements the GetHardwareDetailsCPUModel method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsCPUModel(ctx context.Context) Output {
	return c.runSSH(ctx, `lscpu | grep "Model name:" | awk '{$1=$2=""; print $0}' | sed "s/^[ \t]*//"`)
}

// GetHardwareDetailsCPUClockGigahertz implements the GetHardwareDetailsCPUClockGigahertz method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsCPUClockGigahertz(ctx context.Context) Output {
	return c.runSSH(ctx, `lscpu | grep "CPU max MHz:" |  awk '{printf "%.1f", $4/1000}'`)
}

// GetHardwareDetailsCPUFlags implements the GetHardwareDetailsCPUFlags method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsCPUFlags(ctx context.Context) Output {
	return c.runSSH(ctx, `lscpu | grep "Flags:" |  awk '{ $1=""; print $0}' | sed "s/^[ \t]*//"`)
}

// GetCloudInitOutput implements the GetCloudInitOutput method of the SSHClient interface.
func (c *sshClient) GetCloudInitOutput(ctx context.Context) Output {
	out := c.runSSH(ctx, `cat /var/log/cloud-init-output.log`)
	if out.Err == nil {
		out.StdOut = removeUselessLinesFromCloudInitOutput(out.StdOut)
	}
	return out
}

// GetHardwareDetailsCPUThreads implements the GetHardwareDetailsCPUThreads method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsCPUThreads(ctx context.Context) Output {
	return c.runSSH(ctx, `lscpu | grep "CPU(s):" | head -1 |  awk '{ print $2}'`)
}

// GetHardwareDetailsCPUCores implements the GetHardwareDetailsCPUCores method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsCPUCores(ctx context.Context) Output {
	return c.runSSH(ctx, `grep 'cpu cores' /proc/cpuinfo | uniq | awk '{print $4}'`)
}

// GetHardwareDetailsDebug implements the GetHardwareDetailsDebug method of the SSHClient interface.
func (c *sshClient) GetHardwareDetailsDebug(ctx context.Context) Output {
	return c.runSSH(ctx, `ip a; echo ==========----------==========;
	ethtool "*"; echo ==========----------==========;
	lspci; echo ==========----------==========;
	`)
}

// CreateAutoSetup implements the CreateAutoSetup method of the SSHClient interface.
func (c *sshClient) CreateAutoSetup(ctx context.Context, data string) Output {
	return c.runSSH(ctx, fmt.Sprintf(`cat << 'EOF_VIA_SSH' > /autosetup
%s
EOF_VIA_SSH`, data))
}

// DownloadImage implements the DownloadImage method of the SSHClient interface.
func (c *sshClient) DownloadImage(ctx context.Context, path, url string) Output {
	if !strings.HasPrefix(url, "oci://") {
		return c.runSSH(ctx, fmt.Sprintf(`curl -sLo "%q" "%q"`, path, url))
	}
	return c.runSSH(ctx, fmt.Sprintf(`cat << 'ENDOFSCRIPT' > /root/download-from-oci.sh
%s
ENDOFSCRIPT
chmod a+rx /root/download-from-oci.sh
OCI_REGISTRY_AUTH_TOKEN=%s /root/download-from-oci.sh %s %s`, downloadFromOciShellScript,
		os.Getenv("OCI_REGISTRY_AUTH_TOKEN"),
		strings.TrimPrefix(url, "oci://"), path))
}

// CreatePostInstallScript implements the CreatePostInstallScript method of the SSHClient interface.
func (c *sshClient) CreatePostInstallScript(ctx context.Context, data string) Output {
	out := c.runSSH(ctx, fmt.Sprintf(`cat << 'EOF_VIA_SSH' > /root/post-install.sh
%s
EOF_VIA_SSH`, data))

	if out.Err != nil || out.StdErr != "" {
		return out
	}
	return c.runSSH(ctx, `chmod +x /root/post-install.sh`)
}

// GetInstallImageState returns the running installimage processes.
func (c *sshClient) GetInstallImageState(ctx context.Context) (InstallImageState, error) {
	out := c.runSSH(ctx, `ps aux| grep installimage | grep -v grep; true`)
	if out.Err != nil {
		return "", fmt.Errorf("failed to run `ps aux` to get running installimage process: %w", out.Err)
	}
	if out.StdOut != "" {
		// installimage is running
		return InstallImageStateRunning, nil
	}

	out = c.runSSH(ctx, `[ -e /root/installimage-wrapper.sh.log ]`)
	exitStatus, err := out.ExitStatus()
	if err != nil {
		return "", fmt.Errorf("failed to check if installimage-wrapper.sh.log exists: %w", err)
	}
	if exitStatus == 0 {
		// above log file exists, but installimage is not running: finished.
		return InstallImageStateFinished, nil
	}
	// installimage is not running and the log file does not exist: not started yet.
	return InstallImageStateNotStartedYet, nil
}

// ExecuteInstallImage implements the ExecuteInstallImage method of the SSHClient interface.
func (c *sshClient) ExecuteInstallImage(ctx context.Context, hasPostInstallScript bool) Output {
	var cmd string
	if hasPostInstallScript {
		cmd = `/root/hetzner-installimage/installimage -a -c /autosetup -x /root/post-install.sh`
	} else {
		cmd = `/root/hetzner-installimage/installimage -a -c /autosetup`
	}

	out := c.runSSH(ctx, fmt.Sprintf(`cat << 'EOF_VIA_SSH' > /root/installimage-wrapper.sh
#!/bin/bash
export TERM=xterm

# don't wait 20 seconds before starting: echo "x"
echo "x" | %s
EOF_VIA_SSH`, cmd))
	if out.Err != nil || out.StdErr != "" {
		return out
	}

	out = c.runSSH(ctx, `chmod +x /root/installimage-wrapper.sh . `)
	if out.Err != nil || out.StdErr != "" {
		return out
	}

	return c.runSSH(ctx, `nohup /root/installimage-wrapper.sh >/root/installimage-wrapper.sh.log 2>&1 </dev/null &`)
}

// GetResultOfInstallImage returns the logs of install-image.
// Before calling this method be sure that installimage is already terminated.
func (c *sshClient) GetResultOfInstallImage(ctx context.Context) (string, error) {
	out := c.runSSH(ctx, `cat /root/debug.txt`)
	if out.Err != nil {
		return "", fmt.Errorf("failed to get debug.txt: %w", out.Err)
	}
	debugTxt := out.StdOut

	out = c.runSSH(ctx, `cat /root/installimage-wrapper.sh.log`)
	if out.Err != nil {
		return "", fmt.Errorf("failed to get installimage-wrapper.sh.log: %w", out.Err)
	}
	wrapperLog := out.StdOut

	return fmt.Sprintf(`debug.txt:
%s

######################################

/root/installimage-wrapper.sh stdout+stderr:

%s
`,
		debugTxt, wrapperLog), nil
}

// Reboot implements the Reboot method of the SSHClient interface.
func (c *sshClient) Reboot(ctx context.Context) Output {
	out := c.runSSH(ctx, `reboot`)
	if out.Err != nil && strings.Contains(out.Err.Error(), ErrCommandExitedWithoutExitSignal.Error()) {
		return Output{}
	}
	return out
}

// CloudInitStatus implements the CloudInitStatus method of the SSHClient interface.
func (c *sshClient) CloudInitStatus(ctx context.Context) Output {
	return c.runSSH(ctx, "cloud-init status")
}

// CheckCloudInitLogsForSigTerm implements the CheckCloudInitLogsForSigTerm method of the SSHClient interface.
func (c *sshClient) CheckCloudInitLogsForSigTerm(ctx context.Context) Output {
	out := c.runSSH(ctx, `cat /var/log/cloud-init.log | grep "SIGTERM"`)
	if out.Err != nil {
		exitStatus, err := out.ExitStatus()
		if err == nil && exitStatus == 1 {
			// grep exits with status 1 when no matching line is found.
			// That's expected in this check and should not fail reconciliation.
			return Output{}
		}
	}
	return out
}

// CleanCloudInitLogs implements the CleanCloudInitLogs method of the SSHClient interface.
func (c *sshClient) CleanCloudInitLogs(ctx context.Context) Output {
	return c.runSSH(ctx, `cloud-init clean --logs`)
}

// CleanCloudInitInstances implements the CleanCloudInitInstances method of the SSHClient interface.
func (c *sshClient) CleanCloudInitInstances(ctx context.Context) Output {
	return c.runSSH(ctx, `rm -rf /var/lib/cloud/instances`)
}

// ResetKubeadm implements the ResetKubeadm method of the SSHClient interface.
func (c *sshClient) ResetKubeadm(ctx context.Context) Output {
	// if `kubeadm reset` fails, we disable all pods and related services explicitly.
	output := c.runSSH(ctx, `kubeadm reset -f 2>&1
	echo
	echo ========= stopping all pods =========
	crictl pods -q | while read -r podid; do
      crictl stopp "$podid"
    done
	echo
	echo ========= disabling kubelet =========
	systemctl disable --now kubelet
	echo
	echo ========= deleting directories =========
	rm -rf /etc/kubernetes /var/run/kubeadm /var/lib/etcd
	echo ========= done =========
`)
	return output
}

func (c *sshClient) DetectLinuxOnAnotherDisk(ctx context.Context, sliceOfWwns []string) Output {
	return c.runSSH(ctx, fmt.Sprintf(`cat >/root/detect-linux-on-another-disk.sh <<'EOF_VIA_SSH'
%s
EOF_VIA_SSH
chmod a+rx /root/detect-linux-on-another-disk.sh
/root/detect-linux-on-another-disk.sh %s
`, detectLinuxOnAnotherDiskShellScript, strings.Join(sliceOfWwns, " ")))
}

var (
	// I found no details about the format. I found these examples:
	// 10:00:00:05:1e:7a:7a:00 eui.00253885910c8cec 0x500a07511bb48b25 alias.CDIMS1_A3:20:54:d0:39:ea:3d:b8:74
	// https://www.reddit.com/r/zfs/comments/1glttvl/validate_wwn/
	isValidWWNRegex = regexp.MustCompile(`^[0-9a-zA-Z._=:-]{5,64}$`)

	// ErrInvalidWWN indicates that a WWN has an invalid syntax.
	ErrInvalidWWN = fmt.Errorf("WWN does not match regex %q", isValidWWNRegex.String())
)

func (c *sshClient) WipeDisk(ctx context.Context, sliceOfWwns []string) (string, error) {
	log := ctrl.LoggerFrom(ctx)
	if len(sliceOfWwns) == 0 {
		return "", nil
	}
	if slices.Contains(sliceOfWwns, "all") {
		out := c.runSSH(ctx, "lsblk --nodeps --noheadings -o WWN | sort -u")
		if out.Err != nil {
			return "", fmt.Errorf("failed to find WWNs of all disks: %w", out.Err)
		}
		log.Info("WipeDisk: 'all' was given. Found these WWNs", "WWNs", sliceOfWwns)
		sliceOfWwns = strings.Fields(out.StdOut)
	} else {
		for _, wwn := range sliceOfWwns {
			// validate WWN.
			// It is unlikely, but someone could use this wwn: `"; do-nasty-things-here`
			if !isValidWWNRegex.MatchString(wwn) {
				return "", fmt.Errorf("WWN %q is invalid. %w", wwn, ErrInvalidWWN)
			}
		}
	}
	out := c.runSSH(ctx, fmt.Sprintf(`cat >/root/wipe-disk.sh <<'EOF_VIA_SSH'
%s
EOF_VIA_SSH
chmod a+rx /root/wipe-disk.sh
/root/wipe-disk.sh %s
`, wipeDiskShellScript, strings.Join(sliceOfWwns, " ")))
	if out.Err != nil {
		return "", fmt.Errorf("WipeDisk for %+v failed: %s. %s: %w", sliceOfWwns, out.StdOut, out.StdErr, out.Err)
	}
	return out.String(), nil
}

func (c *sshClient) CheckDisk(ctx context.Context, sliceOfWwns []string) (info string, err error) {
	if len(sliceOfWwns) == 0 {
		return "", nil
	}

	out := c.runSSH(ctx, fmt.Sprintf(`cat >/root/check-disk.sh <<'EOF_VIA_SSH'
%s
EOF_VIA_SSH
chmod a+rx /root/check-disk.sh
/root/check-disk.sh %s
`, checkDiskShellScript, strings.Join(sliceOfWwns, " ")))
	exitStatus, err := out.ExitStatus()
	if err != nil {
		// Network error or similar. Script was not called.
		return "", fmt.Errorf("CheckDisk for %+v failed: %w", sliceOfWwns, err)
	}
	if exitStatus == 1 {
		// Script detected a broken disk.
		return "", fmt.Errorf("CheckDisk for %+v failed: %s. %s. %w %w", sliceOfWwns, out.StdOut, out.StdErr, out.Err, ErrCheckDiskBrokenDisk)
	}
	if exitStatus == 0 {
		// Everything was fine.
		return out.String(), nil
	}
	// Some other strange error like "unknown WWN"
	return "", fmt.Errorf("CheckDisk for %+v failed: %s. %s: %w", sliceOfWwns, out.StdOut, out.StdErr, out.Err)
}

func (c *sshClient) UntarTGZ(ctx context.Context) Output {
	// read tgz from container image.
	fileName := "/installimage.tgz"
	data, err := os.ReadFile(fileName)
	if err != nil {
		return Output{Err: fmt.Errorf("ReadInstallimageTgzFailed %s: %w", fileName, err)}
	}

	// send base64 encoded binary to machine via ssh.
	return c.runSSH(ctx, fmt.Sprintf("echo %s | base64 -d | tar -xzf-",
		base64.StdEncoding.EncodeToString(data)))
}

// IsConnectionRefusedError checks whether the ssh error is a connection refused error.
func IsConnectionRefusedError(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED)
}

// IsAuthenticationFailedError checks whether the ssh error is an authentication failed error.
func IsAuthenticationFailedError(err error) bool {
	return strings.Contains(err.Error(), ErrAuthenticationFailed.Error())
}

// IsCommandExitedWithoutExitSignalError checks whether the ssh error is an unplanned exit error.
func IsCommandExitedWithoutExitSignalError(err error) bool {
	return strings.Contains(err.Error(), ErrCommandExitedWithoutExitSignal.Error())
}

// IsTimeoutError checks whether the ssh error is an unplanned exit error.
func IsTimeoutError(err error) bool {
	return strings.Contains(err.Error(), ErrTimeout.Error())
}

// getSSHClient returns a shared *ssh.Client for this machine, reusing a
// pooled connection when one exists and is still alive, dialing a fresh one
// otherwise. The returned client must not be closed by the caller: it is
// owned by the pool and may be in use by other callers concurrently or
// reused by a later call for the same machine.
//
// pc.mu is held for the entire call, including the liveness probe and, on a
// miss, the full dial (TCP connect + SSH handshake) -- worst case around
// 2*sshTimeOut. That serializes concurrent callers targeting the same
// (ip, port, keyHash), which is deliberate: it guarantees at most one dial in
// flight per pooled entry, so two callers racing to establish the same
// connection can't both pay for a handshake or clobber each other's pc.client
// write. Releasing the lock before dialing would remove that guarantee. This
// is a non-issue in practice because each host is normally driven by a single
// reconciler goroutine at a time; if that ever changes for a given (ip, port,
// key), calls to it will queue up behind this lock rather than run
// concurrently.
func (c *sshClient) getSSHClient(ctx context.Context) (*ssh.Client, error) {
	pc := c.factory.entry(c.connKey())

	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.client != nil {
		if isConnAlive(pc.client) {
			pc.lastUsed = time.Now()
			return pc.client, nil
		}
		_ = pc.client.Close()
		pc.client = nil
	}

	client, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	pc.client = client
	pc.lastUsed = time.Now()
	return client, nil
}

// isConnAlive does a cheap liveness probe on an existing connection so a
// pooled connection killed by the remote end (reboot, idle timeout, ...)
// isn't handed out as if it were still usable.
//
// The probe works because the SSH protocol (RFC 4254 4) requires a peer that
// gets a global request it doesn't understand to still reply, with failure,
// if a reply was requested. So client.SendRequest is expected to come back
// with ok=false here -- that's ignored. Only err is checked: err == nil means
// some reply arrived at all, i.e. the transport is still processing
// messages; err != nil (or the timeout below firing first) means it isn't.
func isConnAlive(client *ssh.Client) bool {
	done := make(chan bool, 1)
	go func() {
		_, _, err := client.SendRequest(connKeepAliveRequest, true, nil)
		done <- err == nil
	}()

	select {
	case ok := <-done:
		return ok
	case <-time.After(sshTimeOut):
		return false
	}
}

// dial opens a brand new TCP connection and performs the SSH handshake.
func (c *sshClient) dial(ctx context.Context) (*ssh.Client, error) {
	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey([]byte(c.privateSSHKey))
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key (%s): %w", c.connectionDetails(), err)
	}

	config := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //#nosec
		Timeout:         sshTimeOut,
	}

	addr := net.JoinHostPort(c.ip, strconv.Itoa(c.port))

	// ctx-aware TCP dial.
	d := net.Dialer{Timeout: sshTimeOut}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		// Return ctx.Err() unwrapped so os.IsTimeout detects it.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("failed to dial ssh (%s): %w", c.connectionDetails(), err)
	}

	// If ctx fires during the SSH handshake, close the underlying conn so
	// NewClientConn returns. stop() deregisters the callback on normal exit.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("failed ssh handshake (%s): %w", c.connectionDetails(), err)
	}
	return ssh.NewClient(sshConn, chans, reqs), nil
}

// runSSH runs command over the pooled connection for this machine. If the
// command fails due to a transport-level problem (dead connection, broken
// session) rather than merely exiting non-zero, the pooled entry is evicted
// so the next call dials a fresh connection. The failed command itself is
// not retried here: retrying blindly risks re-issuing a non-idempotent
// command that may already have run on the remote side before the transport
// failed, and the reconcile loop's own polling already re-issues it safely.
func (c *sshClient) runSSH(ctx context.Context, command string) Output {
	logger := ctrl.LoggerFrom(ctx).WithName("ssh-client")

	client, err := c.getSSHClient(ctx)
	if err != nil {
		return Output{Err: err}
	}

	sess, err := client.NewSession()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Output{Err: ctxErr}
		}
		c.factory.evict(c.connKey())
		return Output{Err: fmt.Errorf("unable to create new ssh session (%s): %w", c.connectionDetails(), err)}
	}

	// If ctx fires, close the session (not the shared client) so any in-flight
	// sess.Run returns. stop() deregisters the callback on normal exit.
	stop := context.AfterFunc(ctx, func() { _ = sess.Close() })
	defer stop()

	defer func() {
		if err := sess.Close(); err != nil && !errors.Is(err, io.EOF) {
			logger.Error(err, "failed to close ssh session")
		}
	}()

	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer

	sess.Stdout = &stdoutBuffer
	sess.Stderr = &stderrBuffer

	err = sess.Run(command)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Output{
			StdOut: stdoutBuffer.String(),
			StdErr: stderrBuffer.String(),
			Err:    ctxErr,
		}
	}

	if isTransportError(err) {
		c.factory.evict(c.connKey())
	}
	if err != nil {
		err = fmt.Errorf("ssh command failed (%s): %w", c.connectionDetails(), err)
	}
	return Output{
		StdOut: stdoutBuffer.String(),
		StdErr: stderrBuffer.String(),
		Err:    err,
	}
}

func (c *sshClient) connectionDetails() string {
	return fmt.Sprintf("user=%s host=%s port=%d timeout=%s", sshUser, c.ip, c.port, sshTimeOut)
}

func removeUselessLinesFromCloudInitOutput(s string) string {
	regexes := []string{
		`^\s*\d+K [. ]+ \d+%.*\ds$`,                  //  10000K .......... .......... .......... .......... ..........  6%!M(MISSING) 1s
		`^Get:\d+ https?://.* [kM]B.*`,               // Get:17 http://archive.ubuntu.com/ubuntu focal/universe Translation-en [5,124 kB[]`
		`^Preparing to unpack \.\.\..*`,              // Preparing to unpack .../04-libx11-6_2%!a(MISSING)1.6.9-2ubuntu1.6_amd64.deb ...\r
		`^Selecting previously unselected package.*`, // Selecting previously unselected package kubeadm.\r
		`^Setting up .* \.\.\..*`,                    // Setting up hicolor-icon-theme (0.17-2) ...\r
		`^Unpacking .* \.\.\..*`,                     // Unpacking libatk1.0-0:amd64 (2.35.1-1ubuntu2) ...\r
	}

	// Compile the regexes
	compiledRegexes := make([]*regexp.Regexp, 0, len(regexes))
	for _, re := range regexes {
		compiled, err := regexp.Compile(re)
		if err != nil {
			return fmt.Sprintf("removeUselessLinesFromCloudInitOutput: Failed to compile regex %s: %v\n%s",
				re, err, s)
		}
		compiledRegexes = append(compiledRegexes, compiled)
	}

	var output []string

	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		line := scanner.Text()

		// Check if the line matches any of the regexes
		matches := false
		for _, re := range compiledRegexes {
			if re.MatchString(line) {
				matches = true
				break
			}
		}

		if matches {
			continue
		}
		output = append(output, line)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Sprintf("Error reading string: %v\n%s", err, s)
	}
	return strings.Join(output, "\n")
}

func (c *sshClient) ExecutePreProvisionCommand(ctx context.Context, command string) (int, string, error) {
	client, err := c.getSSHClient(ctx)
	if err != nil {
		return 0, "", err
	}

	scpClient, err := scp.NewClientBySSH(client)
	if err != nil {
		c.factory.evict(c.connKey())
		return 0, "", fmt.Errorf("couldn't create a new scp client: %w", err)
	}

	defer scpClient.Close()

	f, err := os.Open(command) //nolint:gosec // the variable was valided.
	if err != nil {
		return 0, "", fmt.Errorf("error opening file %q: %w", command, err)
	}

	baseName := filepath.Base(command)
	dest := "/root/" + baseName
	err = scpClient.CopyFromFile(ctx, *f, dest, "0700")
	if err != nil {
		c.evictUnlessCanceled(ctx)
		return 0, "", fmt.Errorf("error copying file %q to %s:%d:%s %w", command, c.ip, c.port, dest, err)
	}

	out := c.runSSH(ctx, dest)
	exitStatus, err := out.ExitStatus()
	if err != nil {
		return 0, "", fmt.Errorf("error executing %q on %s:%d: %w", dest, c.ip, c.port, err)
	}

	s := out.StdOut + "\n" + out.StdErr
	s = strings.TrimSpace(s)

	return exitStatus, s, nil
}

func (c *sshClient) StartImageURLCommand(ctx context.Context, command, imageURL string, bootstrapData []byte, machineName string, deviceNames []string) (int, string, error) {
	logger := ctrl.LoggerFrom(ctx).WithName("ssh-client")

	// validate deviceNames
	for _, dn := range deviceNames {
		if strings.Contains(dn, "/") {
			return 0, "", fmt.Errorf("deviceName must not contain a slash (example: only sda not /dev/sda): %q", dn)
		}
		if strings.Contains(dn, " ") {
			return 0, "", fmt.Errorf("deviceName must not contain spaces: %q", dn)
		}
		if dn == "" {
			return 0, "", errors.New("deviceName must not be empty")
		}
	}

	if command == "" {
		return 0, "", fmt.Errorf("image-url-command is empty")
	}

	fdCommand, err := os.Open(command) //nolint:gosec // the variable was valided.
	if err != nil {
		return 0, "", fmt.Errorf("error opening image-url-command %q: %w", command, err)
	}
	defer func() {
		if err := fdCommand.Close(); err != nil {
			logger.Error(err, "failed to close image-url-command file", "path", command)
		}
	}()

	client, err := c.getSSHClient(ctx)
	if err != nil {
		return 0, "", err
	}

	scpClient, err := scp.NewClientBySSH(client)
	if err != nil {
		c.factory.evict(c.connKey())
		return 0, "", fmt.Errorf("couldn't create a new scp client: %w", err)
	}

	defer scpClient.Close()

	baseName := "image-url-command"
	dest := "/root/" + baseName
	err = scpClient.CopyFromFile(ctx, *fdCommand, dest, "0700")
	if err != nil {
		c.evictUnlessCanceled(ctx)
		return 0, "", fmt.Errorf("error copying file %q to %s:%d:%s %w", command, c.ip, c.port, dest, err)
	}

	reader := bytes.NewReader(bootstrapData)
	dest = "/root/bootstrap.data"
	err = scpClient.CopyFile(ctx, reader, dest, "0700")
	if err != nil {
		c.evictUnlessCanceled(ctx)
		return 0, "", fmt.Errorf("error copying bootstrap data to %s:%d:%s %w", c.ip, c.port, dest, err)
	}

	cmd := fmt.Sprintf(`#!/usr/bin/bash
OCI_REGISTRY_AUTH_TOKEN='%s' nohup /root/image-url-command '%s' /root/bootstrap.data '%s' '%s' >%s 2>&1 </dev/null &
echo $! > /root/image-url-command.pid
`, os.Getenv("OCI_REGISTRY_AUTH_TOKEN"), imageURL, machineName, strings.Join(deviceNames, " "),
		imageURLCommandLog)

	out := c.runSSH(ctx, cmd)

	exitStatus, err := out.ExitStatus()
	if err != nil {
		return 0, "", fmt.Errorf("error executing %q on %s:%d: %w", dest, c.ip, c.port, err)
	}

	s := out.StdOut + "\n" + out.StdErr
	s = strings.TrimSpace(s)

	return exitStatus, s, nil
}

func (c *sshClient) StateOfImageURLCommand(ctx context.Context) (state ImageURLCommandState, stdoutStderr string, err error) {
	out := c.runSSH(ctx, `[ -e /root/image-url-command.pid ]`)
	exitStatus, err := out.ExitStatus()
	if err != nil {
		return ImageURLCommandStateNotStarted, "", fmt.Errorf("getting exit status of custom provisioner failed: %w", err)
	}
	if exitStatus > 0 {
		// file does exists
		return ImageURLCommandStateNotStarted, "", nil
	}

	out = c.runSSH(ctx, `ps -p "$(cat /root/image-url-command.pid)" -o args= | grep -q image-url-command`)
	exitStatus, err = out.ExitStatus()
	if err != nil {
		return ImageURLCommandStateNotStarted, "", fmt.Errorf("detecting if image-url-command is still running failed: %w", err)
	}

	logFile, err := c.getImageURLCommandOutput(ctx)
	if err != nil {
		return ImageURLCommandStateFailed, logFile, err
	}

	if exitStatus == 0 {
		return ImageURLCommandStateRunning, logFile, nil
	}

	out = c.runSSH(ctx, fmt.Sprintf("tail -n 1 %s | grep -q IMAGE_URL_DONE", imageURLCommandLog))
	exitStatus, err = out.ExitStatus()
	if err != nil {
		return ImageURLCommandStateNotStarted, logFile, fmt.Errorf("detecting if image-url-command was successful failed: %w", err)
	}

	if exitStatus > 0 {
		return ImageURLCommandStateFailed,
			fmt.Sprintf("IMAGE_URL_DONE not found in %s:\n%s", imageURLCommandLog, logFile), nil
	}
	return ImageURLCommandStateFinishedSuccessfully, logFile, nil
}

func (c *sshClient) getImageURLCommandOutput(ctx context.Context) (string, error) {
	out := c.runSSH(ctx, fmt.Sprintf("cat %s", imageURLCommandLog)) // TODO: implement getFile for sshClient.
	exitStatus, err := out.ExitStatus()
	if err != nil {
		return "", fmt.Errorf("getting logs of custom provisioner failed: %w", err)
	}
	if exitStatus > 0 {
		return "", fmt.Errorf("getting logs of custom provisioner failed. Non zero status of 'cat'")
	}
	return out.StdOut, nil
}

func (c *sshClient) ReadOutputJSON(ctx context.Context) (string, error) {
	client, err := c.getSSHClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get ssh client: %w", err)
	}

	scpClient, err := scp.NewClientBySSH(client)
	if err != nil {
		c.factory.evict(c.connKey())
		return "", fmt.Errorf("failed to create scp client: %w", err)
	}
	defer scpClient.Close()

	for range outputJSONMaxRetries {
		out := c.runSSH(ctx, "test -f "+outputJSONPath)
		exitStatus, err := out.ExitStatus()
		if err != nil {
			return "", fmt.Errorf("failed to test if output.json exists: %w", err)
		}
		if exitStatus != 0 {
			return "", nil // file does not exist
		}

		var buf bytes.Buffer
		if err := scpClient.CopyFromRemotePassThru(ctx, &buf, outputJSONPath, nil); err != nil {
			c.evictUnlessCanceled(ctx)
			return "", fmt.Errorf("failed to copy output.json from rescue system to caph: %w", err)
		}

		content := strings.TrimSpace(buf.String())
		if content == "" {
			return "", nil
		}
		if strings.HasSuffix(content, "}") {
			return content, nil
		}

		time.Sleep(50 * time.Millisecond)
	}

	return "", fmt.Errorf("output.json did not end with '}' after %d attempts", outputJSONMaxRetries)
}
