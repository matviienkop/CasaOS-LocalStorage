package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/CasaOS-LocalStorage/model"
	"github.com/IceWhaleTech/CasaOS-LocalStorage/pkg/utils/command"
)

const smartPollInterval = 15 * time.Minute

type smartEntry struct {
	deviceType string
	data       model.SmartctlA
	device     os.FileInfo
	nextPoll   time.Time
}

type smartCache struct {
	mu       sync.Mutex
	entries  map[string]smartEntry
	now      func() time.Time
	identify func(string) (os.FileInfo, string, error)
	read     func(string, string) ([]byte, int)
}

var diskSMART = &smartCache{
	entries:  make(map[string]smartEntry),
	now:      time.Now,
	identify: smartDevice,
	read:     command.ExecSmartCTLByPath,
}

func smartDevice(path string) (os.FileInfo, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}
	devicePath, err := filepath.EvalSymlinks(filepath.Join("/sys/class/block", filepath.Base(path), "device"))
	if err == nil {
		for _, part := range strings.Split(devicePath, "/") {
			if strings.HasPrefix(part, "ata") {
				if _, err := strconv.Atoi(strings.TrimPrefix(part, "ata")); err == nil {
					return info, "ata", nil
				}
			}
		}
	}
	return info, "", nil
}

func (c *smartCache) get(path string) model.SmartctlA {
	c.mu.Lock()
	defer c.mu.Unlock()
	device, deviceType, err := c.identify(path)
	if err != nil {
		delete(c.entries, path)
		return model.SmartctlA{ReadState: "unavailable"}
	}
	entry := c.entries[path]
	if entry.device == nil || !os.SameFile(entry.device, device) {
		entry = smartEntry{device: device}
	}
	now := c.now()
	if now.Before(entry.nextPoll) {
		return entry.data
	}
	if deviceType == "" {
		deviceType = entry.deviceType
	}
	buf, status := c.read(path, deviceType)
	entry.nextPoll = now.Add(smartPollInterval)
	switch {
	case status == 3:
		// smartctl's dedicated standby exit code must not be confused with an I/O error.
		entry.data.ReadState = "sleeping"
	case status < 0 || status&7 != 0:
		entry.data = model.SmartctlA{ReadState: "unavailable"}
	default:
		var sample model.SmartctlA
		if err := json.Unmarshal(buf, &sample); err != nil || sample.ModelName == "" {
			entry.data = model.SmartctlA{ReadState: "unavailable"}
		} else {
			sample.ReadState = "fresh"
			sample.SampledAt = now.Unix()
			entry.data = sample
			entry.deviceType = sample.Device.Type
		}
	}
	c.entries[path] = entry
	return entry.data
}
