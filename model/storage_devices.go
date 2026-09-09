package model

import (
	"strconv"
	"strings"
)

// StorageDevices builds a mounted-volume view without changing the lsblk graph
// used by physical disk management. Shared RAID nodes may occur under every member.
func StorageDevices(disks []LSBLKModel, systemPath string) []LSBLKModel {
	view := storageDeviceView{
		systemPath: systemPath,
		devices:    make([]LSBLKModel, 0),
		groups:     make(map[string]int),
		seen:       make(map[[2]string]bool),
	}
	for i, disk := range disks {
		owner := disk
		if containsSystemStorage(disk, systemPath) {
			owner.Model = "System"
		}
		key := disk.Path
		if key == "" {
			key = strconv.Itoa(i)
		}
		view.visit(disk, owner, key)
	}
	return view.devices
}

type storageDeviceView struct {
	systemPath string
	devices    []LSBLKModel
	groups     map[string]int
	seen       map[[2]string]bool
}

func (v *storageDeviceView) visit(node, owner LSBLKModel, groupKey string) {
	if strings.HasPrefix(node.Type, "raid") && node.Path != "" {
		owner = node
		owner.Model = strings.ToUpper(node.Type)
		owner.Tran = node.Type
		groupKey = node.Path
		if containsSystemStorage(node, v.systemPath) {
			owner.Model = "System"
		}
	}
	v.addMountedVolume(node, owner, groupKey)
	for _, child := range node.Children {
		v.visit(child, owner, groupKey)
	}
}

func (v *storageDeviceView) addMountedVolume(node, owner LSBLKModel, groupKey string) {
	if node.MountPoint == "" || node.MountPoint == "[SWAP]" {
		return
	}
	key := [2]string{node.Path, node.MountPoint}
	if node.Path != "" && v.seen[key] {
		return
	}
	v.seen[key] = true
	index, ok := v.groups[groupKey]
	if !ok {
		index = len(v.devices)
		v.groups[groupKey] = index
		owner.Children = nil
		owner.MountPoint = ""
		v.devices = append(v.devices, owner)
	}
	volume := node
	volume.Children = nil
	v.devices[index].Children = append(v.devices[index].Children, volume)
}

func containsSystemStorage(node LSBLKModel, systemPath string) bool {
	if node.MountPoint == "/" || (systemPath != "" && node.Path == systemPath) {
		return true
	}
	for _, child := range node.Children {
		if containsSystemStorage(child, systemPath) {
			return true
		}
	}
	return false
}

// StorageUsage counts a filesystem once even when it has several mount points.
// Health is populated separately from the physical drives' SMART reports.
func StorageUsage(disks []LSBLKModel) DiskStatus {
	status := DiskStatus{}
	seen := make(map[string]bool)
	for _, disk := range StorageDevices(disks, "") {
		for _, volume := range disk.Children {
			if volume.Path != "" && seen[volume.Path] {
				continue
			}
			seen[volume.Path] = true
			size, _ := strconv.ParseUint(volume.FSSize.String(), 10, 64)
			avail, _ := strconv.ParseUint(volume.FSAvail.String(), 10, 64)
			used, _ := strconv.ParseUint(volume.FSUsed.String(), 10, 64)
			status.Size += size
			status.Avail += avail
			status.Used += used
		}
	}
	return status
}
