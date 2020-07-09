package task

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

type mockTask struct {
	Pause bool

	progress float32
	detail   string
	rwm      sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func testNewMockTask() *mockTask {
	task := mockTask{}
	task.ctx, task.cancel = context.WithCancel(context.Background())
	return &task
}

func (task *mockTask) prepare(ctx context.Context) error {
	// check is canceled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	task.wg.Add(1)
	go task.watcher()
	return nil
}

func (task *mockTask) process(ctx context.Context, t *Task) error {
	// check is canceled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// do something
	for i := 0; i < 5; i++ {
		t.Paused() // if task is paused, it will block here

		if task.Pause && i == 3 {
			err := t.Pause()
			if err != nil {
				return err
			}

			// UI block
			time.Sleep(3 * time.Second)

			err = t.Continue()
			if err != nil {
				return err
			}
		}

		select {
		case <-time.After(200 * time.Millisecond):
			task.updateProgress()
			task.updateDetail(fmt.Sprintf("mock task detail: %d", i))
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (task *mockTask) updateProgress() {
	task.rwm.Lock()
	defer task.rwm.Unlock()
	task.progress += 0.2
}

func (task *mockTask) updateDetail(detail string) {
	task.rwm.Lock()
	defer task.rwm.Unlock()
	task.detail = detail
}

func (task *mockTask) watcher() {
	defer task.wg.Done()
	for {
		select {
		case <-time.After(time.Second):
			fmt.Println("watcher is alive")
		case <-task.ctx.Done():
			return
		}
	}
}

func (task *mockTask) getProgress() float32 {
	task.rwm.RLock()
	defer task.rwm.RUnlock()
	return task.progress
}

func (task *mockTask) getDetail() string {
	task.rwm.RLock()
	defer task.rwm.RUnlock()
	return task.detail
}

func (task *mockTask) clean() {
	task.cancel()
	task.wg.Wait()
}

func TestTask(t *testing.T) {

}

func TestTask_Start(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		mt := testNewMockTask()
		// defer mt.clean()

		cfg := Config{
			Prepare:  mt.prepare,
			Process:  mt.process,
			Progress: mt.getProgress,
			Detail:   mt.getDetail,
		}
		task := New("mock", &cfg)

		err := task.Start()
		require.NoError(t, err)

		mt.clean()

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})
}
