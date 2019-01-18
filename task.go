package async

import (
	"fmt"
	"sync"
)

// Delegate forms the body of an async.Run
type Delegate func() (interface{}, error)

// Continuation forms the signature of task continuation function
type Continuation func(Task) (interface{}, error)

// Task represents an asynchronous operation
type Task interface {
	Result() (interface{}, error)
	ContinueWith(Continuation) Task
	IsComplete() bool
	Wait()
}

type task struct {
	errChannel   chan error
	valueChannel chan interface{}
	doneChannel  chan bool

	err   error
	value interface{}
	done  bool

	mutext sync.Mutex
}

func (t *task) Wait() {
	if t.IsComplete() {
		return
	}

	t.mutext.Lock()
	defer t.mutext.Unlock()

	// check once again inside the mutex just to make sure another operation didn't set the done variable
	if t.done {
		return
	}

	// no default case, block until one channel returns a value
	select {
	case t.err = <-t.errChannel:
	case t.value = <-t.valueChannel:
	}

	// do a non blocking check of the done channel
	_ = t.checkDone()
}

func (t *task) Result() (interface{}, error) {
	t.Wait()
	return t.value, t.err
}

func (t *task) ContinueWith(continuation Continuation) Task {
	if t.done {
		return Run(func() (interface{}, error) {
			return continuation(t)
		})
	}
	return Run(func() (interface{}, error) {
		_, err := t.Result()
		if err != nil {
			return nil, err
		}
		return continuation(t)
	})
}

func (t *task) IsComplete() bool {
	if t.done {
		return true
	}

	t.mutext.Lock()
	defer t.mutext.Unlock()
	if t.done {
		return true
	}

	// do a non blocking check of the done channel
	return t.checkDone()
}

func (t *task) checkDone() bool {
	// do a non blocking check of the done channel
	select {
	case t.done = <-t.doneChannel:
		return t.done
	default:
		return false
	}
}

// WhenAll creates a task that waits on all tasks to finish
func WhenAll(tasks ...Task) Task {
	return Run(func() (interface{}, error) {
		if len(tasks) == 0 {
			return nil, nil
		}

		var waitGroup sync.WaitGroup
		waitGroup.Add(len(tasks))
		for _, task := range tasks {
			go func(task Task) {
				defer waitGroup.Done()
				_, _ = task.Result()
			}(task)
		}
		waitGroup.Wait()

		return nil, nil
	})
}

// WaitAll blocks until all tasks are completed
func WaitAll(tasks ...Task) {
	t := WhenAll(tasks...)
	t.Wait()
}

// WhenAny waits on a single task to respond and returns the responding task as the Task Result (task in task)
func WhenAny(tasks ...Task) Task {
	return Run(func() (interface{}, error) {
		if len(tasks) == 0 {
			return nil, nil
		}

		finished := make(chan Task)

		var waitGroup sync.WaitGroup
		waitGroup.Add(1)

		// perhaps add cancelation here to avoid blocking these unfinished go routines?
		for _, task := range tasks {
			go func(task Task, finished chan Task) {
				defer waitGroup.Done()
				_, _ = task.Result()
				finished <- task
			}(task, finished)
		}
		waitGroup.Wait()
		return <-finished, nil
	})
}

// WaitAny blocks until any task is completed. The task that first responds is returned
func WaitAny(tasks ...Task) Task {
	t := WhenAny()
	value, err := t.Result()
	if err != nil {
		return FromError(err)
	}

	// create unwrap function?
	innerTask, ok := value.(Task)
	if !ok {
		return FromError(fmt.Errorf("unable to cast value to task"))
	}
	return innerTask
}

// FromResult returns a task that wraps a result
func FromResult(value interface{}) Task {
	return FromCompleted(value, nil)
}

// FromError returns a task that wraps a error
func FromError(err error) Task {
	return FromCompleted(nil, err)
}

// FromCompleted returns a completed task
func FromCompleted(value interface{}, err error) Task {
	return &task{
		err:   err,
		value: value,
		done:  true,
	}
}

// Run runs the given delegate in a go routine and returns a task
func Run(delegate Delegate) Task {
	t := create()
	go func() {
		defer close(t.errChannel)
		defer close(t.valueChannel)
		defer close(t.doneChannel)

		value, err := delegate()
		if err != nil {
			t.errChannel <- err
		} else {
			t.valueChannel <- value
		}
		// make sure to write to done only after writing to any other channel
		t.doneChannel <- true
	}()
	return t
}

func create() *task {

	channel := make(chan interface{})
	errCh := make(chan error)
	doneCh := make(chan bool)

	return &task{
		errChannel:   errCh,
		doneChannel:  doneCh,
		valueChannel: channel,
	}
}
