/*@Author: LinkLeong link@icewhale.com
 *@Date: 2022-07-11 16:02:29
 *@LastEditors: LinkLeong
 *@LastEditTime: 2022-08-17 19:14:50
 *@FilePath: /CasaOS/route/v1/storage.go
 *@Description:
 *@Website: https://www.casaos.io
 *Copyright (c) 2022 by icewhale, All Rights Reserved.
 */
package v1

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/model"
	"github.com/IceWhaleTech/CasaOS-Common/utils/common_err"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	model1 "github.com/IceWhaleTech/CasaOS-LocalStorage/model"
	model2 "github.com/IceWhaleTech/CasaOS-LocalStorage/service/model"

	"github.com/IceWhaleTech/CasaOS-LocalStorage/service"
)

func GetStorageList(ctx echo.Context) error {
	showSystem := ctx.QueryParam("system") != ""
	blkList := service.MyService.Disk().LSBLK(false)
	df, err := service.MyService.Disk().GetSystemDf()
	systemPath := ""
	if err == nil {
		systemPath = df.FileSystem
	}

	storages := []model1.Storages{}
	for _, disk := range model1.StorageDevices(blkList, systemPath) {
		isSystem := disk.Model == "System"
		if isSystem && !showSystem {
			continue
		}
		storage := model1.Storages{
			DiskName: disk.Model,
			Path:     disk.Path,
			Size:     disk.Size,
			Type:     disk.Tran,
			Children: []model1.Storage{},
		}
		for _, volume := range disk.Children {
			if isSystem && (volume.MountPoint == "/boot/efi" || volume.FsType == "swap") {
				continue
			}
			label := volume.Label
			if label == "" {
				label = filepath.Base(volume.MountPoint)
				if volume.MountPoint == "/" {
					label = "System"
				}
			}
			storage.Children = append(storage.Children, model1.Storage{
				UUID:        volume.UUID,
				MountPoint:  volume.MountPoint,
				Size:        volume.FSSize.String(),
				Avail:       volume.FSAvail.String(),
				Used:        volume.FSUsed.String(),
				Path:        volume.Path,
				Type:        volume.FsType,
				DriveName:   volume.Name,
				Label:       label,
				PersistedIn: service.MyService.Disk().GetPersistentTypeByUUID(volume.UUID),
			})
		}
		if len(storage.Children) > 0 {
			storages = append(storages, storage)
		}
	}
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: storages})
}

