// Copyright 2021 The golang.design Initiative Authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.
//
// Written by Changkun Ou <changkun.de>

//go:build darwin && !ios
// +build darwin,!ios

package clip

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa
#import <Foundation/Foundation.h>
#import <Cocoa/Cocoa.h>

unsigned int clipboard_read_string(void **out);
unsigned int clipboard_read_image(void **out);
int clipboard_write_string(const void *bytes, NSInteger n);
int clipboard_write_image(const void *bytes, NSInteger n);
NSInteger clipboard_change_count();
*/
import "C"
import (
	"context"
	"errors"
	"sync"
	"time"
	"unsafe"
)

// Format represents the format of clipboard data.
type Format int

// All sorts of supported clipboard data
const (
	// FmtText indicates plain text clipboard format
	FmtText Format = iota
	// FmtImage indicates image/png clipboard format
	FmtImage
)

var (
	// activate only for running tests.
	debug          = false
	errUnavailable = errors.New("clipboard unavailable")
	errUnsupported = errors.New("unsupported format")
)

var (
	// Due to the limitation on operating systems (such as darwin),
	// concurrent read can even cause panic, use a global lock to
	// guarantee one read at a time.
	lock      = sync.Mutex{}
	initOnce  sync.Once
	initError error
)

func Read(t Format) (buf []byte, err error) {
	lock.Lock()
	defer lock.Unlock()

	var (
		data unsafe.Pointer
		n    C.uint
	)
	switch t {
	case FmtText:
		n = C.clipboard_read_string(&data)
	case FmtImage:
		n = C.clipboard_read_image(&data)
	}
	if data == nil {
		return nil, errUnavailable
	}
	defer C.free(unsafe.Pointer(data))
	if n == 0 {
		return nil, nil
	}
	return C.GoBytes(data, C.int(n)), nil
}

func Write(t Format, buf []byte) (<-chan struct{}, error) {
	lock.Lock()
	defer lock.Unlock()

	var ok C.int
	switch t {
	case FmtText:
		if len(buf) == 0 {
			ok = C.clipboard_write_string(unsafe.Pointer(nil), 0)
		} else {
			ok = C.clipboard_write_string(unsafe.Pointer(&buf[0]),
				C.NSInteger(len(buf)))
		}
	case FmtImage:
		if len(buf) == 0 {
			ok = C.clipboard_write_image(unsafe.Pointer(nil), 0)
		} else {
			ok = C.clipboard_write_image(unsafe.Pointer(&buf[0]),
				C.NSInteger(len(buf)))
		}
	default:
		return nil, errUnsupported
	}
	if ok != 0 {
		return nil, errUnavailable
	}

	// use unbuffered data to prevent goroutine leak
	changed := make(chan struct{}, 1)
	cnt := C.long(C.clipboard_change_count())
	go func() {
		for {
			// not sure if we are too slow or the user too fast :)
			time.Sleep(time.Second)
			cur := C.long(C.clipboard_change_count())
			if cnt != cur {
				changed <- struct{}{}
				close(changed)
				return
			}
		}
	}()
	return changed, nil
}

func Watch(ctx context.Context, t Format) <-chan []byte {
	recv := make(chan []byte, 1)
	// not sure if we are too slow or the user too fast :)
	ti := time.NewTicker(time.Millisecond * 100)
	lastCount := C.long(C.clipboard_change_count())
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(recv)
				return
			case <-ti.C:
				this := C.long(C.clipboard_change_count())
				if lastCount != this {
					b, _ := Read(t)
					if b == nil {
						continue
					}
					recv <- b
					lastCount = this
				}
			}
		}
	}()
	return recv
}

func AdaptWatchDoubleText(ctx context.Context) <-chan string {
	recv := make(chan string, 1)
	// not sure if we are too slow or the user too fast :)
	ti := time.NewTicker(time.Millisecond * 200)
	lastCount := C.long(C.clipboard_change_count())
	missCount := 0
	lastText := ""
	lastMill := time.Now().UnixMilli()

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(recv)
				return
			case <-ti.C:
				this := C.long(C.clipboard_change_count())
				if lastCount != this {
					b, _ := Read(FmtText)
					if b == nil {
						continue
					}
					text := string(b)
					if text == "" {
						continue
					}
					currMill := time.Now().UnixMilli()
					if text == lastText && currMill-lastMill < 500 {
						recv <- text
						lastText = ""
					} else {
						lastText = text
					}
					lastMill = currMill
					lastCount = this
					ti.Reset(time.Duration(100) * time.Millisecond)
					missCount = 0
				}
				if lastCount == this {
					missCount++
					if missCount == 50 || missCount > 100 {
						ti.Reset(time.Millisecond * 200)
						missCount = 0
					}
				}
			}
		}
	}()
	return recv
}

func ClipboardCount() int {
	count := C.long(C.clipboard_change_count())
	return int(count)
}
