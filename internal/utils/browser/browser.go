// This code was adapted from the original source code:
// https://cs.opensource.google/go/go/+/master:src/cmd/internal/browser/browser.go
//
// Copyright (c) 2009 The Go Authors. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package browser

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
)

// Open tries to open url in a browser and reports whether it succeeded.
func Open(ctx context.Context, url string) error {
	var args []string
	if exe := os.Getenv("BROWSER"); exe != "" {
		args = []string{exe}
	} else {
		switch runtime.GOOS {
		case "darwin":
			args = []string{"/usr/bin/open"}
		case "windows":
			args = []string{"cmd", "/c", "start"}
		default:
			if os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "" {
				// xdg-open is only for use in a desktop environment.
				args = []string{"xdg-open"}
			}
		}
	}
	if args == nil {
		return errors.New("no open command found")
	}
	cmd := exec.CommandContext(ctx, args[0], append(args[1:], url)...)
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}
