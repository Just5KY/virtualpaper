package process

import (
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"gopkg.in/gographics/imagick.v3/imagick"
	"os"
	"path"
	"time"
	"tryffel.net/go/virtualpaper/config"
	"tryffel.net/go/virtualpaper/models"
	"tryffel.net/go/virtualpaper/storage"
)

type fileProcessor struct {
	*Task
	document *models.Document
	input    chan fileOp
	file     string
	rawFile  *os.File
	tempFile *os.File
}

func newFileProcessor(id int, db *storage.Database) *fileProcessor {
	fp := &fileProcessor{
		Task:  newTask(id, db),
		input: make(chan fileOp, 5),
	}
	fp.idle = true
	fp.runFunc = fp.waitFile
	return fp
}

func (fp *fileProcessor) waitFile() {
	timer := time.NewTimer(time.Millisecond * 10)
	select {
	case <-timer.C:
		// pass

	case fileOp := <-fp.input:
		fp.file = fileOp.file
		fp.processFile()
	}
}

func (fp *fileProcessor) processFile() {
	logrus.Infof("task %d, process file %s", fp.id, fp.file)

	fp.lock.Lock()
	fp.idle = false
	fp.lock.Unlock()
	var err error

	fp.rawFile, err = os.OpenFile(fp.file, os.O_RDONLY, os.ModePerm)

	defer fp.cleanup()

	if err != nil {
		logrus.Errorf("process file %s, open: %v", fp.file, err)
		return
	}

	duplicate, err := fp.isDuplicate()
	if duplicate {
		logrus.Infof("file %s is a duplicate, ignore file", fp.file)
		return
	}

	if err != nil {
		logrus.Errorf("get duplicate status: %v", err)
		return
	}

	err = fp.createNewDocumentRecord()
	if err != nil {
		logrus.Error(err)
		return
	}

	err = fp.generateThumbnail()
	if err != nil {
		logrus.Error("generate thumbnail: %v", err)
		return
	}

}

func (fp *fileProcessor) cleanup() {
	logrus.Infof("Stop processing file %s", fp.file)
	fp.document = nil
	if fp.rawFile != nil {
		fp.rawFile.Close()
		fp.rawFile = nil
	}
	if fp.tempFile != nil {
		fp.tempFile.Close()

		err := os.Remove(fp.tempFile.Name())
		if err != nil {
			logrus.Errorf("remove temp file %s: %v", fp.tempFile.Name(), err)
		}
		fp.tempFile = nil
	}
	fp.file = ""
	fp.lock.Lock()
	fp.idle = true
	fp.lock.Unlock()
}

func (fp *fileProcessor) isDuplicate() (bool, error) {
	hash, err := getHash(fp.rawFile)
	if err != nil {
		return false, err
	}

	document, err := fp.db.DocumentStore.GetByHash(hash)
	if err != nil {
		if errors.Is(err, storage.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	if document != nil {
		return true, nil
	}
	return false, nil
}

func (fp *fileProcessor) createNewDocumentRecord() error {
	_, fileName := path.Split(fp.file)

	doc := &models.Document{
		Id:       0,
		UserId:   5,
		Name:     fileName,
		Content:  "",
		Filename: fileName,
		Preview:  "",
	}

	var err error
	doc.Hash, err = getHash(fp.rawFile)
	if err != nil {
		return fmt.Errorf("get hash: %v", err)
	}

	err = fp.db.DocumentStore.Create(doc)
	if err != nil {
		return fmt.Errorf("store document: %v", err)
	}

	fp.document = doc
	return nil
}

func (fp *fileProcessor) generateThumbnail() error {
	imagick.Initialize()
	defer imagick.Terminate()

	job := &models.Job{
		DocumentId: fp.document.Id,
		Message:    "Generate thumbnail (500x500)",
		Status:     models.JobRunning,
		StartedAt:  time.Now(),
		StoppedAt:  time.Time{},
	}
	defer fp.persistJob(job)

	output := path.Join(config.C.Processing.PreviewsDir, fp.document.Hash+".png")

	_, err := imagick.ConvertImageCommand([]string{
		"convert", "-thumbnail", "x500", "-background", "white", "-alpha", "remove", fp.file + "[0]", output,
	})

	if err != nil {
		job.Status = models.JobFailure
		job.Message += "; " + err.Error()
		return fmt.Errorf("call imagick: %v", err)
	}

	fp.document.Preview = fp.document.Hash + ".png"
	job.Status = models.JobFinished
	return nil
}

func (fp *fileProcessor) persistJob(job *models.Job) {
	job.StoppedAt = time.Now()
	err := fp.db.JobStore.Create(job.DocumentId, job)
	if err != nil {
		logrus.Errorf("save job to database: %v", err)
	}
}
