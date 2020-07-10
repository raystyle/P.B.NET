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

const testTaskName = "mock"

type mockTask struct {
	Pause       bool
	PrepareErr  bool
	PrepareSlow bool
	ProcessSlow bool

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

func (mt *mockTask) Prepare(ctx context.Context) error {
	// some operation need context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if mt.PrepareErr {
		return errors.New("mock task prepare error")
	}
	if mt.PrepareSlow {
		// select {
		// case <-time.After(3 * time.Second):
		// case <-ctx.Done():
		// 	return ctx.Err()
		// }
		time.Sleep(3 * time.Second)
	}

	mt.wg.Add(1)
	go mt.watcher()
	return nil
}

func (mt *mockTask) Process(ctx context.Context, task *Task) error {
	if task.Canceled() {
		return ctx.Err()
	}

	if mt.ProcessSlow {
		time.Sleep(3 * time.Second)
		return nil
	}

	// do something
	for i := 0; i < 5; i++ {
		// if task is paused, it will block here
		task.Paused()

		// if task canceled return process at once.
		if task.Canceled() {
			return ctx.Err()
		}

		// self call Pause
		if mt.Pause && i == 3 {
			err := task.Pause()
			if err != nil {
				return err
			}

			// UI block, wait user interact
			time.Sleep(3 * time.Second)

			err = task.Continue()
			if err != nil {
				return err
			}
		}

		select {
		case <-time.After(200 * time.Millisecond):
			mt.updateProgress()
			mt.updateDetail(fmt.Sprintf("mock task detail: %d", i))
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (mt *mockTask) updateProgress() {
	mt.rwm.Lock()
	defer mt.rwm.Unlock()
	mt.progress += 0.2
}

func (mt *mockTask) updateDetail(detail string) {
	mt.rwm.Lock()
	defer mt.rwm.Unlock()
	mt.detail = detail
}

func (mt *mockTask) watcher() {
	defer mt.wg.Done()
	for {
		select {
		case <-time.After(time.Second):
			fmt.Println("watcher is alive")
		case <-mt.ctx.Done():
			return
		}
	}
}

func (mt *mockTask) Progress() float32 {
	mt.rwm.RLock()
	defer mt.rwm.RUnlock()
	return mt.progress
}

func (mt *mockTask) Detail() string {
	mt.rwm.RLock()
	defer mt.rwm.RUnlock()
	return mt.detail
}

func (mt *mockTask) Clean() {
	mt.cancel()
	mt.wg.Wait()
}

func TestTask(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	mt := testNewMockTask()
	mt.Pause = true
	task := New(testTaskName, mt, nil)

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

	testsuite.IsDestroyed(t, task)
	testsuite.IsDestroyed(t, mt)
}

func TestTask_Start(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		mt := testNewMockTask()
		task := New(testTaskName, mt, nil)

		err := task.Start()
		require.NoError(t, err)

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})

	t.Run("cancel before start", func(t *testing.T) {
		mt := testNewMockTask()
		task := New(testTaskName, mt, nil)

		task.Cancel()

		err := task.Start()
		require.Error(t, err)

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})

	t.Run("failed to prepare", func(t *testing.T) {
		mt := testNewMockTask()
		mt.PrepareErr = true
		task := New(testTaskName, mt, nil)

		err := task.Start()
		require.Error(t, err)

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})

	t.Run("cancel before checkProcess", func(t *testing.T) {
		mt := testNewMockTask()
		mt.PrepareSlow = true
		task := New(testTaskName, mt, nil)

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

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})

	t.Run("panic in checkStart", func(t *testing.T) {
		mt := testNewMockTask()
		task := New(testTaskName, mt, nil)

		// set invalid state
		task.fsm.SetState(StateCancel)

		err := task.Start()
		require.Error(t, err)

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})

	t.Run("panic in checkProcess", func(t *testing.T) {
		mt := testNewMockTask()
		mt.PrepareSlow = true
		task := New(testTaskName, mt, nil)

		// set invalid state
		go func() {
			time.Sleep(time.Second)
			task.fsm.SetState(StateCancel)
		}()

		err := task.Start()
		require.Error(t, err)

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})

	t.Run("cancel before event complete", func(t *testing.T) {
		mt := testNewMockTask()
		mt.ProcessSlow = true
		task := New(testTaskName, mt, nil)

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

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})

	t.Run("panic in process", func(t *testing.T) {
		mt := testNewMockTask()
		task := New(testTaskName, mt, nil)

		// set invalid state
		go func() {
			time.Sleep(300 * time.Millisecond)
			task.fsm.SetState(StateCancel)
		}()

		err := task.Start()
		require.Error(t, err)

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})
}

func TestTask_Pause(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		mt := testNewMockTask()
		task := New(testTaskName, mt, nil)

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
		}()

		err := task.Start()
		require.NoError(t, err)

		wg.Wait()

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})
}

func TestTask_Continue(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {

	})
}

func TestTask_Cancel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		mt := testNewMockTask()
		task := New(testTaskName, mt, nil)

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

		testsuite.IsDestroyed(t, task)
		testsuite.IsDestroyed(t, mt)
	})
}
