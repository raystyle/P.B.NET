package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

type mockTask struct {
	Pause       bool
	PrepareErr  bool
	PrepareSlow bool

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

func (task *mockTask) Prepare(ctx context.Context) error {
	// check is canceled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if task.PrepareErr {
		return errors.New("mock task prepare error")
	}
	if task.PrepareSlow {
		// select {
		// case <-time.After(3 * time.Second):
		// case <-ctx.Done():
		// 	return ctx.Err()
		// }
		time.Sleep(3 * time.Second)
	}

	task.wg.Add(1)
	go task.watcher()
	return nil
}

func (task *mockTask) Process(ctx context.Context, t *Task) error {
	// check is canceled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// do something
	for i := 0; i < 5; i++ {
		// if task is paused, it will block here
		t.Paused()

		if task.Pause && i == 3 {
			err := t.Pause()
			if err != nil {
				return err
			}

			// UI block, wait user interact
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

func (task *mockTask) Progress() float32 {
	task.rwm.RLock()
	defer task.rwm.RUnlock()
	return task.progress
}

func (task *mockTask) Detail() string {
	task.rwm.RLock()
	defer task.rwm.RUnlock()
	return task.detail
}

func (task *mockTask) clean() {
	task.cancel()
	task.wg.Wait()
}

func TestTask(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	mt := testNewMockTask()
	mt.Pause = true
	task := New("mock", mt, nil)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		time.Sleep(100 * time.Millisecond)

		err := task.Pause()
		require.NoError(t, err)

		time.Sleep(time.Second)

		err = task.Continue()
		require.NoError(t, err)

		t.Log(task.Name())
		t.Log(task.State())
		t.Log(task.Progress())
		t.Log(task.Detail())
	}()

	err := task.Start()
	require.NoError(t, err)

	wg.Wait()

	mt.clean()

	testsuite.IsDestroyed(t, task)
	testsuite.IsDestroyed(t, mt)
}

func TestTask_Start(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		mt := testNewMockTask()
		task := New("mock", mt, nil)

		err := task.Start()
		require.NoError(t, err)

		mt.clean()

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})

	t.Run("cancel before start", func(t *testing.T) {
		mt := testNewMockTask()
		task := New("mock", mt, nil)

		task.Cancel()

		err := task.Start()
		require.Error(t, err)

		mt.clean()

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})

	t.Run("failed to prepare", func(t *testing.T) {
		mt := testNewMockTask()
		mt.PrepareErr = true
		task := New("mock", mt, nil)

		err := task.Start()
		require.Error(t, err)

		mt.clean()

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})

	t.Run("cancel before checkProcess", func(t *testing.T) {
		mt := testNewMockTask()
		mt.PrepareSlow = true
		task := New("mock", mt, nil)

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()

			time.Sleep(time.Second)
			task.Cancel()
		}()

		err := task.Start()
		require.Error(t, err)

		wg.Wait()

		mt.clean()

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})
}

func TestTask_Cancel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		mt := testNewMockTask()
		task := New("mock", mt, nil)

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()

			time.Sleep(600 * time.Millisecond)
			task.Cancel()
		}()

		err := task.Start()
		require.Error(t, err)

		wg.Wait()

		mt.clean()

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})
}