func PostAddStorage(ctx echo.Context) error {
	js := make(map[string]interface{})
	if err := ctx.Bind(&js); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS), Data: err.Error()})
	}

	path := js["path"].(string)
	name := js["name"].(string)
	format := js["format"].(bool)

	if len(path) == 0 {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}
	if _, ok := diskMap[path]; ok {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.DISK_BUSYING, Message: common_err.GetMsg(common_err.DISK_BUSYING)})
	}

	diskMap[path] = "busying"

	defer service.MyService.Disk().RemoveLSBLKCache()
	defer delete(diskMap, path)
	currentDisk := service.MyService.Disk().GetDiskInfo(path)
	if format {

		if err := service.MyService.Disk().UmountPointAndRemoveDir(currentDisk); err != nil {
			logger.Error("error when trying to umount storage", zap.Error(err), zap.String("path", path))
			return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.REMOVE_MOUNT_POINT_ERROR, Message: err.Error()})
		}

		logger.Info("deleting storage...", zap.String("path", path))
		if err := service.MyService.Disk().DeletePartition(path); err != nil {
			logger.Error("error when trying to delete partition", zap.Error(err), zap.String("path", path))
			return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
		}

		logger.Info("formatting storage...", zap.String("path", path))
		if err := service.MyService.Disk().AddPartition(path); err != nil {
			return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
		}
	}
	currentDisk = service.MyService.Disk().GetDiskInfo(path)
	if len(currentDisk.Children) == 0 && service.IsDiskSupported(currentDisk) {
		currentDisk.Children = append(currentDisk.Children, currentDisk)
		// mountPoint := currentDisk.GetMountPoint(name)

		// // mount disk
		// if output, err := service.MyService.Disk().MountDisk(currentDisk.Path, mountPoint); err != nil {
		// 	logger.Error("err", zap.Error(err), zap.String("output", mountPoint))
		// 	return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: output, Data: err.Error()})
		// 	return
		// }

		// var b model1.LSBLKModel
		// retry := 3 // ugly workaround for lsblk not returning UUID after creating partition on time - need a better solution
		// for b.UUID == "" && retry > 0 {
		// 	time.Sleep(1 * time.Second)
		// 	b = service.MyService.Disk().GetDiskInfo(currentDisk.Path)
		// 	retry--
		// }

		// m := model2.Volume{
		// 	MountPoint: b.MountPoint,
		// 	UUID:       b.UUID,
		// 	CreatedAt:  time.Now().Unix(),
		// }

		// if err := service.MyService.Disk().SaveMountPointToDB(m); err != nil {
		// 	return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
		// 	return
		// }

		// // send notify to client
		// go func(blkChild model1.LSBLKModel) {
		// 	message := map[string]interface{}{
		// 		"data": StorageMessage{
		// 			Action: "ADDED",
		// 			Path:   blkChild.Path,
		// 			Volume: "/mnt/",
		// 			Size:   blkChild.Size,
		// 			Type:   blkChild.Tran,
		// 		},
		// 	}

		// 	if err := service.MyService.Notify().SendNotify(messagePathStorageStatus, message); err != nil {
		// 		logger.Error("error when sending notification", zap.Error(err), zap.String("message path", messagePathStorageStatus), zap.Any("message", message))
		// 	}
		// }(currentDisk)
	}
	message := ""
	for _, blkChild := range currentDisk.Children {

		mountPoint := blkChild.GetMountPoint(name)
		// mount disk
		if output, err := service.MyService.Disk().MountDisk(blkChild.Path, mountPoint); err != nil {
			logger.Error("err", zap.Error(err), zap.String("mountPoint", mountPoint), zap.String("output", output))
			message += blkChild.Path + "\n"
			continue
			// return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: output, Data: err.Error()})
			// return
		}

		var b model1.LSBLKModel
		retry := 3 // ugly workaround for lsblk not returning UUID after creating partition on time - need a better solution
		for b.UUID == "" && retry > 0 {
			time.Sleep(1 * time.Second)
			b = service.MyService.Disk().GetDiskInfo(blkChild.Path)
			retry--
		}

		m := model2.Volume{
			MountPoint: b.MountPoint,
			UUID:       b.UUID,
			CreatedAt:  time.Now().Unix(),
		}

		if err := service.MyService.Disk().SaveMountPointToDB(m); err != nil {
			blkChild.MountPoint = mountPoint
			service.MyService.Disk().UmountPointAndRemoveDir(blkChild)
			message += blkChild.Path + "\n"
			continue
			// return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
			// return
		}

		// send notify to client
		go func(blkChild model1.LSBLKModel) {
			message := map[string]interface{}{
				"data": StorageMessage{
					Action: "ADDED",
					Path:   blkChild.Path,
					Volume: "/media/",
					Size:   blkChild.Size,
					Type:   blkChild.Tran,
				},
			}

			if err := service.MyService.Notify().SendNotify(messagePathStorageStatus, message); err != nil {
				logger.Error("error when sending notification", zap.Error(err), zap.String("message path", messagePathStorageStatus), zap.Any("message", message))
			}
		}(blkChild)
	}
	if len(message) > 0 {
		return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SERVICE_ERROR, Message: message})
	}
	return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

