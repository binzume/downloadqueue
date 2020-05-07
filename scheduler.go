package downloader

import (
	"context"
	"sync"
)

type Task interface {
	Run()
}

type TaskScheduler struct {
	semaphoreCh chan struct{}
	taskCh      chan Task
	wg          sync.WaitGroup
	mutex       sync.RWMutex
	tasks       map[string]*TaskState
}

func NewTaskScheduler(parallel int, queueLen int, start bool) *TaskScheduler {
	d := &TaskScheduler{
		semaphoreCh: make(chan struct{}, parallel),
		taskCh:      make(chan Task, queueLen),
		tasks:       map[string]*TaskState{},
	}
	if start {
		d.Start(context.Background())
	}
	return d
}

func (d *TaskScheduler) Start(ctx context.Context) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case d.semaphoreCh <- struct{}{}:
				select {
				case <-ctx.Done():
					<-d.semaphoreCh
					return
				case task := <-d.taskCh:
					d.wg.Add(1)

					go func() {
						defer d.wg.Done()
						defer func() { <-d.semaphoreCh }()
						task.Run()
					}()
				}
			}
		}
	}()
}

func (d *TaskScheduler) Wait() {
	d.wg.Wait()
}

func (d *TaskScheduler) PostTask(t Task, block bool) bool {
	if block {
		d.taskCh <- t
	} else {
		select {
		case d.taskCh <- t:
		default:
			return false
		}
	}
	return true
}

type TaskState struct {
	task Task
	id   string
	done chan struct{}
	d    *TaskScheduler
}

func (t *TaskState) ID() string {
	return t.id
}

func (t *TaskState) Done() <-chan struct{} {
	return t.done
}

func (t *TaskState) Run() {
	defer t.finish()
	t.task.Run()
}

func (t *TaskState) finish() {
	close(t.done)

	t.d.mutex.Lock()
	defer t.d.mutex.Unlock()
	delete(t.d.tasks, t.ID())
}

func (d *TaskScheduler) addTaskState(task Task, id string, block bool) *TaskState {
	var once sync.Once
	d.mutex.Lock()
	defer once.Do(func() { d.mutex.Unlock() })

	if t, exists := d.tasks[id]; exists {
		return t
	}
	ts := &TaskState{task, id, make(chan struct{}), d}
	if id != "" {
		d.tasks[id] = ts
	}
	if !block {
		if !d.PostTask(ts, block) {
			delete(d.tasks, id)
			return nil
		}
	} else {
		once.Do(func() { d.mutex.Unlock() })
		d.PostTask(ts, block)
	}
	return ts
}

func (d *TaskScheduler) PostWithId(task Task, id string) *TaskState {
	return d.addTaskState(task, id, true)
}

func (d *TaskScheduler) TryPostWithId(task Task, id string) *TaskState {
	return d.addTaskState(task, id, false)
}

type taskFunc func()

func (f taskFunc) Run() {
	f()
}

func (d *TaskScheduler) PostFunc(taskFn func(), id string) *TaskState {
	return d.PostWithId(taskFunc(taskFn), id)
}

func (d *TaskScheduler) TryPostFunc(taskFn func(), id string) *TaskState {
	return d.TryPostWithId(taskFunc(taskFn), id)
}
