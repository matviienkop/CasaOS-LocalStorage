package service

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

const smartSample = `{"model_name":"test disk","serial_number":"first","device":{"type":"ata"},"temperature":{"current":40},"smart_status":{"passed":true}}`

func TestSMARTSleepAndRefresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100000, 0)
	calls, status := 0, 0
	body := smartSample
	c := &smartCache{entries: make(map[string]smartEntry), now: func() time.Time { return now },
		identify: func(path string) (os.FileInfo, string, error) { info, err := os.Stat(path); return info, "ata", err },
		read: func(path, kind string) ([]byte, int) {
			calls++
			if kind != "ata" {
				t.Errorf("type = %s", kind)
			}
			return []byte(body), status
		},
	}
	initial := c.get(path)
	if initial.Temperature.Current != 40 || initial.ReadState != "fresh" || initial.SampledAt != now.Unix() {
		t.Fatalf("unexpected first sample: %+v", initial)
	}
	now = now.Add(15*time.Minute - time.Second)
	c.get(path)
	if calls != 1 {
		t.Fatal("polled before TTL")
	}
	now = now.Add(time.Second)
	status, body = 3, `{}`
	for i := 0; i < 3; i++ {
		sleeping := c.get(path)
		if sleeping.ReadState != "sleeping" || sleeping.SampledAt != initial.SampledAt || sleeping.Temperature.Current != 40 {
			t.Fatalf("lost sleeping sample: %+v", sleeping)
		}
		c.get(path)
		if calls != i+2 {
			t.Fatal("sleeping disk was not deferred")
		}
		now = now.Add(smartPollInterval)
	}
	status, body = 0, `{"model_name":"test disk","temperature":{"current":36}}`
	awake := c.get(path)
	if awake.ReadState != "fresh" || awake.Temperature.Current != 36 || awake.SampledAt != now.Unix() {
		t.Fatalf("did not refresh: %+v", awake)
	}
}

func TestSMARTFailuresDoNotReuseOldReadings(t *testing.T) {
	for _, status := range []int{-1, 1, 2, 4, 5} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "disk")
			if err := os.WriteFile(path, nil, 0600); err != nil {
				t.Fatal(err)
			}
			now := time.Unix(100000, 0)
			code := 0
			calls := 0
			c := &smartCache{entries: make(map[string]smartEntry), now: func() time.Time { return now }, identify: func(p string) (os.FileInfo, string, error) { s, e := os.Stat(p); return s, "ata", e }, read: func(string, string) ([]byte, int) { calls++; return []byte(smartSample), code }}
			c.get(path)
			now = now.Add(smartPollInterval)
			code = status
			got := c.get(path)
			if got.ReadState != "unavailable" || got.SampledAt != 0 || got.Temperature.Current != 0 {
				t.Fatal("failure reused old SMART data")
			}
			c.get(path)
			if calls != 2 {
				t.Fatal("failure retried without backoff")
			}
		})
	}
}

func TestSMARTMalformedAndHealthWarning(t *testing.T) {
	for _, tt := range []struct {
		body   string
		status int
		state  string
	}{{"{", 0, "unavailable"}, {"{}", 0, "unavailable"}, {smartSample, 8, "fresh"}, {smartSample, 64, "fresh"}, {"{}", 3, "sleeping"}} {
		path := filepath.Join(t.TempDir(), "disk")
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		c := &smartCache{entries: make(map[string]smartEntry), now: time.Now, identify: func(p string) (os.FileInfo, string, error) { s, e := os.Stat(p); return s, "ata", e }, read: func(string, string) ([]byte, int) { return []byte(tt.body), tt.status }}
		got := c.get(path)
		if got.ReadState != tt.state {
			t.Fatalf("status %d body %s: %s", tt.status, tt.body, got.ReadState)
		}
		if tt.state == "sleeping" && got.SampledAt != 0 {
			t.Fatal("invented timestamp without an earlier reading")
		}
	}
}

func TestSMARTDisconnectAndDeviceReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "disk")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	c := &smartCache{entries: make(map[string]smartEntry), now: time.Now, identify: func(p string) (os.FileInfo, string, error) { s, e := os.Stat(p); return s, "ata", e }, read: func(string, string) ([]byte, int) { calls++; return []byte(smartSample), 0 }}
	c.get(path)
	if err := os.Rename(path, filepath.Join(dir, "old")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	c.get(path)
	if calls != 2 {
		t.Fatal("replacement inherited cached reading")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := c.get(path); got.ReadState != "unavailable" || got.SampledAt != 0 {
		t.Fatal("disconnected disk still has a sample")
	}
	if len(c.entries) != 0 {
		t.Fatal("disconnected entry retained")
	}
}

func TestSMARTConcurrentReadersShareOnePoll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	c := &smartCache{entries: make(map[string]smartEntry), now: time.Now, identify: func(p string) (os.FileInfo, string, error) { s, e := os.Stat(p); return s, "ata", e }, read: func(string, string) ([]byte, int) { calls++; return []byte(smartSample), 0 }}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); c.get(path) }()
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("%d polls", calls)
	}
}