// @Param  pwd formData string true "user password"
// @Param  volume formData string true "mount point"
// @Success 200 {string} string "ok"
// @Router /disk/format [post]
func PutFormatStorage(ctx echo.Context) error {
	js := make(map[string]string)
	if err := ctx.Bind(&js); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS), Data: err.Error()})
	}

	// requires password from user to confirm the action
	// if claims, err := jwt.ParseToken(c.GetHeader("Authorization"), false); err != nil || encryption.GetMD5ByStr(js["password"]) != claims.Password {
	// 	return ctx.JSON(http.StatusUnauthorized, model.Result{Success: common_err.PWD_INVALID, Message: common_err.GetMsg(common_err.PWD_INVALID)})
	// 	return
	// }

	path := js["path"]
	mountPoint := js["volume"]

	if len(path) == 0 {
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}

	if _, ok := diskMap[path]; ok {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.DISK_BUSYING, Message: common_err.GetMsg(common_err.DISK_BUSYING)})
	}

	diskMap[path] = "busying"

	defer service.MyService.Disk().RemoveLSBLKCache()
	defer delete(diskMap, path)
	diskInfo := service.MyService.Disk().GetDiskInfo(path)
	if err := service.MyService.Disk().UmountPointAndRemoveDir(diskInfo); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.REMOVE_MOUNT_POINT_ERROR, Message: err.Error()})
	}

	if err := service.MyService.Disk().FormatDisk(path); err != nil {
		delete(diskMap, path)
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.FORMAT_ERROR, Message: common_err.GetMsg(common_err.FORMAT_ERROR)})
	}
	currentDisk := service.MyService.Disk().GetDiskInfo(path)
	for diskInfo.UUID == currentDisk.UUID {
		time.Sleep(1 * time.Second)
		currentDisk = service.MyService.Disk().GetDiskInfo(path)
	}
	if mountPoint == "" {
		mountPoint = currentDisk.GetMountPoint("")
	}

	if output, err := service.MyService.Disk().MountDisk(path, mountPoint); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: output})
	}

	m := model2.Volume{
		MountPoint: mountPoint,
		UUID:       currentDisk.UUID,
		CreatedAt:  time.Now().Unix(),
	}

	if err := service.MyService.Disk().SaveMountPointToDB(m); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

func DeleteStorage(ctx echo.Context) error {
	js := make(map[string]string)
	if err := ctx.Bind(&js); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS), Data: err.Error()})
	}

	// requires password from user to confirm the action
	// if claims, err := jwt.ParseToken(c.GetHeader("Authorization"), false); err != nil || encryption.GetMD5ByStr(js["password"]) != claims.Password {
	// 	return ctx.JSON(http.StatusUnauthorized, model.Result{Success: common_err.PWD_INVALID, Message: common_err.GetMsg(common_err.PWD_INVALID)})
	// 	return
	// }

	path := js["path"]
	mountPoint := js["volume"]

	if len(path) == 0 || len(mountPoint) == 0 {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.INVALID_PARAMS, Message: common_err.GetMsg(common_err.INVALID_PARAMS)})
	}

	if _, ok := diskMap[path]; ok {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.DISK_BUSYING, Message: common_err.GetMsg(common_err.DISK_BUSYING)})
	}
	diskInfo := service.MyService.Disk().GetDiskInfo(path)
	if err := service.MyService.Disk().UmountPointAndRemoveDir(diskInfo); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.Result{Success: common_err.REMOVE_MOUNT_POINT_ERROR, Message: err.Error()})
	}

	// delete data
	defer func() {
		if err := service.MyService.Disk().DeleteMountPointFromDB(path, mountPoint); err != nil {
			logger.Error("error when deleting mount point from database", zap.Error(err))
		}
	}()
	defer service.MyService.Disk().RemoveLSBLKCache()

	// send notify to client
	go func() {
		message := map[string]interface{}{
			"data": StorageMessage{
				Action: "REMOVED",
				Path:   path,
				Volume: mountPoint,
				Size:   0,
				Type:   "",
			},
		}

		if err := service.MyService.Notify().SendNotify(messagePathStorageStatus, message); err != nil {
			logger.Error("error when sending notification", zap.Error(err), zap.String("message path", messagePathStorageStatus), zap.Any("message", message))
		}
	}()

	return ctx.JSON(http.StatusOK, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}
