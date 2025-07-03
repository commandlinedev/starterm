package starfileutil

import (
	"fmt"

	"github.com/commandlinedev/starterm/pkg/filestore"
	"github.com/commandlinedev/starterm/pkg/remote/fileshare/fsutil"
	"github.com/commandlinedev/starterm/pkg/util/fileutil"
	"github.com/commandlinedev/starterm/pkg/wshrpc"
)

const (
	StarFilePathPattern = "starfile://%s/%s"
)

func StarFileToFileInfo(wf *filestore.StarFile) *wshrpc.FileInfo {
	path := fmt.Sprintf(StarFilePathPattern, wf.ZoneId, wf.Name)
	rtn := &wshrpc.FileInfo{
		Path:          path,
		Dir:           fsutil.GetParentPathString(path),
		Name:          wf.Name,
		Opts:          &wf.Opts,
		Size:          wf.Size,
		Meta:          &wf.Meta,
		SupportsMkdir: false,
	}
	fileutil.AddMimeTypeToFileInfo(path, rtn)
	return rtn
}

func StarFileListToFileInfoList(wfList []*filestore.StarFile) []*wshrpc.FileInfo {
	var fileInfoList []*wshrpc.FileInfo
	for _, wf := range wfList {
		fileInfoList = append(fileInfoList, StarFileToFileInfo(wf))
	}
	return fileInfoList
}
