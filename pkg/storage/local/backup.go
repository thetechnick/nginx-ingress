package local

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

var (
	// ErrBackupFileNotExist is returned when the file the backup was saved to does not exist anymore
	ErrBackupFileNotExist = errors.New("Backup file does not exist")
	// ErrBackupOriginalFileNotExist is returned when the original file of the backup does not exist
	ErrBackupOriginalFileNotExist = errors.New("Original file does not exist")
)

// BackupFactory creates backups
type BackupFactory func(filename string) (*Backup, error)

// Backup represents the backup of a file
type Backup struct {
	original string
	backup   string
}

// CreateBackup creates a new backup
func CreateBackup(path string) (backup *Backup, err error) {
	_, filename := filepath.Split(path)
	backupFile, err := ioutil.TempFile("/tmp", filename)
	if err != nil {
		return
	}
	defer backupFile.Close()

	originalFile, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = ErrBackupOriginalFileNotExist
			return
		}
		return
	}
	defer originalFile.Close()

	if _, err = io.Copy(backupFile, originalFile); err != nil {
		return
	}
	if err = backupFile.Sync(); err != nil {
		return
	}

	backup = &Backup{
		original: path,
		backup:   backupFile.Name(),
	}

	return
}

// Delete deletes the backup file
func (b *Backup) Delete() error {
	return os.Remove(b.backup)
}

// Restore restores the original file from the backup
func (b *Backup) Restore() (err error) {
	backupFile, err := os.Open(b.backup)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrBackupFileNotExist
		}
		return
	}
	defer backupFile.Close()

	originalFile, err := os.Create(b.original)
	if err != nil {
		return
	}
	defer originalFile.Close()

	if _, err = io.Copy(originalFile, backupFile); err != nil {
		return
	}
	err = originalFile.Sync()
	return
}
