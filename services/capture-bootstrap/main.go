// capture-bootstrap is an immutable startup gate, replaced by the application
// after host-side capture policy verification. No capture daemon stays running.
package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const address = "@adplatform-capture-start-v1"

type message struct {
	Op    string `json:"op"`
	Nonce string `json:"nonce,omitempty"`
	OK    bool   `json:"ok,omitempty"`
}

func exchange(op, nonce string) (message, error) {
	c, err := net.DialTimeout("unix", address, time.Second)
	if err != nil {
		return message{}, err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(time.Second))
	if err = json.NewEncoder(c).Encode(message{Op: op, Nonce: nonce}); err != nil {
		return message{}, err
	}
	var reply message
	err = json.NewDecoder(io.LimitReader(c, 512)).Decode(&reply)
	if err == nil && !reply.OK {
		err = fmt.Errorf("startup gate rejected request")
	}
	return reply, err
}

// A container-root process may rebind the fixed abstract socket after the
// gate execs the application. Docker init is PID 1; its single direct child
// is the entrypoint. Only that child may report readiness or release the gate.
func verifyGateLeader() error {
	children, err := os.ReadFile("/proc/1/task/1/children")
	if err != nil {
		return err
	}
	pids := strings.Fields(string(children))
	if len(pids) != 1 {
		return fmt.Errorf("capture startup gate is not the sole Docker init child")
	}
	leader, err := os.Stat(filepath.Join("/proc", pids[0], "exe"))
	if err != nil {
		return err
	}
	self, err := os.Stat("/proc/self/exe")
	if err != nil {
		return err
	}
	if os.SameFile(leader, self) {
		return nil
	}
	// The one-shot bootstrap initializer atomically replaces this read-only
	// volume file during upgrades. A gate already waiting on its old inode is
	// still trusted; only the immutable volume path can produce this link.
	path, err := os.Readlink(filepath.Join("/proc", pids[0], "exe"))
	if err != nil || path != "/opt/adplatform-capture/start (deleted)" {
		return fmt.Errorf("container entrypoint is not the capture startup gate")
	}
	return nil
}
func gate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("original image command is required")
	}
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return err
	}
	nonce := hex.EncodeToString(nonceBytes)
	listener, err := net.Listen("unix", address)
	if err != nil {
		return err
	}
	for {
		c, err := listener.Accept()
		if err != nil {
			listener.Close()
			return err
		}
		_ = c.SetDeadline(time.Now().Add(time.Second))
		var request message
		err = json.NewDecoder(io.LimitReader(c, 512)).Decode(&request)
		release := err == nil && request.Op == "release" && subtle.ConstantTimeCompare([]byte(request.Nonce), []byte(nonce)) == 1
		if err == nil && request.Op == "probe" {
			_ = json.NewEncoder(c).Encode(message{OK: true, Nonce: nonce})
		} else {
			_ = json.NewEncoder(c).Encode(message{OK: release})
		}
		c.Close()
		if release {
			break
		}
	}
	listener.Close()
	executable, err := lookPath(args[0])
	if err != nil {
		return err
	}
	return syscall.Exec(executable, args, os.Environ())
}
func lookPath(name string) (string, error) {
	if filepath.IsAbs(name) || filepath.Base(name) != name {
		return name, nil
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return path, nil
		}
	}
	return "", fmt.Errorf("original executable %q not found", name)
}
func main() {
	var err error
	switch {
	case len(os.Args) == 3 && os.Args[1] == "install":
		var binary []byte
		binary, err = os.ReadFile("/proc/self/exe")
		if err == nil {
			err = os.MkdirAll(os.Args[2], 0755)
		}
		if err == nil {
			temporary := filepath.Join(os.Args[2], ".start-new")
			err = os.WriteFile(temporary, binary, 0555)
			if err == nil {
				err = os.Chmod(temporary, 0555)
			}
			if err == nil {
				err = os.Rename(temporary, filepath.Join(os.Args[2], "start"))
			}
		}
	case len(os.Args) == 2 && os.Args[1] == "probe":
		var reply message
		err = verifyGateLeader()
		if err == nil {
			reply, err = exchange("probe", "")
		}
		if err == nil {
			fmt.Println(reply.Nonce)
		}
	case len(os.Args) == 3 && os.Args[1] == "release":
		err = verifyGateLeader()
		if err == nil {
			_, err = exchange("release", os.Args[2])
		}
	case len(os.Args) == 2 && os.Args[1] == "status":
		err = verifyGateLeader()
	case len(os.Args) >= 3 && os.Args[1] == "run":
		err = gate(os.Args[2:])
	default:
		err = fmt.Errorf("expected install, status, probe, release, or run COMMAND")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "capture startup gate:", err)
		os.Exit(1)
	}
}
