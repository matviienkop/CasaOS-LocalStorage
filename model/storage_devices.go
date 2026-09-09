package model

import (
	"strconv"
	"strings"
)

// StorageDevices builds a mounted-volume view without changing the lsblk graph
// used by physical disk management. Shared RAID nodes may occur under every member.
func StorageDevices(disks []LSBLKModel, systemPath string) []LSBLKModel {
	result := make([]LSBLKModel, 0)
	groups := make(map[string]int)
	seen := make(map[[2]string]bool)
	var visit func(LSBLKModel, LSBLKModel, string)
	visit = func(node, owner LSBLKModel, groupKey string) {
		if strings.HasPrefix(node.Type, "raid") && node.Path != "" {
			owner = node
			owner.Model = strings.ToUpper(node.Type)
			owner.Tran = node.Type
			groupKey = node.Path
			if containsSystemStorage(node, systemPath) {
				owner.Model = "System"
			}
		}
		if node.MountPoint != "" && node.MountPoint != "[SWAP]" {
			key := [2]string{node.Path, node.MountPoint}
			if node.Path == "" || !seen[key] {
				seen[key] = true
				index, ok := groups[groupKey]
				if !ok {
					index = len(result)
					groups[groupKey] = index
					owner.Children = nil
					owner.MountPoint = ""
					result = append(result, owner)
				}
				volume := node
				volume.Children = nil
				result[index].Children = append(result[index].Children, volume)
			}
		}
		for _, child := range node.Children {
			visit(child, owner, groupKey)
		}
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
		visit(disk, owner, key)
	}
	return result
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
