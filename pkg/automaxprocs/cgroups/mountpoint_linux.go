//go:build linux

package cgroups

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chaosblade-io/chaosblade-spec-go/log"
)

const (
	_mountInfoSep               = " "
	_mountInfoOptsSep           = ","
	_mountInfoOptionalFieldsSep = "-"
)

const (
	_miFieldIDMountID = iota
	_miFieldIDParentID
	_miFieldIDDeviceID
	_miFieldIDRoot
	_miFieldIDMountPoint
	_miFieldIDOptions
	_miFieldIDOptionalFields

	_miFieldCountFirstHalf
)

const (
	_miFieldOffsetFSType = iota
	_miFieldOffsetMountSource
	_miFieldOffsetSuperOptions

	_miFieldCountSecondHalf
)

const _miFieldCountMin = _miFieldCountFirstHalf + _miFieldCountSecondHalf

// MountPoint is the data structure for the mount points in
// `/proc/$PID/mountinfo`. See also proc(5) for more information.
type MountPoint struct {
	MountID        int
	ParentID       int
	DeviceID       string
	Root           string
	MountPoint     string
	Options        []string
	OptionalFields []string
	FSType         string
	MountSource    string
	SuperOptions   []string
}

// NewMountPointFromLine parses a line read from `/proc/$PID/mountinfo` and
// returns a new *MountPoint.
func NewMountPointFromLine(line, actualCGRoot string) (*MountPoint, error) {
	fields := strings.Split(line, _mountInfoSep)

	if len(fields) < _miFieldCountMin {
		return nil, mountPointFormatInvalidError{line}
	}

	mountID, err := strconv.Atoi(fields[_miFieldIDMountID])
	if err != nil {
		return nil, err
	}

	parentID, err := strconv.Atoi(fields[_miFieldIDParentID])
	if err != nil {
		return nil, err
	}

	for i, field := range fields[_miFieldIDOptionalFields:] {
		if field == _mountInfoOptionalFieldsSep {
			// End of optional fields.
			fsTypeStart := _miFieldIDOptionalFields + i + 1

			// Now we know where the optional fields end, split the line again with a
			// limit to avoid issues with spaces in super options as present on WSL.
			fields = strings.SplitN(line, _mountInfoSep, fsTypeStart+_miFieldCountSecondHalf)
			if len(fields) != fsTypeStart+_miFieldCountSecondHalf {
				return nil, mountPointFormatInvalidError{line}
			}

			miFieldIDFSType := _miFieldOffsetFSType + fsTypeStart
			miFieldIDMountSource := _miFieldOffsetMountSource + fsTypeStart
			miFieldIDSuperOptions := _miFieldOffsetSuperOptions + fsTypeStart

			mp := &MountPoint{
				MountID:        mountID,
				ParentID:       parentID,
				DeviceID:       fields[_miFieldIDDeviceID],
				Root:           fields[_miFieldIDRoot],
				MountPoint:     replaceCgroupFsPathForDaemonSetPod(fields[_miFieldIDMountPoint], actualCGRoot),
				Options:        strings.Split(fields[_miFieldIDOptions], _mountInfoOptsSep),
				OptionalFields: fields[_miFieldIDOptionalFields:(fsTypeStart - 1)],
				FSType:         fields[miFieldIDFSType],
				MountSource:    fields[miFieldIDMountSource],
				SuperOptions:   strings.Split(fields[miFieldIDSuperOptions], _mountInfoOptsSep),
			}
			log.Debugf(context.Background(), "NewMountPointFromLine, mp: %+v", mp)
			return mp, nil
		}
	}

	return nil, mountPointFormatInvalidError{line}
}

// CustomTranslate converts an absolute path inside the *MountPoint's file system
// to the host file system path in the mount namespace the *MountPoint belongs to.
// It directly joins the MountPoint with the absPath.
//
// If the absPath is not within the MountPoint's root, it returns an error.
func (mp *MountPoint) CustomTranslate(absPath string) (string, error) {
	relPath, err := filepath.Rel(mp.Root, absPath)
	if err != nil {
		return "", err
	}
	if relPath == ".." || strings.HasPrefix(relPath, "../") {
		return "", pathNotExposedFromMountPointError{
			mountPoint: mp.MountPoint,
			root:       mp.Root,
			path:       absPath,
		}
	}

	return filepath.Join(mp.MountPoint, absPath), nil
}

// parseMountInfo parses procPathMountInfo (usually at `/proc/$PID/mountinfo`)
// and yields parsed *MountPoint into newMountPoint.
func parseMountInfo(procPathMountInfo, actualCGRoot string, newMountPoint func(*MountPoint) error) error {
	mountInfoFile, err := os.Open(procPathMountInfo)
	if err != nil {
		return err
	}
	defer mountInfoFile.Close()
	fileInfo, err := os.Stat(procPathMountInfo)
	if err != nil {
		log.Errorf(context.Background(), "stat file failed, err: %v, path: %v", err, procPathMountInfo)
		return err
	}
	log.Infof(context.Background(), "parseMountInfo, path: %v, fileInfo: %+v", procPathMountInfo, fileInfo)
	scanner := bufio.NewScanner(mountInfoFile)

	for scanner.Scan() {
		mountPoint, err := NewMountPointFromLine(scanner.Text(), actualCGRoot)
		if err != nil {
			return err
		}
		if err := newMountPoint(mountPoint); err != nil {
			return err
		}
	}

	return scanner.Err()
}
