package msfrpc

import (
	"context"
	"encoding/hex"
	"io/ioutil"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestMSFRPC_ModuleExploits(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := msfrpc.ModuleExploits(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		modules, err := msfrpc.ModuleExploits(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, modules)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.ModuleExploits(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleAuxiliary(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := msfrpc.ModuleAuxiliary(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		modules, err := msfrpc.ModuleAuxiliary(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, modules)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.ModuleAuxiliary(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModulePost(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := msfrpc.ModulePost(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		modules, err := msfrpc.ModulePost(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, modules)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.ModulePost(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModulePayloads(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := msfrpc.ModulePayloads(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		modules, err := msfrpc.ModulePayloads(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, modules)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.ModulePayloads(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleEncoders(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := msfrpc.ModuleEncoders(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		modules, err := msfrpc.ModuleEncoders(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, modules)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.ModuleEncoders(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleNops(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := msfrpc.ModuleNops(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		modules, err := msfrpc.ModuleNops(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, modules)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.ModuleNops(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleEvasion(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := msfrpc.ModuleEvasion(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		modules, err := msfrpc.ModuleEvasion(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, modules)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			modules, err := msfrpc.ModuleEvasion(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleInfo(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		info, err := msfrpc.ModuleInfo(ctx, "exploit", "multi/handler")
		require.NoError(t, err)

		t.Log(info.Type)
		t.Log(info.Name)
		t.Log(info.FullName)
		t.Log(info.Rank)
		t.Log(info.DisclosureDate)
		t.Log(info.Description)
		t.Log(info.License)
		t.Log(info.Filepath)
		t.Log(info.Arch)
		t.Log(info.Platform)
		t.Log(info.Authors)
		t.Log(info.Privileged)
		t.Log(info.References)
		t.Log(info.Targets)
		t.Log(info.DefaultTarget)
		t.Log(info.Stance)
		t.Log("----------options----------")
		for name, opt := range info.Options {
			t.Log("name:", name)
			t.Log(opt.Type)
			t.Log(opt.Required)
			t.Log(opt.Advanced)
			t.Log(opt.Description)
			t.Log(opt.Default)
			t.Log("------------------------------")
		}
	})

	t.Run("failed", func(t *testing.T) {
		info, err := msfrpc.ModuleInfo(ctx, "foo type", "bar name")
		require.EqualError(t, err, "Invalid Module")
		require.Nil(t, info)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		info, err := msfrpc.ModuleInfo(ctx, "foo", "bar")
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, info)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			info, err := msfrpc.ModuleInfo(ctx, "foo", "bar")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, info)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleOptions(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		options, err := msfrpc.ModuleOptions(ctx, "exploit", "multi/handler")
		require.NoError(t, err)

		for name, option := range options {
			t.Log("name:", name)
			t.Log(option.Type)
			t.Log(option.Required)
			t.Log(option.Advanced)
			t.Log(option.Evasion)
			t.Log(option.Description)
			t.Log(option.Default)
			t.Log(option.Enums)
			t.Log("------------------------------")
		}
	})

	t.Run("failed", func(t *testing.T) {
		options, err := msfrpc.ModuleOptions(ctx, "foo type", "bar name")
		require.EqualError(t, err, "Invalid Module")
		require.Nil(t, options)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		options, err := msfrpc.ModuleOptions(ctx, "foo", "bar")
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, options)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			options, err := msfrpc.ModuleOptions(ctx, "foo", "bar")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, options)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleCompatiblePayloads(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		payloads, err := msfrpc.ModuleCompatiblePayloads(ctx, "exploit/multi/handler")
		require.NoError(t, err)
		for i := 0; i < len(payloads); i++ {
			t.Log(payloads[i])
		}
	})

	t.Run("failed", func(t *testing.T) {
		payloads, err := msfrpc.ModuleCompatiblePayloads(ctx, "foo")
		require.EqualError(t, err, "Invalid Module")
		require.Nil(t, payloads)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		payloads, err := msfrpc.ModuleCompatiblePayloads(ctx, "foo")
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, payloads)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			payloads, err := msfrpc.ModuleCompatiblePayloads(ctx, "foo")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, payloads)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleTargetCompatiblePayloads(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()
	const (
		module  = "exploit/multi/handler"
		target  = 0
		iModule = "foo"
	)

	t.Run("success", func(t *testing.T) {
		payloads, err := msfrpc.ModuleTargetCompatiblePayloads(ctx, module, target)
		require.NoError(t, err)
		for i := 0; i < len(payloads); i++ {
			t.Log(payloads[i])
		}
	})

	t.Run("invalid module", func(t *testing.T) {
		payloads, err := msfrpc.ModuleTargetCompatiblePayloads(ctx, iModule, target)
		require.EqualError(t, err, "Invalid Module")
		require.Nil(t, payloads)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		payloads, err := msfrpc.ModuleTargetCompatiblePayloads(ctx, iModule, target)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, payloads)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			payloads, err := msfrpc.ModuleTargetCompatiblePayloads(ctx, iModule, target)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, payloads)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleCompatibleSessions(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()
	const module = "post/windows/gather/enum_proxy"

	t.Run("success", func(t *testing.T) {
		sessions, err := msfrpc.ModuleCompatibleSessions(ctx, module)
		require.NoError(t, err)
		// now is noting
		for i := 0; i < len(sessions); i++ {
			t.Log(sessions[i])
		}
	})

	t.Run("invalid module", func(t *testing.T) {
		sessions, err := msfrpc.ModuleCompatibleSessions(ctx, "foo")
		require.EqualError(t, err, "Invalid Module")
		require.Nil(t, sessions)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		sessions, err := msfrpc.ModuleCompatibleSessions(ctx, "foo")
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, sessions)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			sessions, err := msfrpc.ModuleCompatibleSessions(ctx, "foo")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, sessions)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleCompatibleEvasionPayloads(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()
	const module = "windows/windows_defender_exe"

	t.Run("success", func(t *testing.T) {
		payloads, err := msfrpc.ModuleCompatibleEvasionPayloads(ctx, module)
		require.NoError(t, err)
		for i := 0; i < len(payloads); i++ {
			t.Log(payloads[i])
		}
	})

	t.Run("invalid module", func(t *testing.T) {
		payloads, err := msfrpc.ModuleCompatibleEvasionPayloads(ctx, "foo")
		require.EqualError(t, err, "Invalid Module")
		require.Nil(t, payloads)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		payloads, err := msfrpc.ModuleCompatibleEvasionPayloads(ctx, "foo")
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, payloads)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			payloads, err := msfrpc.ModuleCompatibleEvasionPayloads(ctx, "foo")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, payloads)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleTargetCompatibleEvasionPayloads(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()
	const (
		module  = "windows/windows_defender_exe"
		target  = 0
		iModule = "foo"
	)

	t.Run("success", func(t *testing.T) {
		payloads, err := msfrpc.ModuleTargetCompatibleEvasionPayloads(ctx, module, target)
		require.NoError(t, err)
		for i := 0; i < len(payloads); i++ {
			t.Log(payloads[i])
		}
	})

	t.Run("invalid module", func(t *testing.T) {
		payloads, err := msfrpc.ModuleTargetCompatibleEvasionPayloads(ctx, iModule, target)
		require.EqualError(t, err, "Invalid Module")
		require.Nil(t, payloads)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		payloads, err := msfrpc.ModuleTargetCompatibleEvasionPayloads(ctx, iModule, target)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, payloads)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			payloads, err := msfrpc.ModuleTargetCompatibleEvasionPayloads(ctx, iModule, target)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, payloads)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleEncodeFormats(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		formats, err := msfrpc.ModuleEncodeFormats(ctx)
		require.NoError(t, err)
		for i := 0; i < len(formats); i++ {
			t.Log(formats[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		formats, err := msfrpc.ModuleEncodeFormats(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, formats)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			formats, err := msfrpc.ModuleEncodeFormats(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, formats)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleExecutableFormats(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		formats, err := msfrpc.ModuleExecutableFormats(ctx)
		require.NoError(t, err)
		for i := 0; i < len(formats); i++ {
			t.Log(formats[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		formats, err := msfrpc.ModuleExecutableFormats(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, formats)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			formats, err := msfrpc.ModuleExecutableFormats(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, formats)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleTransformFormats(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		formats, err := msfrpc.ModuleTransformFormats(ctx)
		require.NoError(t, err)
		for i := 0; i < len(formats); i++ {
			t.Log(formats[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		formats, err := msfrpc.ModuleTransformFormats(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, formats)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			formats, err := msfrpc.ModuleTransformFormats(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, formats)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleEncryptionFormats(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		formats, err := msfrpc.ModuleEncryptionFormats(ctx)
		require.NoError(t, err)
		for i := 0; i < len(formats); i++ {
			t.Log(formats[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		formats, err := msfrpc.ModuleEncryptionFormats(ctx)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, formats)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			formats, err := msfrpc.ModuleEncryptionFormats(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, formats)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleEncode(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()
	const (
		data    = "AAA"
		encoder = "x86/shikata_ga_nai"
		path    = "../internal/module/shellcode/testdata/windows_32.txt"
	)
	// read shellcode
	scHEX, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	sc := make([]byte, len(scHEX)/2)
	_, err = hex.Decode(sc, scHEX)
	require.NoError(t, err)
	opts := &ModuleEncodeOptions{
		Format:       "c",
		EncodeCount:  1,
		AddShellcode: string(sc),
	}

	t.Run("success", func(t *testing.T) {
		encoded, err := msfrpc.ModuleEncode(ctx, data, encoder, opts)
		require.NoError(t, err)
		t.Logf("\n%s\n", encoded)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		encoded, err := msfrpc.ModuleEncode(ctx, data, encoder, opts)
		require.EqualError(t, err, testErrInvalidToken)
		require.Zero(t, encoded)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			encoded, err := msfrpc.ModuleEncode(ctx, data, encoder, opts)
			monkey.IsMonkeyError(t, err)
			require.Zero(t, encoded)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleExecute(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("exploit", func(t *testing.T) {
		const exploit = "multi/handler"
		opts := make(map[string]interface{})
		opts["PAYLOAD"] = "windows/meterpreter/reverse_tcp"
		opts["TARGET"] = 0
		opts["LHOST"] = "127.0.0.1"
		opts["LPORT"] = "0"

		t.Run("success", func(t *testing.T) {
			result, err := msfrpc.ModuleExecute(ctx, "exploit", exploit, opts)
			require.NoError(t, err)

			jobID := strconv.FormatUint(result.JobID, 10)
			info, err := msfrpc.JobInfo(jobID)
			require.NoError(t, err)
			t.Log(info.Name)
			for key, value := range info.DataStore {
				t.Log(key, value)
			}
			err = msfrpc.JobStop(jobID)
			require.NoError(t, err)
		})

		t.Run("invalid port", func(t *testing.T) {
			opts["LPORT"] = "foo"
			defer func() { opts["LPORT"] = "0" }()
			result, err := msfrpc.ModuleExecute(ctx, "exploit", exploit, opts)
			require.NoError(t, err)

			jobID := strconv.FormatUint(result.JobID, 10)
			info, err := msfrpc.JobInfo(jobID)
			require.Error(t, err)
			require.Nil(t, info)
		})
	})

	t.Run("generate payload", func(t *testing.T) {
		const payload = "windows/meterpreter/reverse_tcp"
		opts := NewModuleExecuteOptions()
		opts.Format = "c"
		opts.Iterations = 1
		opts.DataStore["LHOST"] = "127.0.0.1"
		opts.DataStore["LPORT"] = "1999"

		t.Run("success", func(t *testing.T) {
			result, err := msfrpc.ModuleExecute(ctx, "payload", payload, opts)
			require.NoError(t, err)
			t.Log(result.Payload)
		})

		t.Run("invalid port", func(t *testing.T) {
			const errStr = "failed to generate: One or more options failed to validate: LPORT."
			opts.DataStore["LPORT"] = "foo"
			defer func() { opts.DataStore["LPORT"] = "1999" }()
			result, err := msfrpc.ModuleExecute(ctx, "payload", payload, opts)
			require.EqualError(t, err, errStr)
			require.Nil(t, result)
		})
	})

	t.Run("invalid module type", func(t *testing.T) {
		result, err := msfrpc.ModuleExecute(ctx, "foo", "bar", nil)
		require.EqualError(t, err, "invalid module type: foo")
		require.Nil(t, result)
	})

	const (
		typ  = "exploit"
		name = "foo"
	)
	opts := make(map[string]interface{})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		result, err := msfrpc.ModuleExecute(ctx, typ, name, opts)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, result)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			result, err := msfrpc.ModuleExecute(ctx, typ, name, opts)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, result)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}

func TestMSFRPC_ModuleCheck(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	msfrpc, err := NewMSFRPC(testHost, testPort, testUsername, testPassword, nil)
	require.NoError(t, err)
	err = msfrpc.Login()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		const exploit = "windows/smb/ms17_010_eternalblue"
		opts := make(map[string]interface{})
		opts["TARGET"] = 0
		opts["RHOST"] = "127.0.0.1"
		opts["RPORT"] = "1"
		opts["PAYLOAD"] = "windows/meterpreter/reverse_tcp"
		opts["LHOST"] = "127.0.0.1"
		opts["LPORT"] = "1999"

		result, err := msfrpc.ModuleCheck(ctx, "exploit", exploit, opts)
		require.NoError(t, err)

		jobID := strconv.FormatUint(result.JobID, 10)
		info, err := msfrpc.JobInfo(jobID)
		require.NoError(t, err)
		t.Log(info.Name)
		for key, value := range info.DataStore {
			t.Log(key, value)
		}
		err = msfrpc.JobStop(jobID)
		require.NoError(t, err)
	})

	t.Run("invalid module type", func(t *testing.T) {
		result, err := msfrpc.ModuleCheck(ctx, "foo", "bar", nil)
		require.EqualError(t, err, "invalid module type: foo")
		require.Nil(t, result)
	})

	const (
		typ  = "exploit"
		name = "foo"
	)

	t.Run("invalid authentication token", func(t *testing.T) {
		token := msfrpc.GetToken()
		defer msfrpc.SetToken(token)
		msfrpc.SetToken(testInvalidToken)
		result, err := msfrpc.ModuleCheck(ctx, typ, name, nil)
		require.EqualError(t, err, testErrInvalidToken)
		require.Nil(t, result)
	})

	t.Run("send failed", func(t *testing.T) {
		testPatchSend(func() {
			result, err := msfrpc.ModuleCheck(ctx, typ, name, nil)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, result)
		})
	})

	msfrpc.Kill()
	testsuite.IsDestroyed(t, msfrpc)
}
