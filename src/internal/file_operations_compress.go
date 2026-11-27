package internal

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/notify"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
)

const dirZipPerm = 0o755

func validateCompressOperation(sources []string) (int, error) {
	totalFiles := 0
	for _, src := range sources {
		stat, err := os.Stat(src)
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("source path does not exist: %s", src)
		}
		if err != nil {
			return 0, fmt.Errorf("cannot access source path %s: %w", src, err)
		}
		if !stat.IsDir() {
			if err = checkFileReadable(src); err != nil {
				slog.Error("the file is not readable", "error", err)
				return 0, fmt.Errorf("the file is not readable: %s", err.Error())
			}
		}
		count, err := countReadableFiles(src)
		if err != nil {
			slog.Error("Error while zip file count files ", "error", err)
			return 0, fmt.Errorf("error while counting files for %s: %s", src, err.Error())
		}
		totalFiles += count
	}
	return totalFiles, nil
}

// getCompressSelectedFilesCmd opens the dialog for the name of the zip file
func (m *model) getCompressSelectedFilesCmd() tea.Cmd {
	panel := m.getFocusedFilePanel()

	if panel.Empty() {
		return nil
	}
	var filesToCompress []string
	if panel.SelectedCount() == 0 {
		filesToCompress = append(filesToCompress, panel.GetFocusedItem().Location)
	} else {
		filesToCompress = panel.GetSelectedLocationsSortedAsVisible()
	}

	m.typingModal.location = panel.Location
	m.typingModal.open = true
	m.typingModal.zipSources = filesToCompress
	m.typingModal.err = ""
	m.typingModal.zipTopDir = len(filesToCompress) > 1
	m.typingModal.topDirFocused = false
	m.typingModal.textInput = common.GenerateNewFileTextInput()
	m.typingModal.textInput.Placeholder = "name of the zip file"
	name := defaultZipName(panel.Location)
	m.typingModal.textInput.SetValue(name)
	m.typingModal.nameSelectEnd = renameSelectionEnd(name, false)
	m.typingModal.nameSelected = true
	m.typingModal.textInput.SetCursor(m.typingModal.nameSelectEnd)
	m.firstTextInput = true
	return nil
}

// defaultZipName is the name of the directory with the .zip suffix
func defaultZipName(dir string) string {
	name := filepath.Base(dir)
	if name == "." || name == string(filepath.Separator) {
		name = "archive"
	}
	return name + ".zip"
}

// confirmZipName checks the name typed in the typing modal. If it is not usable, the modal stays open
// with the reason shown. Otherwise the modal is closed and the compression is returned.
func (m *model) confirmZipName() tea.Cmd {
	name := strings.TrimSpace(m.typingModal.textInput.Value())
	if err := checkZipName(m.typingModal.location, name); err != nil {
		m.typingModal.err = err.Error()
		return nil
	}
	if !strings.HasSuffix(strings.ToLower(name), ".zip") {
		name += ".zip"
		if err := checkZipName(m.typingModal.location, name); err != nil {
			m.typingModal.err = err.Error()
			return nil
		}
	}
	filesToCompress := m.typingModal.zipSources
	topDir := ""
	if m.typingModal.zipTopDir {
		topDir = name[:len(name)-len(".zip")]
	}
	zipPath := filepath.Join(m.typingModal.location, name)
	m.cancelTypingModal()

	reqID := m.nextIoReqCnt()
	return func() tea.Msg {
		totalFiles, err := validateCompressOperation(filesToCompress)
		if err != nil {
			return NewNotifyModalMsg(notify.New(true, "Invalid file/dir to compress", err.Error(), notify.NoAction),
				reqID)
		}
		if err := zipSources(filesToCompress, totalFiles, zipPath, topDir, &m.processBarModel); err != nil {
			slog.Error("Error in zipping files", "error", err)
			return NewCompressOperationMsg(processbar.Failed, reqID, zipPath)
		}
		return NewCompressOperationMsg(processbar.Successful, reqID, zipPath)
	}
}

func checkZipName(dir, name string) error {
	switch {
	case name == "" || name == ".zip":
		return errors.New("name cannot be empty")
	case strings.ContainsRune(name, filepath.Separator) || name == "." || name == "..":
		return errors.New("name cannot contain a path separator")
	}
	if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
		return errors.New("already exists: " + name)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

// zipSources creates the zip file. If topDir is not empty, the sources are put in a directory of that name.
func zipSources(sources []string, totalFiles int, target, topDir string, processBar *processbar.Model) error {
	var err error

	p, err := processBar.SendAddProcessMsg(filepath.Base(target), processbar.OpCompress, totalFiles, true)
	if err != nil {
		return fmt.Errorf("cannot spawn process : %w", err)
	}
	_, err = os.Stat(target)
	if err == nil {
		p.ErrorMsg = "File already exists"
		p.State = processbar.Cancelled
		p.DoneTime = time.Now()
		pSendErr := processBar.SendUpdateProcessMsg(p, true)
		if pSendErr != nil {
			slog.Error("Error sending process update", "error", pSendErr)
		}
		return errors.New("file already exists")
	}

	f, err := os.Create(target)
	if err != nil {
		return err
	}
	defer f.Close()
	writer := zip.NewWriter(f)
	defer writer.Close()

	zipSourcesCore(sources, topDir, processBar, &p, writer)

	if p.State != processbar.Failed {
		// TODO: User p.SetSuccessful(), p.SetFailed()
		p.State = processbar.Successful
		p.Done = totalFiles
	}
	p.DoneTime = time.Now()
	pSendErr := processBar.SendUpdateProcessMsg(p, true)
	if pSendErr != nil {
		slog.Error("Error sending process update", "error", pSendErr)
	}
	return nil
}

func zipSourcesCore(sources []string, topDir string, processBar *processbar.Model,
	p *processbar.Process, writer *zip.Writer) {
	if topDir != "" {
		header := &zip.FileHeader{Name: topDir + "/", Method: zip.Store, Modified: time.Now()}
		header.SetMode(os.ModeDir | dirZipPerm)
		if _, err := writer.CreateHeader(header); err != nil {
			slog.Error("Error while creating the top directory in zip", "error", err)
			p.State = processbar.Failed
			return
		}
	}
	for _, src := range sources {
		srcParentDir := filepath.Dir(src)
		err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
			p.CurrentFile = filepath.Base(path)
			if err != nil {
				return err
			}
			relPath, err := filepath.Rel(srcParentDir, path)
			if err != nil {
				return err
			}

			if topDir != "" {
				relPath = filepath.Join(topDir, relPath)
			}
			err = writeZipFile(path, relPath, info, writer)
			if err != nil {
				return err
			}

			p.Done++
			processBar.TrySendingUpdateProcessMsg(*p)
			return nil
		})
		if err != nil {
			slog.Error("Error while zip file", "error", err)
			p.State = processbar.Failed
			break
		}
	}
}

func writeZipFile(path string, relPath string, info os.FileInfo, writer *zip.Writer) error {
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Method = zip.Deflate
	header.Name = filepath.ToSlash(relPath)
	if info.IsDir() {
		header.Name += "/"
	}
	headerWriter, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(headerWriter, file)
	if err != nil {
		return err
	}
	return nil
}
