# CasaOS-LocalStorage

[![Go Reference](https://pkg.go.dev/badge/github.com/IceWhaleTech/CasaOS-LocalStorage.svg)](https://pkg.go.dev/github.com/IceWhaleTech/CasaOS-LocalStorage) [![Go Report Card](https://goreportcard.com/badge/github.com/IceWhaleTech/CasaOS-LocalStorage)](https://goreportcard.com/report/github.com/IceWhaleTech/CasaOS-LocalStorage) [![goreleaser](https://github.com/IceWhaleTech/CasaOS-LocalStorage/actions/workflows/release.yml/badge.svg)](https://github.com/IceWhaleTech/CasaOS-LocalStorage/actions/workflows/release.yml) [![codecov](https://codecov.io/gh/IceWhaleTech/CasaOS-LocalStorage/branch/main/graph/badge.svg?token=GKFBMQ3157)](https://codecov.io/gh/IceWhaleTech/CasaOS-LocalStorage)

Local Storage service provides local storage and disk management functionalities to CasaOS.




## publish api to npm

### edit version in package.json

### run
```bash
yarn

yarn start
```

### publish

Manual publish
```bash
yarn publish
```

Auto publish
```bash 
git push origin dev**
```

## SMART polling

SMART data is refreshed on demand at most once every 15 minutes per device.
Background storage notifications and disk API requests share the same cache.
`smartctl -n standby,3,5` skips sleeping ATA disks and devices whose ATA power
state cannot be determined. Native ATA devices are identified through sysfs
and queried with `-d ata` to avoid device autodetection waking them. Other
transports retain their previously detected device type after a successful read;
their initial autodetection may depend on controller behavior.

A confirmed sleep response keeps the last successful sample and defers the next
attempt by 15 minutes. Errors and device removal do not reuse old readings.
Replacing a device node invalidates the previous sample. The cache is in memory:
a disk already sleeping at service startup has no historical sample to display.

The legacy `/v1/disks` response additionally exposes `smart_state` (`fresh`,
`sleeping`, or `unavailable`) and `smart_sampled_at` (Unix seconds of the last
successful sample, omitted when no sample exists). The original `temperature`
field remains available; a sleeping disk's value is the last measurement, not a
new measurement. Consumers should display its timestamp and state.
