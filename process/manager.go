package process

import (
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"math/rand"
	"sync"
	"time"
	config "tryffel.net/go/virtualpaper/config"
	"tryffel.net/go/virtualpaper/storage"
)

type fileOp struct {
	file string
}

// Manager manages multiple goroutines processing files.
type Manager struct {
	lock       *sync.RWMutex
	running    bool
	reportChan chan TaskReport
	db         *storage.Database

	tasks    []*fileProcessor
	numtasks int

	inputWatch *fsnotify.Watcher
}

func NewManager(database *storage.Database) (*Manager, error) {
	manager := &Manager{
		lock:       &sync.RWMutex{},
		reportChan: make(chan TaskReport, 10),
		db:         database,
	}

	count := config.C.Processing.MaxWorkers
	manager.numtasks = count
	manager.tasks = make([]*fileProcessor, count)

	for i := 0; i < count; i++ {
		manager.tasks[i] = newFileProcessor(i, database)
	}
	var err error
	manager.inputWatch, err = fsnotify.NewWatcher()
	return manager, err
}

func (m *Manager) Start() error {
	if m.isRunning() {
		return errors.New("already running")
	}

	logrus.Infof("Watch directory %s", config.C.Processing.InputDir)

	err := m.inputWatch.Add(config.C.Processing.InputDir)
	if err != nil {
		return fmt.Errorf("add input directory watch: %v", err)
	}

	for _, task := range m.tasks {
		task.Start()
	}

	f := func() {
		m.lock.Lock()
		m.running = true
		m.lock.Unlock()
		logrus.Debug("start background task manager")

		for m.isRunning() {
			m.runFunc()
		}
		m.inputWatch.Close()
		logrus.Debug("background task manager stopped")
	}

	go f()
	return nil
}

func (m *Manager) Stop() error {
	if !m.isRunning() {
		return errors.New("not running")
	}

	for _, task := range m.tasks {
		task.Stop()
	}

	m.lock.Lock()
	m.running = false
	m.lock.Unlock()
	return nil
}

func (m *Manager) isRunning() bool {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.running
}

// async function loop to wait for events and launch tasks.
func (m *Manager) runFunc() {
	timer := time.NewTimer(time.Millisecond * 100)

	select {
	case <-timer.C:
		// pass

	case event, ok := <-m.inputWatch.Events:
		if ok {
			logrus.Infof("Got file watcher event: %v", event)
		}

		if event.Op == fsnotify.Write {
			logrus.Infof("Schedule processing for file %s", event.Name)
			m.scheduleNewOp(event.Name)
		}

		//pass

	case report := <-m.reportChan:
		logrus.Infof("Got task report: %v", report)

	}
	time.Sleep(time.Second)
}

// schedule file operation to any idle task. If none of the tasks are idle, queue it to random task.
func (m *Manager) scheduleNewOp(file string) {
	op := fileOp{file: file}
	scheduled := false

	for _, task := range m.tasks {
		if task.isIdle() {
			task.input <- op
			scheduled = true
			break
		}
	}

	if !scheduled {
		id := rand.Intn(m.numtasks)
		m.tasks[id].input <- op
	}
}
