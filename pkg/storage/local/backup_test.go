package local

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	backupFile    = "backup.txt"
	backupContent = "needs backup"
)

func createBackupFile() {
	file, _ := os.Create(backupFile)
	defer file.Close()
	file.WriteString(backupContent)
	file.Sync()
}

func writeErrorInBackupFile(t *testing.T) {
	file, err := os.OpenFile(backupFile, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	_, err = file.WriteString(" ERROR")
	if err != nil {
		t.Fatal(err)
	}
}

func getFileContent(filename string) string {
	file, _ := os.Open(filename)
	defer file.Close()

	c, _ := ioutil.ReadAll(file)
	return string(c)
}

func TestBackup(t *testing.T) {
	os.Remove(backupFile)

	t.Run("CreateBackup", func(t *testing.T) {
		assert := assert.New(t)
		createBackupFile()
		if !assert.Equal(backupContent, getFileContent(backupFile), "setup failed") {
			return
		}

		b, err := CreateBackup(backupFile)
		if assert.NoError(err) {
			assert.Equal(backupFile, b.original)
			assert.Equal(backupContent, getFileContent(b.backup))
			defer b.Delete()
		}
	})

	t.Run("Restore", func(t *testing.T) {
		assert := assert.New(t)

		// setup
		createBackupFile()
		b, err := CreateBackup(backupFile)
		if !assert.NoError(err, "Error creating backup") {
			return
		}
		defer b.Delete()

		writeErrorInBackupFile(t)
		assert.Equal("needs backup ERROR", getFileContent(backupFile), "setup failed")

		if assert.NoError(b.Restore()) {
			assert.Equal(backupContent, getFileContent(backupFile), "restore failed")
		}
	})

	os.Remove(backupFile)
}
