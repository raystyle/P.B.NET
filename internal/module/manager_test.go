package module

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/module/lcx"
	"project/internal/testsuite"
)

func testGenerateModule(t *testing.T) Module {
	dstNetwork := "tcp"
	dstAddress := "127.0.0.1:1234"
	opts := lcx.Options{LocalAddress: "127.0.0.1:0"}
	tranner, err := lcx.NewTranner("test", dstNetwork, dstAddress, logger.Test, &opts)
	require.NoError(t, err)
	return tranner
}

func TestManager_Add(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := NewManager()

	t.Run("ok", func(t *testing.T) {
		module := testGenerateModule(t)
		err := manager.Add("test", module)
		require.NoError(t, err)
	})

	t.Run("with empty tag", func(t *testing.T) {
		err := manager.Add("", nil)
		require.EqualError(t, err, "empty module tag")
	})

	t.Run("already exists", func(t *testing.T) {
		const tag = "test1"
		module := testGenerateModule(t)
		err := manager.Add(tag, module)
		require.NoError(t, err)
		err = manager.Add(tag, module)
		require.EqualError(t, err, "module test1 already exists")
	})

	manager.Close()
	testsuite.IsDestroyed(t, manager)
}

func TestManager_Delete(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := NewManager()

	t.Run("ok", func(t *testing.T) {
		const tag = "test"
		module := testGenerateModule(t)
		err := manager.Add(tag, module)
		require.NoError(t, err)
		err = manager.Delete(tag)
		require.NoError(t, err)
	})

	t.Run("with empty tag", func(t *testing.T) {
		err := manager.Delete("")
		require.EqualError(t, err, "empty module tag")
	})

	t.Run("doesn't exist", func(t *testing.T) {
		err := manager.Delete("tag")
		require.EqualError(t, err, "module tag doesn't exist")
	})

	manager.Close()
	testsuite.IsDestroyed(t, manager)
}

func TestManager_Get(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := NewManager()

	t.Run("ok", func(t *testing.T) {
		const tag = "test"
		module := testGenerateModule(t)
		err := manager.Add(tag, module)
		require.NoError(t, err)
		m, err := manager.Get(tag)
		require.NoError(t, err)
		require.NotNil(t, m)
	})

	t.Run("with empty tag", func(t *testing.T) {
		module, err := manager.Get("")
		require.EqualError(t, err, "empty module tag")
		require.Nil(t, module)
	})

	t.Run("doesn't exist", func(t *testing.T) {
		module, err := manager.Get("tag")
		require.EqualError(t, err, "module tag doesn't exist")
		require.Nil(t, module)
	})

	manager.Close()
	testsuite.IsDestroyed(t, manager)
}

func TestManager_Start(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := NewManager()

	t.Run("ok", func(t *testing.T) {
		const tag = "test"
		module := testGenerateModule(t)
		err := manager.Add(tag, module)
		require.NoError(t, err)
		err = manager.Start(tag)
		require.NoError(t, err)
	})

	t.Run("with empty tag", func(t *testing.T) {
		err := manager.Start("")
		require.EqualError(t, err, "empty module tag")
	})

	manager.Close()
	testsuite.IsDestroyed(t, manager)
}

func TestManager_Stop(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := NewManager()

	t.Run("ok", func(t *testing.T) {
		const tag = "test"
		module := testGenerateModule(t)
		err := manager.Add(tag, module)
		require.NoError(t, err)
		err = manager.Stop(tag)
		require.NoError(t, err)
	})

	t.Run("with empty tag", func(t *testing.T) {
		err := manager.Stop("")
		require.EqualError(t, err, "empty module tag")
	})

	manager.Close()
	testsuite.IsDestroyed(t, manager)
}

func TestManager_Restart(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := NewManager()

	t.Run("ok", func(t *testing.T) {
		const tag = "test"
		module := testGenerateModule(t)
		err := manager.Add(tag, module)
		require.NoError(t, err)
		err = manager.Restart(tag)
		require.NoError(t, err)
	})

	t.Run("with empty tag", func(t *testing.T) {
		err := manager.Restart("")
		require.EqualError(t, err, "empty module tag")
	})

	manager.Close()
	testsuite.IsDestroyed(t, manager)
}

func TestManager_Info(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := NewManager()

	t.Run("ok", func(t *testing.T) {
		const tag = "test"
		module := testGenerateModule(t)
		err := manager.Add(tag, module)
		require.NoError(t, err)
		info, err := manager.Info(tag)
		require.NoError(t, err)
		t.Log(info)
	})

	t.Run("with empty tag", func(t *testing.T) {
		info, err := manager.Info("")
		require.EqualError(t, err, "empty module tag")
		require.Equal(t, "", info)
	})

	manager.Close()
	testsuite.IsDestroyed(t, manager)
}

func TestManager_Status(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := NewManager()

	t.Run("ok", func(t *testing.T) {
		const tag = "test"
		module := testGenerateModule(t)
		err := manager.Add(tag, module)
		require.NoError(t, err)
		status, err := manager.Status(tag)
		require.NoError(t, err)
		t.Log(status)
	})

	t.Run("with empty tag", func(t *testing.T) {
		status, err := manager.Status("")
		require.EqualError(t, err, "empty module tag")
		require.Equal(t, "", status)
	})

	manager.Close()
	testsuite.IsDestroyed(t, manager)
}

func TestManager_Modules(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := NewManager()

	l := len(manager.Modules())
	require.Equal(t, 0, l)

	module := testGenerateModule(t)
	err := manager.Add("tag", module)
	require.NoError(t, err)
	l = len(manager.Modules())
	require.Equal(t, 1, l)

	manager.Close()
	l = len(manager.Modules())
	require.Equal(t, 0, l)
	testsuite.IsDestroyed(t, manager)
}

func TestManager_Parallel(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	manager := NewManager()

	const deleteTag = "delete"
	module := testGenerateModule(t)
	err := manager.Add(deleteTag, module)
	require.NoError(t, err)

	add := func() {
		err := manager.Add("test", module)
		require.NoError(t, err)
	}
	del := func() {
		_ = manager.Delete(deleteTag)
	}
	get := func() {
		module, err := manager.Get("")
		require.EqualError(t, err, "empty module tag")
		require.Nil(t, module)
	}
	modules := func() {
		modules := manager.Modules()
		require.NotNil(t, modules)
	}
	close1 := func() {
		manager.Close()
	}
	testsuite.RunParallel(add, del, get, modules, close1)

	manager.Close()
	testsuite.IsDestroyed(t, manager)
}
