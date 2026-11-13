package main

import (
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
)

func TestExchangeBoundsUntrustedResponses(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		wantError      bool
	}{
		{"valid", `{"ok":true,"nonce":"` + strings.Repeat("a", 64) + `"}`, false},
		{"rejected", `{"ok":false}`, true},
		{"oversized", `{"ok":true,"nonce":"` + strings.Repeat("a", 4096) + `"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("unix", address)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				c, err := listener.Accept()
				if err == nil {
					defer c.Close()
					var request message
					if json.NewDecoder(io.LimitReader(c, 512)).Decode(&request) == nil {
						_, _ = io.WriteString(c, tc.response)
					}
				}
			}()
			_, err = exchange("probe", "")
			if (err != nil) != tc.wantError {
				t.Errorf("exchange error=%v, wantError=%v", err, tc.wantError)
			}
			listener.Close()
			<-done
		})
	}
}
