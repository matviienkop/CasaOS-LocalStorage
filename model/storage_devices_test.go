package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

func testVolume(path, mount string) LSBLKModel {
	return LSBLKModel{Path: path, Name: path, MountPoint: mount, FsType: "ext4",
		UUID: "same-label-is-not-identity", Label: "storage", FSSize: "1000", FSAvail: "800", FSUsed: "200"}
}

func testRAID(mount string, partitionedMembers bool) []LSBLKModel {
	raid := testVolume("/dev/md0", mount)
	raid.Type, raid.Size = "raid10", 2000
	disks := []LSBLKModel{}
	for _, path := range []string{"/dev/sda", "/dev/sdb", "/dev/sdc", "/dev/sdd"} {
		member := LSBLKModel{Path: path, Type: "disk", Tran: "sata", Model: "HDD", Size: 1000, FsType: "linux_raid_member", Children: []LSBLKModel{raid}}
		if partitionedMembers {
			part := member
			part.Path, part.Type = path+"1", "part"
			member.FsType, member.Children = "", []LSBLKModel{part}
		}
		disks = append(disks, member)
	}
	return disks
}

func TestStorageDevicesRAID(t *testing.T) {
	cases := []struct {
		name        string
		mount       string
		partitioned bool
		model       string
	}{
		{"whole-drive data", "/mnt/storage", false, "RAID10"},
		{"whole-drive root", "/", false, "System"},
		{"partitioned data", "/mnt/storage", true, "RAID10"},
		{"partitioned root", "/", true, "System"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkRAIDStorage(t, tc.mount, tc.partitioned, tc.model)
		})
	}
}

func checkRAIDStorage(t *testing.T, mount string, partitioned bool, name string) {
	t.Helper()
	disks := testRAID(mount, partitioned)
	before, _ := json.Marshal(disks)
	got := StorageDevices(disks, "")
	if len(got) != 1 || len(got[0].Children) != 1 {
		t.Fatalf("duplicate/missing volumes: %#v", got)
	}
	if got[0].Model != name || got[0].Path != "/dev/md0" || got[0].Size != 2000 || got[0].Tran != "raid10" {
		t.Fatalf("wrong logical group: %#v", got[0])
	}
	if got[0].Children[0].MountPoint != mount {
		t.Fatal("mount point changed")
	}
	if !reflect.DeepEqual(StorageDevices(got, ""), got) {
		t.Fatal("not idempotent")
	}
	got[0].Children[0].Label = "changed"
	after, _ := json.Marshal(disks)
	if string(before) != string(after) {
		t.Fatal("input tree changed")
	}
}

func TestStorageDevicesSystemMembersAndDF(t *testing.T) {
	disks := testRAID("/", false)
	boot := testVolume("/dev/sda1", "/boot/efi")
	disks[0].Children = append(disks[0].Children, boot)
	got := StorageDevices(disks, "")
	if len(got) != 2 || got[0].Model != "System" || got[1].Model != "System" {
		t.Fatalf("root array or member boot partition lost protection: %#v", got)
	}
	disks = testRAID("/alternate-root", true)
	got = StorageDevices(disks, "/dev/md0")
	if got[0].Model != "System" {
		t.Fatal("df system identity ignored")
	}
}

func TestStorageDevicesPartitionsOnRAID(t *testing.T) {
	data := testVolume("/dev/md0p1", "/mnt/first")
	other := testVolume("/dev/md0p2", "/mnt/second")
	raid := LSBLKModel{Path: "/dev/md0", Type: "raid1", Size: 2500, Children: []LSBLKModel{data, other}}
	disks := []LSBLKModel{
		{Path: "/dev/sda", Children: []LSBLKModel{raid}},
		{Path: "/dev/sdb", Children: []LSBLKModel{raid}},
	}
	got := StorageDevices(disks, "")
	if len(got) != 1 || len(got[0].Children) != 2 || got[0].Size != 2500 {
		t.Fatalf("partitioned RAID not represented once: %#v", got)
	}
	if usage := StorageUsage(disks); usage.Size != 2000 || usage.Avail != 1600 || usage.Used != 400 {
		t.Fatalf("wrong partitioned RAID totals: %#v", usage)
	}
}

func TestStorageDevicesOrdinaryAndMissingMounts(t *testing.T) {
	root := testVolume("/dev/mmcblk0p2", "/")
	boot := testVolume("/dev/mmcblk0p1", "/boot/firmware")
	usb := testVolume("/dev/sde", "/media/usb")
	usb.Tran = "usb"
	other := testVolume("/dev/sdf1", "/mnt/other")
	disks := []LSBLKModel{
		{Path: "/dev/mmcblk0", Children: []LSBLKModel{boot, root}}, usb,
		{Path: "/dev/sdf", Children: []LSBLKModel{other}},
		{Path: "/dev/sdg", FsType: "linux_raid_member"},
		{Path: "/dev/zram0", MountPoint: "[SWAP]", FsType: "swap"},
	}
	got := StorageDevices(disks, "")
	if len(got) != 3 || len(got[0].Children) != 2 || got[0].Model != "System" || got[1].Tran != "usb" {
		t.Fatalf("ordinary volumes changed or unmounted volume exposed: %#v", got)
	}
	if got[1].Children[0].Path == got[2].Children[0].Path {
		t.Fatal("same UUID/label disks merged")
	}
	if len(StorageDevices(nil, "")) != 0 {
		t.Fatal("empty input")
	}
}

func TestStorageUsageMultipleMountsAndArrays(t *testing.T) {
	disks := testRAID("/mnt/storage", true)
	bind := testVolume("/dev/md0", "/mnt/bind")
	second := testVolume("/dev/md1", "/mnt/second")
	second.Type, second.Size = "raid1", 1100
	disks[0].Children = append(disks[0].Children, bind, second)
	disks[1].Children = append(disks[1].Children, second)
	got := StorageDevices(disks, "")
	count := 0
	for _, d := range got {
		count += len(d.Children)
	}
	if count != 3 {
		t.Fatalf("mounts lost or duplicated: %d", count)
	}
	if usage := StorageUsage(disks); usage.Size != 2000 || usage.Avail != 1600 || usage.Used != 400 {
		t.Fatalf("multiple mounts counted more than once: %#v", usage)
	}
}
