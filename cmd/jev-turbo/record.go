package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sightmap/jev-turbo/explore"
	"github.com/sightmap/sightmap/go/browser"
)

// recorder captures the driven tab as JPEG frames over CDP while a goal runs,
// alongside the step events, so scripts/render-demo.py can assemble a video
// with captions. Capture shares the loop's CDP connection; a frame costs a few
// tens of milliseconds and runs on its own goroutine.
type recorder struct {
	dir    string
	t0     time.Time
	frames *os.File
	events *os.File
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	count  int
}

type frameRecord struct {
	T    int    `json:"t"` // ms since the recording started
	File string `json:"file"`
}

type eventRecord struct {
	T    int    `json:"t"`
	Step int    `json:"step"`
	Text string `json:"text"`
	View string `json:"view,omitempty"`
}

func startRecorder(ctx context.Context, conn *browser.CDPConn, dir string) (*recorder, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	frames, err := os.Create(filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return nil, err
	}
	events, err := os.Create(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithCancel(ctx)
	r := &recorder{dir: dir, t0: time.Now(), frames: frames, events: events, cancel: cancel, done: make(chan struct{})}
	go r.loop(cctx, conn)
	return r, nil
}

func (r *recorder) loop(ctx context.Context, conn *browser.CDPConn) {
	defer close(r.done)
	opts := browser.ScreenshotOptions{Format: "jpeg", Quality: 75, OptimizeForSpeed: true}
	for {
		t := time.Now()
		if data, err := browser.ScreenshotWithOptions(ctx, conn, opts); err == nil {
			r.mu.Lock()
			name := fmt.Sprintf("f%05d.jpg", r.count)
			r.count++
			if os.WriteFile(filepath.Join(r.dir, name), data, 0o644) == nil {
				b, _ := json.Marshal(frameRecord{T: int(t.Sub(r.t0).Milliseconds()), File: name})
				r.frames.Write(append(b, '\n'))
			}
			r.mu.Unlock()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(60 * time.Millisecond):
		}
	}
}

func (r *recorder) event(s explore.Step) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, _ := json.Marshal(eventRecord{T: int(time.Since(r.t0).Milliseconds()), Step: s.N, Text: s.Action, View: s.View})
	r.events.Write(append(b, '\n'))
}

func (r *recorder) stop(run *explore.Run) {
	// leave a moment of the final state on screen, then stop capturing
	time.Sleep(700 * time.Millisecond)
	r.cancel()
	<-r.done
	r.frames.Close()
	r.events.Close()
	if run != nil {
		data, _ := json.MarshalIndent(run, "", " ")
		os.WriteFile(filepath.Join(r.dir, "run.json"), data, 0o644)
	}
	fmt.Fprintf(os.Stderr, "recorded %d frames to %s\n", r.count, r.dir)
}
