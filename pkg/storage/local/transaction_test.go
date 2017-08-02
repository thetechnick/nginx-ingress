package local

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	updateFile     = "update.txt"
	updateContent  = "--- update ---"
	updatedContent = "--- ---"
)

func createUpdateFile() {
	file, _ := os.Create(updateFile)
	defer file.Close()
	file.WriteString(updateContent)
}

func TestTransaction(t *testing.T) {
	os.Remove(updateFile)

	var transaction Transaction
	t.Run("Update create file", func(t *testing.T) {
		assert := assert.New(t)
		transaction = CreateTransaction()
		err := transaction.Update(updateFile, updateContent)

		if assert.NoError(err) {
			assert.Equal(updateContent, getFileContent(updateFile))
		}
	})

	t.Run("Rollback Update create file", func(t *testing.T) {
		transaction.Rollback()
		if _, err := os.Stat(updateFile); !os.IsNotExist(err) {
			t.Error("Created file still exists")
		}
		transaction.Apply()
	})

	t.Run("Update change file", func(t *testing.T) {
		createUpdateFile()

		assert := assert.New(t)
		transaction = CreateTransaction()
		err := transaction.Update(updateFile, updatedContent)
		if assert.NoError(err) {
			assert.Equal(updatedContent, getFileContent(updateFile))
		}
	})

	t.Run("Rollback Update change file", func(t *testing.T) {
		assert := assert.New(t)
		transaction.Rollback()
		assert.Equal(updateContent, getFileContent(updateFile))
	})

	t.Run("Delete file", func(t *testing.T) {
		assert := assert.New(t)
		err := transaction.Delete(updateFile)
		if assert.NoError(err) {
			if _, err := os.Stat(updateFile); !os.IsNotExist(err) {
				t.Error("File still exists")
			}
		}
	})

	t.Run("Rollback Deleet change file", func(t *testing.T) {
		transaction.Rollback()
		if _, err := os.Stat(updateFile); os.IsNotExist(err) {
			t.Error("File is still deleted")
		}
		transaction.Apply()
	})

	os.Remove(updateFile)
}
