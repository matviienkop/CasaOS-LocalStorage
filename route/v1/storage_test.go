package v1

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/IceWhaleTech/CasaOS-LocalStorage/model"
	"github.com/IceWhaleTech/CasaOS-LocalStorage/service"
	dbmodel "github.com/IceWhaleTech/CasaOS-LocalStorage/service/model"
	"github.com/labstack/echo/v4"
)

type storageTestDisk struct {
	service.DiskService
	disks      []model.LSBLKModel
	systemPath string
	dfError    error
	smartPaths []string
}

func (d *storageTestDisk) LSBLK(bool) []model.LSBLKModel { return d.disks }
func (d *storageTestDisk) GetSystemDf() (model.DFDiskSpace, error) {
	return model.DFDiskSpace{FileSystem: d.systemPath}, d.dfError
}
func (d *storageTestDisk) GetPersistentTypeByUUID(string) string         { return "fstab" }
func (d *storageTestDisk) GetSerialAllFromDB() ([]dbmodel.Volume, error) { return nil, nil }
func (d *storageTestDisk) SmartCTL(path string) model.SmartctlA {
	d.smartPaths = append(d.smartPaths, path)
	return model.SmartctlA{}
}

type storageTestServices struct {
	service.Services
	disk *storageTestDisk
}

func (s storageTestServices) Disk() service.DiskService { return s.disk }

func useStorageTestService(t *testing.T, disk *storageTestDisk) {
	t.Helper()
	previous := service.MyService
	service.MyService = storageTestServices{disk: disk}
	t.Cleanup(func() { service.MyService = previous })
}

func storageTestRequest(t *testing.T, handler echo.HandlerFunc, path string, result interface{}) {
	t.Helper()
	w := httptest.NewRecorder()
	c := echo.New().NewContext(httptest.NewRequest("GET", path, nil), w)
	if err := handler(c); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 {
		t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), result); err != nil {
		t.Fatal(err)
	}
}

func storageTestRAID(mount string, partitioned bool) []model.LSBLKModel {
	raid := model.LSBLKModel{Path: "/dev/md0", Name: "md0", Type: "raid10", Size: 2000, FsType: "ext4", MountPoint: mount, UUID: "raid-volume", Label: "storage", FSSize: "1900", FSAvail: "1800", FSUsed: "100"}
	disks := []model.LSBLKModel{}
	for _, name := range []string{"sda", "sdb", "sdc", "sdd"} {
		member := model.LSBLKModel{Path: "/dev/" + name, Name: name, Type: "disk", Tran: "sata", Size: 1000, FsType: "linux_raid_member", Children: []model.LSBLKModel{raid}}
		if partitioned {
			part := member
			part.Path, part.Type = member.Path+"1", "part"
			member.FsType, member.Children = "", []model.LSBLKModel{part}
		}
		disks = append(disks, member)
	}
	return disks
}

func TestGetStorageListSharedRAID(t *testing.T) {
	for _, partitioned := range []bool{false, true} {
		disk := &storageTestDisk{disks: storageTestRAID("/mnt/storage", partitioned), dfError: errors.New("df unavailable")}
		useStorageTestService(t, disk)
		before, _ := json.Marshal(disk.disks)
		var result struct {
			Data []model.Storages `json:"data"`
		}
		storageTestRequest(t, GetStorageList, "/v1/storage", &result)
		if len(result.Data) != 1 || len(result.Data[0].Children) != 1 {
			t.Fatalf("duplicate array: %#v", result)
		}
		group := result.Data[0]
		if group.Path != "/dev/md0" || group.Size != 2000 || group.DiskName != "RAID10" {
			t.Fatalf("wrong parent: %#v", group)
		}
		volume := group.Children[0]
		if volume.Path != "/dev/md0" || volume.Size != "1900" || volume.PersistedIn != "fstab" || volume.Label != "storage" {
			t.Fatalf("lost volume metadata: %#v", volume)
		}
		after, _ := json.Marshal(disk.disks)
		if string(before) != string(after) {
			t.Fatal("physical tree changed")
		}
	}
}

