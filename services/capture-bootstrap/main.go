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
		reply, err = exchange("probe", "")
		if err == nil {
			fmt.Println(reply.Nonce)
		}
	case len(os.Args) == 3 && os.Args[1] == "release":
		_, err = exchange("release", os.Args[2])
	case len(os.Args) >= 3 && os.Args[1] == "run":
		err = gate(os.Args[2:])
	default:
		err = fmt.Errorf("expected install, probe, release, or run COMMAND")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "capture startup gate:", err)
		os.Exit(1)
	}
}
