package msfrpc

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

func TestClient_ModuleExploits(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := client.ModuleExploits(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		modules, err := client.ModuleExploits(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, modules)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			modules, err := client.ModuleExploits(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleAuxiliary(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := client.ModuleAuxiliary(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		modules, err := client.ModuleAuxiliary(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, modules)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			modules, err := client.ModuleAuxiliary(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModulePost(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := client.ModulePost(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		modules, err := client.ModulePost(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, modules)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			modules, err := client.ModulePost(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModulePayloads(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := client.ModulePayloads(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		modules, err := client.ModulePayloads(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, modules)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			modules, err := client.ModulePayloads(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleEncoders(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := client.ModuleEncoders(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		modules, err := client.ModuleEncoders(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, modules)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			modules, err := client.ModuleEncoders(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleNops(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := client.ModuleNops(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		modules, err := client.ModuleNops(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, modules)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			modules, err := client.ModuleNops(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleEvasion(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		modules, err := client.ModuleEvasion(ctx)
		require.NoError(t, err)

		for i := 0; i < len(modules); i++ {
			t.Log(modules[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		modules, err := client.ModuleEvasion(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, modules)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			modules, err := client.ModuleEvasion(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, modules)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleInfo(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		info, err := client.ModuleInfo(ctx, "exploit", "multi/handler")
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

	t.Run("get all module information", func(t *testing.T) {
		all := make(map[string][]string)

		for _, testdata := range [...]*struct {
			name string
			fn   func(context.Context) ([]string, error)
		}{
			{"exploit", client.ModuleExploits},
			{"auxiliary", client.ModuleAuxiliary},
			{"post", client.ModulePost},
			{"payload", client.ModulePayloads},
			{"encoder", client.ModuleEncoders},
			{"nop", client.ModuleNops},
			{"evasion", client.ModuleEvasion},
		} {
			modules, err := testdata.fn(ctx)
			require.NoError(t, err)
			all[testdata.name] = modules
		}

		// TODO [external] msfrpcd invalid modules
		// My platform is windows, so these maybe no error in other platforms

		// some module will failed, so we must skip these modules
		skipList := [...]*struct {
			typ  string
			name string
		}{
			{typ: "exploit", name: "linux/misc/saltstack_salt_unauth_rce"},
			{typ: "exploit", name: "linux/smtp/haraka"},
			{typ: "exploit", name: "windows/smb/ms17_010_eternalblue_win8"},

			{typ: "auxiliary", name: "admin/http/grafana_auth_bypass"},
			{typ: "auxiliary", name: "admin/teradata/teradata_odbc_sql"},
			{typ: "auxiliary", name: "dos/http/cable_haunt_websocket_dos"},
			{typ: "auxiliary", name: "dos/http/slowloris"},
			{typ: "auxiliary", name: "dos/smb/smb_loris"},
			{typ: "auxiliary", name: "dos/tcp/claymore_dos"},
			{typ: "auxiliary", name: "gather/chrome_debugger"},
			{typ: "auxiliary", name: "gather/get_user_spns"},
			{typ: "auxiliary", name: "scanner/http/onion_omega2_login"},
			{typ: "auxiliary", name: "scanner/msmail/exchange_enum"},
			{typ: "auxiliary", name: "scanner/msmail/host_id"},
			{typ: "auxiliary", name: "scanner/msmail/onprem_enum"},
			{typ: "auxiliary", name: "scanner/smb/impacket/dcomexec"},
			{typ: "auxiliary", name: "scanner/smb/impacket/secretsdump"},
			{typ: "auxiliary", name: "scanner/smb/impacket/wmiexec"},
			{typ: "auxiliary", name: "scanner/ssl/bleichenbacher_oracle"},
			{typ: "auxiliary", name: "scanner/teradata/teradata_odbc_login"},
			{typ: "auxiliary", name: "scanner/wproxy/att_open_proxy"},
		}

		// include new skip list, add it to skipList if with error
		// invalid module: foo/bar/module
		var invalid []struct {
			typ  string
			name string
		}

		// try to get module information
		testModules := func(typ string, modules []string) {
			for i := 0; i < len(modules); i++ {
				skip := false
				for j := 0; j < len(skipList); j++ {
					if typ == skipList[j].typ && modules[i] == skipList[j].name {
						skip = true
						break
					}
				}
				if skip {
					continue
				}
				_, err := client.ModuleInfo(ctx, typ, modules[i])
				if err != nil {
					fmt.Printf("%s %s %s\n", typ, modules[i], err)
					invalid = append(invalid, struct {
						typ  string
						name string
					}{
						typ:  typ,
						name: modules[i],
					})
				}
			}
		}

		for typ, modules := range all {
			t.Run(fmt.Sprintf("test %s modules", typ), func(_ *testing.T) {
				testModules(typ, modules)
			})
		}

		// print module with error, maybe add it to the skip list
		const format = `{typ: "%s", name: "%s"},` + "\n"
		for i := 0; i < len(invalid); i++ {
			fmt.Printf(format, invalid[i].typ, invalid[i].name)
		}

		require.Empty(t, invalid)
	})

	t.Run("invalid module", func(t *testing.T) {
		info, err := client.ModuleInfo(ctx, "foo type", "bar name")
		require.EqualError(t, err, "invalid module: foo type/bar name")
		require.Nil(t, info)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		info, err := client.ModuleInfo(ctx, "foo", "bar")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, info)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			info, err := client.ModuleInfo(ctx, "foo", "bar")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, info)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleOptions(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		options, err := client.ModuleOptions(ctx, "exploit", "multi/handler")
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

	t.Run("invalid module", func(t *testing.T) {
		options, err := client.ModuleOptions(ctx, "foo type", "bar name")
		require.EqualError(t, err, "invalid module: foo type/bar name")
		require.Nil(t, options)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		options, err := client.ModuleOptions(ctx, "foo", "bar")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, options)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			options, err := client.ModuleOptions(ctx, "foo", "bar")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, options)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleCompatiblePayloads(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		payloads, err := client.ModuleCompatiblePayloads(ctx, "exploit/multi/handler")
		require.NoError(t, err)
		for i := 0; i < len(payloads); i++ {
			t.Log(payloads[i])
		}
	})

	t.Run("invalid module", func(t *testing.T) {
		payloads, err := client.ModuleCompatiblePayloads(ctx, "foo")
		require.EqualError(t, err, "invalid module: exploit/foo")
		require.Nil(t, payloads)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		payloads, err := client.ModuleCompatiblePayloads(ctx, "foo")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, payloads)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			payloads, err := client.ModuleCompatiblePayloads(ctx, "foo")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, payloads)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleTargetCompatiblePayloads(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	const (
		module        = "exploit/multi/handler"
		target        = 0
		invalidModule = "foo"
	)

	t.Run("success", func(t *testing.T) {
		payloads, err := client.ModuleTargetCompatiblePayloads(ctx, module, target)
		require.NoError(t, err)
		for i := 0; i < len(payloads); i++ {
			t.Log(payloads[i])
		}
	})

	t.Run("invalid module", func(t *testing.T) {
		payloads, err := client.ModuleTargetCompatiblePayloads(ctx, invalidModule, target)
		require.EqualError(t, err, "invalid module: exploit/foo")
		require.Nil(t, payloads)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		payloads, err := client.ModuleTargetCompatiblePayloads(ctx, invalidModule, target)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, payloads)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			payloads, err := client.ModuleTargetCompatiblePayloads(ctx, invalidModule, target)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, payloads)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleCompatibleSessions(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()
	const module = "post/windows/gather/enum_proxy"

	t.Run("success", func(t *testing.T) {
		sessions, err := client.ModuleCompatibleSessions(ctx, module)
		require.NoError(t, err)
		// now is noting
		for i := 0; i < len(sessions); i++ {
			t.Log(sessions[i])
		}
	})

	t.Run("invalid module", func(t *testing.T) {
		sessions, err := client.ModuleCompatibleSessions(ctx, "foo")
		require.EqualError(t, err, "invalid module: post/foo")
		require.Nil(t, sessions)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		sessions, err := client.ModuleCompatibleSessions(ctx, "foo")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, sessions)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			sessions, err := client.ModuleCompatibleSessions(ctx, "foo")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, sessions)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleCompatibleEvasionPayloads(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()
	const module = "windows/windows_defender_exe"

	t.Run("success", func(t *testing.T) {
		payloads, err := client.ModuleCompatibleEvasionPayloads(ctx, module)
		require.NoError(t, err)
		for i := 0; i < len(payloads); i++ {
			t.Log(payloads[i])
		}
	})

	t.Run("invalid module", func(t *testing.T) {
		payloads, err := client.ModuleCompatibleEvasionPayloads(ctx, "foo")
		require.EqualError(t, err, "invalid module: evasion/foo")
		require.Nil(t, payloads)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		payloads, err := client.ModuleCompatibleEvasionPayloads(ctx, "foo")
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, payloads)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			payloads, err := client.ModuleCompatibleEvasionPayloads(ctx, "foo")
			monkey.IsMonkeyError(t, err)
			require.Nil(t, payloads)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleTargetCompatibleEvasionPayloads(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	const (
		module        = "windows/windows_defender_exe"
		target        = 0
		invalidModule = "foo"
	)

	t.Run("success", func(t *testing.T) {
		payloads, err := client.ModuleTargetCompatibleEvasionPayloads(ctx, module, target)
		require.NoError(t, err)
		for i := 0; i < len(payloads); i++ {
			t.Log(payloads[i])
		}
	})

	t.Run("invalid module", func(t *testing.T) {
		payloads, err := client.ModuleTargetCompatibleEvasionPayloads(ctx, invalidModule, target)
		require.EqualError(t, err, "invalid module: evasion/foo")
		require.Nil(t, payloads)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		payloads, err := client.ModuleTargetCompatibleEvasionPayloads(ctx, invalidModule, target)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, payloads)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			payloads, err := client.ModuleTargetCompatibleEvasionPayloads(ctx, invalidModule, target)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, payloads)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleEncodeFormats(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		formats, err := client.ModuleEncodeFormats(ctx)
		require.NoError(t, err)
		for i := 0; i < len(formats); i++ {
			t.Log(formats[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		formats, err := client.ModuleEncodeFormats(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, formats)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			formats, err := client.ModuleEncodeFormats(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, formats)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleExecutableFormats(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		formats, err := client.ModuleExecutableFormats(ctx)
		require.NoError(t, err)
		for i := 0; i < len(formats); i++ {
			t.Log(formats[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		formats, err := client.ModuleExecutableFormats(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, formats)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			formats, err := client.ModuleExecutableFormats(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, formats)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleTransformFormats(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		formats, err := client.ModuleTransformFormats(ctx)
		require.NoError(t, err)
		for i := 0; i < len(formats); i++ {
			t.Log(formats[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		formats, err := client.ModuleTransformFormats(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, formats)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			formats, err := client.ModuleTransformFormats(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, formats)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleEncryptionFormats(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		formats, err := client.ModuleEncryptionFormats(ctx)
		require.NoError(t, err)
		for i := 0; i < len(formats); i++ {
			t.Log(formats[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		formats, err := client.ModuleEncryptionFormats(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, formats)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			formats, err := client.ModuleEncryptionFormats(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, formats)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModulePlatforms(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		platforms, err := client.ModulePlatforms(ctx)
		require.NoError(t, err)
		for i := 0; i < len(platforms); i++ {
			t.Log(platforms[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		platforms, err := client.ModulePlatforms(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, platforms)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			platforms, err := client.ModulePlatforms(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, platforms)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleArchitectures(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		architectures, err := client.ModuleArchitectures(ctx)
		require.NoError(t, err)
		for i := 0; i < len(architectures); i++ {
			t.Log(architectures[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		architectures, err := client.ModuleArchitectures(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, architectures)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			architectures, err := client.ModuleArchitectures(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, architectures)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleEncode(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
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
		encoded, err := client.ModuleEncode(ctx, data, encoder, opts)
		require.NoError(t, err)
		t.Logf("\n%s\n", encoded)
	})

	t.Run("no data", func(t *testing.T) {
		encoded, err := client.ModuleEncode(ctx, "", encoder, opts)
		require.EqualError(t, err, "no data")
		require.Zero(t, encoded)
	})

	t.Run("invalid format", func(t *testing.T) {
		opts.Format = "foo"
		defer func() { opts.Format = "c" }()

		encoded, err := client.ModuleEncode(ctx, data, encoder, opts)
		require.EqualError(t, err, "invalid format: foo")
		require.Zero(t, encoded)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		encoded, err := client.ModuleEncode(ctx, data, encoder, opts)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Zero(t, encoded)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			encoded, err := client.ModuleEncode(ctx, data, encoder, opts)
			monkey.IsMonkeyError(t, err)
			require.Zero(t, encoded)
		})
	})

	err = client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleExecute(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("exploit", func(t *testing.T) {
		const exploit = "multi/handler"
		opts := make(map[string]interface{})
		opts["PAYLOAD"] = "windows/meterpreter/reverse_tcp"
		opts["TARGET"] = 0
		opts["LHOST"] = "127.0.0.1"
		opts["LPORT"] = "0"

		t.Run("success", func(t *testing.T) {
			result, err := client.ModuleExecute(ctx, "exploit", exploit, opts)
			require.NoError(t, err)

			jobID := strconv.FormatUint(result.JobID, 10)
			info, err := client.JobInfo(ctx, jobID)
			require.NoError(t, err)
			t.Log(info.Name)
			for key, value := range info.DataStore {
				t.Log(key, value)
			}
			err = client.JobStop(ctx, jobID)
			require.NoError(t, err)
		})

		t.Run("invalid port", func(t *testing.T) {
			opts["LPORT"] = "foo"
			defer func() { opts["LPORT"] = "0" }()
			result, err := client.ModuleExecute(ctx, "exploit", exploit, opts)
			require.NoError(t, err)

			jobID := strconv.FormatUint(result.JobID, 10)
			info, err := client.JobInfo(ctx, jobID)
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
			result, err := client.ModuleExecute(ctx, "payload", payload, opts)
			require.NoError(t, err)
			t.Log(result.Payload)
		})

		t.Run("invalid port", func(t *testing.T) {
			const errStr = "failed to generate: One or more options failed to validate: LPORT."
			opts.DataStore["LPORT"] = "foo"
			defer func() { opts.DataStore["LPORT"] = "1999" }()
			result, err := client.ModuleExecute(ctx, "payload", payload, opts)
			require.EqualError(t, err, errStr)
			require.Nil(t, result)
		})
	})

	t.Run("invalid module type", func(t *testing.T) {
		result, err := client.ModuleExecute(ctx, "foo", "bar", nil)
		require.EqualError(t, err, "invalid module type: foo")
		require.Nil(t, result)
	})

	const (
		typ  = "exploit"
		name = "foo"
	)
	opts := make(map[string]interface{})

	t.Run("invalid module", func(t *testing.T) {
		result, err := client.ModuleExecute(ctx, typ, name, opts)
		require.EqualError(t, err, "invalid module: exploit/foo")
		require.Nil(t, result)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		result, err := client.ModuleExecute(ctx, typ, name, opts)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, result)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			result, err := client.ModuleExecute(ctx, typ, name, opts)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, result)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleCheck(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
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

		result, err := client.ModuleCheck(ctx, "exploit", exploit, opts)
		require.NoError(t, err)

		jobID := strconv.FormatUint(result.JobID, 10)
		info, err := client.JobInfo(ctx, jobID)
		require.NoError(t, err)
		t.Log(info.Name)
		for key, value := range info.DataStore {
			t.Log(key, value)
		}
		err = client.JobStop(ctx, jobID)
		require.NoError(t, err)
	})

	t.Run("invalid module type", func(t *testing.T) {
		result, err := client.ModuleCheck(ctx, "foo", "bar", nil)
		require.EqualError(t, err, "invalid module type: foo")
		require.Nil(t, result)
	})

	const (
		typ  = "exploit"
		name = "foo"
	)

	t.Run("invalid module", func(t *testing.T) {
		result, err := client.ModuleCheck(ctx, typ, name, nil)
		require.EqualError(t, err, "invalid module: exploit/foo")
		require.Nil(t, result)
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		result, err := client.ModuleCheck(ctx, typ, name, nil)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, result)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			result, err := client.ModuleCheck(ctx, typ, name, nil)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, result)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}

func TestClient_ModuleRunningStats(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	client := testGenerateClientAndLogin(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		result, err := client.ModuleRunningStats(ctx)
		require.NoError(t, err)

		for i := 0; i < len(result.Waiting); i++ {
			t.Log("waiting", result.Waiting[i])
		}
		for i := 0; i < len(result.Running); i++ {
			t.Log("running", result.Running[i])
		}
		for i := 0; i < len(result.Results); i++ {
			t.Log("results", result.Results[i])
		}
	})

	t.Run("invalid authentication token", func(t *testing.T) {
		token := client.GetToken()
		defer client.SetToken(token)
		client.SetToken(testInvalidToken)

		result, err := client.ModuleRunningStats(ctx)
		require.EqualError(t, err, ErrInvalidTokenFriendly)
		require.Nil(t, result)
	})

	t.Run("failed to send", func(t *testing.T) {
		testPatchClientSend(func() {
			result, err := client.ModuleRunningStats(ctx)
			monkey.IsMonkeyError(t, err)
			require.Nil(t, result)
		})
	})

	err := client.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, client)
}
