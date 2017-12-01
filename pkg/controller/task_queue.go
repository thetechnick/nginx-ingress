package controller

import (
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

// TaskQueue manages a work queue through an independent worker that
// invokes the given sync function for every work item inserted.
type TaskQueue interface {
	Run(period time.Duration, stopCh <-chan struct{})
	// Enqueue enqueues ns/name of the given api object in the task queue.
	Enqueue(obj interface{})
	Requeue(key string, err error)
	RequeueAfter(key string, err error, after time.Duration)
	// Shutdown shuts down the work queue and waits for the worker to ACK
	Shutdown()
}

type taskQueue struct {
	// queue is the work queue the worker polls
	queue *workqueue.Type
	// sync is called for each item in the queue
	sync func(string)
	// workerDone is closed when the worker exits
	workerDone chan struct{}
	log        *log.Entry
}

// NewTaskQueue creates a new task queue with the given sync function.
// The sync function is called for every element inserted into the queue.
func NewTaskQueue(syncFn func(string), log *log.Entry) TaskQueue {
	tq := &taskQueue{
		queue:      workqueue.New(),
		sync:       syncFn,
		workerDone: make(chan struct{}),
		log:        log,
	}
	if tq.log == nil {
		tq.log = log.WithField("module", "Generic TaskQueue")
	}
	return tq
}

func (t *taskQueue) Run(period time.Duration, stopCh <-chan struct{}) {
	wait.Until(t.worker, period, stopCh)
}

func (t *taskQueue) Enqueue(obj interface{}) {
	key, err := keyFunc(obj)
	if err != nil {
		log.
			WithField("obj", obj).
			WithError(err).
			Error("Couldn't get key for object, skipping")
		return
	}
	t.queue.Add(key)
}

func (t *taskQueue) Requeue(key string, err error) {
	log.
		WithField("key", key).
		WithError(err).
		Error("Requeuing")
	t.queue.Add(key)
}

func (t *taskQueue) RequeueAfter(key string, err error, after time.Duration) {
	log.
		WithField("key", key).
		WithError(err).
		Errorf("Requeuing after %s", after.String())
	go func(key string, after time.Duration) {
		time.Sleep(after)
		t.queue.Add(key)
	}(key, after)
}

func (t *taskQueue) Shutdown() {
	t.queue.ShutDown()
	<-t.workerDone
}

// worker processes work in the queue through sync.
func (t *taskQueue) worker() {
	for {
		key, quit := t.queue.Get()
		if quit {
			close(t.workerDone)
			return
		}
		log.WithField("key", key).Debug("Syncing form taskQueue")
		t.sync(key.(string))
		t.queue.Done(key)
	}
}