func TestGetStorageListSystemRAID(t *testing.T) {
	for _, partitioned := range []bool{false, true} {
		for _, dfFailure := range []bool{false, true} {
			disk := &storageTestDisk{disks: storageTestRAID("/", partitioned), systemPath: "/dev/md0"}
			if dfFailure {
				disk.dfError = errors.New("df unavailable")
			}
			useStorageTestService(t, disk)
			var hidden struct {
				Data []model.Storages `json:"data"`
			}
			storageTestRequest(t, GetStorageList, "/v1/storage", &hidden)
			if len(hidden.Data) != 0 {
				t.Fatal("system RAID exposed as user storage")
			}
			var shown struct {
				Data []model.Storages `json:"data"`
			}
			storageTestRequest(t, GetStorageList, "/v1/storage?system=show", &shown)
			if len(shown.Data) != 1 || shown.Data[0].DiskName != "System" {
				t.Fatalf("system protection lost: %#v", shown)
			}
		}
	}
}

func TestGetStorageListSystemAndUSB(t *testing.T) {
	disk := &storageTestDisk{disks: []model.LSBLKModel{
		{Path: "/dev/mmcblk0", Children: []model.LSBLKModel{
			{Path: "/dev/mmcblk0p1", MountPoint: "/boot/firmware", FsType: "vfat"},
			{Path: "/dev/mmcblk0p2", MountPoint: "/", FsType: "ext4"},
			{Path: "/dev/mmcblk0p3", MountPoint: "/boot/efi", FsType: "vfat"},
		}},
		{Path: "/dev/sde", Tran: "usb", MountPoint: "/media/usb", FsType: "exfat", UUID: "usb"},
	}, dfError: errors.New("df unavailable")}
	useStorageTestService(t, disk)
	var result struct {
		Data []model.Storages `json:"data"`
	}
	storageTestRequest(t, GetStorageList, "/v1/storage?system=show", &result)
	if len(result.Data) != 2 || len(result.Data[0].Children) != 2 || result.Data[0].DiskName != "System" || result.Data[1].Type != "usb" {
		t.Fatalf("system/USB regression: %#v", result)
	}
	if result.Data[0].Children[1].Label != "System" || result.Data[1].Children[0].Label != "usb" {
		t.Fatal("label fallback changed")
	}
}

func TestGetDiskListKeepsRAIDMembers(t *testing.T) {
	for _, partitioned := range []bool{false, true} {
		disk := &storageTestDisk{disks: storageTestRAID("/mnt/storage", partitioned)}
		useStorageTestService(t, disk)
		var result struct {
			Data struct {
				Disks []model.Drive `json:"disks"`
				Avail []model.Drive `json:"avail"`
			} `json:"data"`
		}
		storageTestRequest(t, GetDiskList, "/v1/disks", &result)
		if len(result.Data.Disks) != 4 || len(result.Data.Avail) != 0 || len(disk.smartPaths) != 4 {
			t.Fatalf("lost or free RAID members: %#v, SMART=%v", result, disk.smartPaths)
		}
		for i, d := range result.Data.Disks {
			if d.Path != disk.disks[i].Path || d.Size != 1000 {
				t.Fatal("physical identity changed")
			}
		}
	}
}

func TestDiskInUseUnassembledRAID(t *testing.T) {
	member := model.LSBLKModel{FsType: "linux_raid_member"}
	for _, disk := range []model.LSBLKModel{member, {Children: []model.LSBLKModel{member}}, {Children: []model.LSBLKModel{{Children: []model.LSBLKModel{{MountPoint: "/mnt/nested"}}}}}} {
		if !diskInUse(disk) {
			t.Fatal("member or nested mount offered as free")
		}
	}
	if diskInUse(model.LSBLKModel{Path: "/dev/empty"}) {
		t.Fatal("empty disk hidden")
	}
}
