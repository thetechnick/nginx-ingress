package local

import (
	"os"

	log "github.com/sirupsen/logrus"
)

// TransactionFactory creates new transactions
type TransactionFactory func() Transaction

// Transaction records file changes and is able to roll them back
type Transaction interface {
	Delete(filename string) error
	Update(filename, content string) error
	Rollback()
	Apply()
}

type transaction struct {
	backups      []*Backup
	createdFiles []string
	log          *log.Entry
	createBackup BackupFactory
}

// CreateTransaction creates a new transaction instance
func CreateTransaction() Transaction {
	return &transaction{
		backups:      []*Backup{},
		createdFiles: []string{},
		log:          log.WithField("module", "transaction"),
		createBackup: CreateBackup,
	}
}

// Delete deletes a file
func (t *transaction) Delete(filename string) error {
	backup, err := t.createBackup(filename)
	if err != nil {
		if err == ErrBackupOriginalFileNotExist {
			return nil
		}
		return err
	}

	err = os.Remove(filename)
	if err != nil {
		return err
	}
	t.backups = append(t.backups, backup)
	return nil
}

// Update changes a file or creates it
func (t *transaction) Update(filename, content string) error {
	backup, err := t.createBackup(filename)
	if err != nil && err != ErrBackupOriginalFileNotExist {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return err
	}

	if backup != nil {
		t.backups = append(t.backups, backup)
	} else {
		t.createdFiles = append(t.createdFiles, filename)
	}
	return nil
}

// Rollback reverts the changes
func (t *transaction) Rollback() {
	for _, createdFile := range t.createdFiles {
		err := os.Remove(createdFile)
		if err != nil {
			t.log.
				WithError(err).
				Error("Error removing created file")
		}
	}
	for _, b := range t.backups {
		err := b.Restore()
		if err != nil {
			t.log.
				WithError(err).
				Error("Error restoring backup")
		}
	}
}

// Apply deletes the backup files
func (t *transaction) Apply() {
	for _, b := range t.backups {
		err := b.Delete()
		if err != nil {
			t.log.
				WithError(err).
				Error("Error deleting backup")
		}
	}
}
