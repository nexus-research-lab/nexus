//go:build linux

package runtimeidentity

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

var managementCommands = map[string]struct{}{
	"ensure-host":    {},
	"ensure-user":    {},
	"repair-user":    {},
	"prepare":        {},
	"project-ensure": {},
	"project-grant":  {},
	"project-list":   {},
	"signal-process": {},
	"stop-user":      {},
}

// Run 执行 root-owned launcher。返回值可直接交给 os.Exit。
func Run(args []string, environ []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-v" || args[0] == "--version" || args[0] == "version") {
		_, _ = fmt.Fprintln(stdout, launcherVersion)
		return 0
	}
	config, err := loadLauncherConfig()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if err = verifyTrustedCaller(config); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if len(args) == 0 {
		return runRuntime(config, environ, nil, stderr)
	}
	if _, management := managementCommands[args[0]]; management {
		if err = promoteManagementRootIdentity(); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
	}
	switch args[0] {
	case "ensure-host":
		err = runEnsureHost(config, args[1:], stdout)
	case "ensure-user":
		err = runEnsureUser(config, args[1:], stdout)
	case "repair-user":
		err = runRepairUser(config, args[1:], stdout)
	case "prepare":
		err = runPrepare(config, args[1:], stdout)
	case "project-ensure":
		err = runProjectEnsure(config, args[1:], stdout)
	case "project-grant":
		err = runProjectGrant(config, args[1:], stdout)
	case "project-list":
		err = runProjectList(config, args[1:], stdout)
	case "signal-process":
		err = runSignalProcess(config, args[1:], stdout)
	case "stop-user":
		err = runStopUser(config, args[1:], stdout)
	case "run-script":
		err = runScript(config, environ, args[1:], stdout, stderr)
	default:
		// bridge 直接把 runtime flags 交给 launcher；只有存在有效票据时
		// 才把整组参数视为 runtime argv。
		err = runRuntimeError(config, environ, args, stderr)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// promoteManagementRootIdentity 在受信任 host 调用方校验完成后，把 setuid
// launcher 的 real/fs UID/GID 一并提升为 root。管理命令需要遍历历史私有
// 目录；只保留 effective uid 会让部分容器内核按调用方 fsuid 执行 DAC 检查。
// runtime argv 不走此路径，仍在 exec 前直接降到 owner UID/GID 并进入 Landlock。
func promoteManagementRootIdentity() error {
	if err := syscall.Setgroups([]int{0}); err != nil {
		return fmt.Errorf("收紧 launcher management supplementary groups: %w", err)
	}
	if err := syscall.Setresgid(0, 0, 0); err != nil {
		return fmt.Errorf("提升 launcher management gid: %w", err)
	}
	if err := syscall.Setresuid(0, 0, 0); err != nil {
		return fmt.Errorf("提升 launcher management uid: %w", err)
	}
	if err := unix.Setfsgid(0); err != nil {
		return fmt.Errorf("提升 launcher management fsgid: %w", err)
	}
	if err := unix.Setfsuid(0); err != nil {
		return fmt.Errorf("提升 launcher management fsuid: %w", err)
	}
	if os.Getuid() != 0 || os.Geteuid() != 0 ||
		os.Getgid() != 0 || os.Getegid() != 0 {
		return errors.New("launcher management root 身份提升后校验失败")
	}
	return nil
}

func runEnsureHost(config launcherConfig, args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("ensure-host 不接受参数")
	}
	if !config.LandlockRequired {
		return errors.New("enforce launcher 配置必须启用 Landlock")
	}
	abi, err := landlockABI()
	if err != nil {
		return fmt.Errorf("查询 Landlock ABI: %w", err)
	}
	if abi < 3 {
		return fmt.Errorf("Landlock ABI %d 不足，enforce 至少需要 ABI 3", abi)
	}
	runtimeKinds := make([]string, 0, len(config.RuntimeExecutables))
	for runtimeKind := range config.RuntimeExecutables {
		runtimeKinds = append(runtimeKinds, runtimeKind)
	}
	slices.Sort(runtimeKinds)
	for _, runtimeKind := range runtimeKinds {
		if _, err = trustedRuntimeExecutable(config, runtimeKind); err != nil {
			return fmt.Errorf("校验 %s runtime executable: %w", runtimeKind, err)
		}
	}
	if err = ensureCgroupHost(config); err != nil {
		return fmt.Errorf("校验 cgroup v2: %w", err)
	}
	if err = validateRuntimeIsolationHardLinks(config.StateRoot); err != nil {
		return err
	}
	if err := ensureHostLayout(config); err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{
		"landlock_abi":   abi,
		"ready":          true,
		"state_root":     config.StateRoot,
		"cgroup_enabled": cgroupEnabled(config),
	})
}

func runEnsureUser(config launcherConfig, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("ensure-user", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	ownerUserID := flags.String("owner", "", "owner user id")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("ensure-user 参数无效")
	}
	value, err := withLockedRegistry(config, true, func(current *registry) (preparedIdentity, bool, error) {
		identityValue, changed, ensureErr := ensureIdentity(config, current, *ownerUserID)
		if ensureErr != nil {
			return preparedIdentity{}, false, ensureErr
		}
		if _, ensureErr = ensureRuntimeCgroup(config, identityValue.Username); ensureErr != nil {
			return preparedIdentity{}, false, ensureErr
		}
		return preparedIdentity{
			Username:   identityValue.Username,
			UID:        identityValue.UID,
			PrivateGID: identityValue.PrivateGID,
			HomeDir:    identityValue.HomeDir,
			TempDir:    identityValue.TempDir,
		}, changed, nil
	})
	if err != nil {
		return err
	}
	return writeJSON(stdout, value)
}

func runRepairUser(config launcherConfig, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("repair-user", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	ownerUserID := flags.String("owner", "", "owner user id")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("repair-user 参数无效")
	}
	value, err := withLockedRegistry(config, true, func(current *registry) (map[string]any, bool, error) {
		identityValue := current.Identities[strings.TrimSpace(*ownerUserID)]
		if identityValue == nil || identityValue.Status != "active" {
			return nil, false, errors.New("runtime identity 不存在或已停用")
		}
		if err := ensureOSAccount(*identityValue); err != nil {
			return nil, false, err
		}
		if err := repairRuntimeACL(config, identityValue); err != nil {
			return nil, false, err
		}
		return map[string]any{
			"owner_user_id": identityValue.OwnerUserID,
			"repaired":      true,
		}, false, nil
	})
	if err != nil {
		return err
	}
	return writeJSON(stdout, value)
}

func runPrepare(config launcherConfig, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("prepare", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	ownerUserID := flags.String("owner", "", "owner user id")
	runtimeKind := flags.String("runtime", "", "runtime kind")
	cwd := flags.String("cwd", "", "runtime cwd")
	var readRoots stringListFlag
	var environmentNames stringListFlag
	flags.Var(&readRoots, "read-root", "additional read root")
	flags.Var(&environmentNames, "env", "explicit runtime environment name")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("prepare 参数无效")
	}
	policy, err := withLockedRegistry(config, true, func(current *registry) (preparedPolicy, bool, error) {
		return preparePolicy(
			config,
			current,
			*ownerUserID,
			*runtimeKind,
			*cwd,
			readRoots,
			environmentNames,
		)
	})
	if err != nil {
		return err
	}
	return writeJSON(stdout, policy)
}

func runProjectEnsure(config launcherConfig, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("project-ensure", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectID := flags.String("project", "", "project id")
	root := flags.String("path", "", "project root")
	ownerUserID := flags.String("owner", "", "initial project owner")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("project-ensure 参数无效")
	}
	type ensureResponse struct {
		Project *project `json:"project"`
		Created bool     `json:"created"`
	}
	response, err := withLockedRegistry(config, true, func(current *registry) (ensureResponse, bool, error) {
		value, created, ensureErr := ensureProject(config, current, *projectID, *root)
		if ensureErr != nil {
			return ensureResponse{}, false, ensureErr
		}
		changed := created
		if created && strings.TrimSpace(*ownerUserID) != "" {
			grantChanged, grantErr := grantProject(
				config,
				current,
				*projectID,
				*ownerUserID,
				projectAccessWrite,
			)
			if grantErr != nil {
				return ensureResponse{}, false, grantErr
			}
			changed = changed || grantChanged
		}
		return ensureResponse{Project: value, Created: created}, changed, nil
	})
	if err != nil {
		return err
	}
	return writeJSON(stdout, response)
}

func runProjectGrant(config launcherConfig, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("project-grant", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectID := flags.String("project", "", "project id")
	ownerUserID := flags.String("owner", "", "owner user id")
	access := flags.String("access", "", "read, write or none")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("project-grant 参数无效")
	}
	changed, err := withLockedRegistry(config, true, func(current *registry) (bool, bool, error) {
		mutated, grantErr := grantProject(config, current, *projectID, *ownerUserID, *access)
		return mutated, mutated, grantErr
	})
	if err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{"changed": changed})
}

func runProjectList(config launcherConfig, args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("project-list 不接受参数")
	}
	projects, err := withLockedRegistry(config, false, func(current *registry) ([]*project, bool, error) {
		result := make([]*project, 0, len(current.Projects))
		for _, value := range current.Projects {
			result = append(result, value)
		}
		slices.SortFunc(result, func(left *project, right *project) int {
			return strings.Compare(left.ProjectID, right.ProjectID)
		})
		return result, false, nil
	})
	if err != nil {
		return err
	}
	return writeJSON(stdout, projects)
}

func runStopUser(config launcherConfig, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("stop-user", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	ownerUserID := flags.String("owner", "", "owner user id")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("stop-user 参数无效")
	}
	identityValue, err := withLockedRegistry(config, false, func(current *registry) (*identity, bool, error) {
		value := current.Identities[strings.TrimSpace(*ownerUserID)]
		if value == nil {
			return nil, false, nil
		}
		return value, false, nil
	})
	if err != nil {
		return err
	}
	if identityValue != nil {
		if err = killRuntimeCgroup(config, identityValue.Username); err != nil {
			return err
		}
	}
	return writeJSON(stdout, map[string]any{
		"owner_user_id": strings.TrimSpace(*ownerUserID),
		"stopped":       identityValue != nil,
	})
}

func runSignalProcess(config launcherConfig, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("signal-process", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	ownerUserID := flags.String("owner", "", "owner user id")
	processID := flags.Int("pid", 0, "runtime process id")
	signalName := flags.String("signal", "", "interrupt, terminate or kill")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("signal-process 参数无效")
	}
	value, err := withLockedRegistry(config, false, func(current *registry) (*identity, bool, error) {
		identityValue := current.Identities[strings.TrimSpace(*ownerUserID)]
		if identityValue == nil || identityValue.Status != "active" {
			return nil, false, errors.New("runtime identity 不存在或已停用")
		}
		return identityValue, false, nil
	})
	if err != nil {
		return err
	}
	processSignal, sessionWide, err := parseRuntimeProcessSignal(*signalName)
	if err != nil {
		return err
	}
	if err = signalRuntimeSession(config, value, *processID, processSignal, sessionWide); err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{
		"owner_user_id": strings.TrimSpace(*ownerUserID),
		"pid":           *processID,
		"signal":        strings.TrimSpace(*signalName),
	})
}

func parseRuntimeProcessSignal(value string) (syscall.Signal, bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "interrupt":
		return syscall.SIGINT, false, nil
	case "terminate":
		return syscall.SIGTERM, false, nil
	case "kill":
		return syscall.SIGKILL, true, nil
	default:
		return 0, false, errors.New("runtime process signal 无效")
	}
}

func runRuntimeError(
	config launcherConfig,
	environ []string,
	args []string,
	stderr io.Writer,
) error {
	code := runRuntime(config, environ, args, stderr)
	if code == 0 {
		return nil
	}
	return fmt.Errorf("runtime launcher 退出，code=%d", code)
}

func runScript(
	config launcherConfig,
	environ []string,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) error {
	script, err := validateScriptArguments(args)
	if err != nil {
		return err
	}

	// Landlock policy 绑定调用线程；子进程必须从已进入 domain 的线程 fork。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ticket, policy, err := loadLaunchPolicy(config, environ)
	if err != nil {
		return err
	}
	if policy.RuntimeKind != scriptRuntimeKind {
		return errors.New("run-script 需要 script runtime ticket")
	}
	if err = attachRuntimeCgroup(config, policy.Identity.Username); err != nil {
		return fmt.Errorf("加入 runtime cgroup: %w", err)
	}
	policy, err = refreshLaunchPolicy(config, ticket)
	if err != nil {
		return fmt.Errorf("复核 script isolation ticket: %w", err)
	}
	if err = enterLaunchCWD(policy, false); err != nil {
		return err
	}
	executable, err := trustedRuntimeExecutable(config, policy.RuntimeKind)
	if err != nil {
		return err
	}
	syscall.Umask(0o007)
	landlockABIValue, err := applyLandlock(config, policy, executable)
	if err != nil {
		return err
	}
	if err = markUnexpectedDescriptorsCloseOnExec(); err != nil {
		return fmt.Errorf("收紧 script 文件描述符继承失败: %w", err)
	}

	groups := make([]uint32, 0, len(policy.Identity.SupplementaryGIDs))
	for _, groupID := range policy.Identity.SupplementaryGIDs {
		groups = append(groups, uint32(groupID))
	}
	command := exec.Command(executable, "-lc", script)
	command.Stdout = stdout
	command.Stderr = stderr
	command.Env = sanitizedRuntimeEnvironment(environ, config, policy)
	command.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:    uint32(policy.Identity.UID),
			Gid:    uint32(policy.Identity.PrivateGID),
			Groups: groups,
		},
		Setpgid:   true,
		Pdeathsig: syscall.SIGKILL,
	}
	if err = command.Start(); err != nil {
		return fmt.Errorf("启动受限自动化脚本: %w", err)
	}
	_, _ = fmt.Fprintf(
		stderr,
		"nexus_script_isolation owner=%s uid=%d gid=%d groups=%v generation=%d landlock_abi=%d cwd=%s\n",
		policy.OwnerUserID,
		policy.Identity.UID,
		policy.Identity.PrivateGID,
		policy.Identity.SupplementaryGIDs,
		policy.Generation,
		landlockABIValue,
		policy.CWD,
	)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signalChannel)
	waitChannel := make(chan error, 1)
	go func() {
		waitChannel <- command.Wait()
	}()
	select {
	case waitErr := <-waitChannel:
		return waitErr
	case <-signalChannel:
		killErr := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		waitErr := <-waitChannel
		if killErr != nil && !errors.Is(killErr, syscall.ESRCH) {
			return errors.Join(errors.New("自动化脚本已中断"), killErr, waitErr)
		}
		return errors.Join(errors.New("自动化脚本已中断"), waitErr)
	}
}

func loadLaunchPolicy(
	config launcherConfig,
	environ []string,
) (launchTicket, preparedPolicy, error) {
	ticketID := environmentValue(environ, "NEXUS_RUNTIME_ISOLATION_TICKET")
	if ticketID == "" {
		return launchTicket{}, preparedPolicy{}, errors.New("缺少 runtime isolation ticket")
	}
	ticket, err := readLaunchTicket(config, ticketID)
	if err != nil {
		return launchTicket{}, preparedPolicy{}, err
	}
	if strings.TrimSpace(ticket.OwnerUserID) == "" ||
		strings.TrimSpace(ticket.RuntimeKind) == "" ||
		strings.TrimSpace(ticket.CWD) == "" ||
		ticket.Generation == 0 ||
		ticket.CreatedAt.IsZero() ||
		ticket.ExpiresAt.IsZero() ||
		ticket.ExpiresAt.Before(ticket.CreatedAt) {
		return launchTicket{}, preparedPolicy{}, errors.New("runtime isolation ticket 内容无效")
	}
	policy, err := refreshLaunchPolicy(config, ticket)
	if err != nil {
		return launchTicket{}, preparedPolicy{}, err
	}
	return ticket, policy, nil
}

func refreshLaunchPolicy(
	config launcherConfig,
	ticket launchTicket,
) (preparedPolicy, error) {
	policy, err := withLockedRegistry(config, false, func(current *registry) (preparedPolicy, bool, error) {
		value, policyErr := policyForTicket(config, current, ticket)
		return value, false, policyErr
	})
	if err != nil {
		return preparedPolicy{}, err
	}
	if policy.Generation != ticket.Generation {
		return preparedPolicy{}, errors.New("runtime isolation ticket 已被撤销")
	}
	return policy, nil
}

func enterLaunchCWD(policy preparedPolicy, requireCurrentCWD bool) error {
	if requireCurrentCWD {
		actualCWD, err := os.Getwd()
		if err != nil {
			return err
		}
		actualCWD, err = canonicalExistingOrPendingPath(actualCWD)
		if err != nil || actualCWD != policy.CWD {
			return errors.New("runtime cwd 与 ticket 不一致")
		}
	}
	cwdFD, err := openPathNoSymlink(policy.CWD, true)
	if err != nil {
		return fmt.Errorf("打开 runtime cwd: %w", err)
	}
	if err = unix.Fchdir(cwdFD); err != nil {
		_ = unix.Close(cwdFD)
		return fmt.Errorf("进入 runtime cwd: %w", err)
	}
	_ = unix.Close(cwdFD)
	return nil
}

func runRuntime(
	config launcherConfig,
	environ []string,
	args []string,
	stderr io.Writer,
) int {
	// Landlock policy 绑定调用线程。锁定当前 OS 线程，确保后续 exec 不会因
	// Go 调度迁移到尚未进入该 Landlock domain 的 runtime 线程。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ticket, policy, err := loadLaunchPolicy(config, environ)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if policy.RuntimeKind == scriptRuntimeKind {
		_, _ = fmt.Fprintln(stderr, "script ticket 必须使用 run-script")
		return 1
	}
	if err = attachRuntimeCgroup(config, policy.Identity.Username); err != nil {
		_, _ = fmt.Fprintln(stderr, fmt.Errorf("加入 runtime cgroup: %w", err))
		return 1
	}
	// stop-user 可能与上面的 attach 并发；重新读取 registry，确保 ACL
	// generation 在进入最终 Landlock/exec 前仍然有效。
	policy, err = refreshLaunchPolicy(config, ticket)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, fmt.Errorf("复核 runtime isolation ticket: %w", err))
		return 1
	}
	if err = enterLaunchCWD(policy, true); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	executable, err := trustedRuntimeExecutable(config, policy.RuntimeKind)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if err = validateRuntimeArgs(args); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if err = prepareRuntimeArgFiles(config, policy, args); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	syscall.Umask(0o007)
	landlockABIValue, err := applyLandlock(config, policy, executable)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if err = dropRuntimeIdentity(policy.Identity); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	environment := sanitizedRuntimeEnvironment(environ, config, policy)
	_, _ = fmt.Fprintf(
		stderr,
		"nexus_runtime_isolation owner=%s uid=%d gid=%d groups=%v generation=%d landlock_abi=%d cwd=%s\n",
		policy.OwnerUserID,
		policy.Identity.UID,
		policy.Identity.PrivateGID,
		policy.Identity.SupplementaryGIDs,
		policy.Generation,
		landlockABIValue,
		policy.CWD,
	)
	if err = markUnexpectedDescriptorsCloseOnExec(); err != nil {
		_, _ = fmt.Fprintln(stderr, fmt.Errorf("收紧 runtime 文件描述符继承失败: %w", err))
		return 1
	}
	argv := append([]string{executable}, args...)
	if err = syscall.Exec(executable, argv, environment); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func trustedRuntimeExecutable(config launcherConfig, runtimeKind string) (string, error) {
	configured := config.RuntimeExecutables[strings.ToLower(strings.TrimSpace(runtimeKind))]
	if configured == "" {
		return "", errors.New("runtime executable 未配置")
	}
	resolved, err := filepath.EvalSymlinks(configured)
	if err != nil {
		return "", fmt.Errorf("解析 runtime executable: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 ||
		info.Mode().Perm()&0o022 != 0 {
		return "", errors.New("runtime executable 权限不安全")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 {
		return "", errors.New("runtime executable 必须由 root 持有")
	}
	return resolved, nil
}

func validateRuntimeArgs(args []string) error {
	if len(args) < 5 ||
		args[0] != "--output-format" || args[1] != "stream-json" ||
		args[2] != "--verbose" ||
		args[3] != "--input-format" || args[4] != "stream-json" {
		return errors.New("runtime argv 缺少受控 stream-json 前缀")
	}
	for _, argument := range args {
		if strings.ContainsRune(argument, 0) {
			return errors.New("runtime argv 包含 NUL")
		}
		flagName := strings.ToLower(strings.TrimSpace(argument))
		if index := strings.IndexByte(flagName, '='); index >= 0 {
			flagName = flagName[:index]
		}
		switch flagName {
		case "--bare", "--disable-hooks", "--no-hooks":
			return fmt.Errorf("runtime argv 包含被禁止的安全旁路: %s", argument)
		}
	}
	return nil
}

func dropRuntimeIdentity(value preparedIdentity) error {
	groups := slices.Clone(value.SupplementaryGIDs)
	slices.Sort(groups)
	groups = slices.Compact(groups)
	if err := syscall.Setgroups(groups); err != nil {
		return fmt.Errorf("设置 runtime supplementary groups: %w", err)
	}
	if err := syscall.Setresgid(value.PrivateGID, value.PrivateGID, value.PrivateGID); err != nil {
		return fmt.Errorf("切换 runtime gid: %w", err)
	}
	if err := syscall.Setresuid(value.UID, value.UID, value.UID); err != nil {
		return fmt.Errorf("切换 runtime uid: %w", err)
	}
	if os.Getuid() != value.UID || os.Geteuid() != value.UID ||
		os.Getgid() != value.PrivateGID || os.Getegid() != value.PrivateGID {
		return errors.New("runtime UID/GID 切换后校验失败")
	}
	actualGroups, err := os.Getgroups()
	if err != nil {
		return fmt.Errorf("读取 runtime supplementary groups: %w", err)
	}
	slices.Sort(actualGroups)
	actualGroups = slices.Compact(actualGroups)
	if !slices.Equal(actualGroups, groups) {
		return fmt.Errorf(
			"runtime supplementary groups 切换后校验失败: actual=%v expected=%v",
			actualGroups,
			groups,
		)
	}
	return nil
}

func markUnexpectedDescriptorsCloseOnExec() error {
	if err := unix.CloseRange(3, ^uint(0), unix.CLOSE_RANGE_CLOEXEC); err == nil {
		return nil
	}
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return fmt.Errorf("读取 /proc/self/fd: %w", err)
	}
	for _, entry := range entries {
		fd, parseErr := strconv.Atoi(entry.Name())
		if parseErr != nil || fd < 3 {
			continue
		}
		if _, err = unix.FcntlInt(uintptr(fd), unix.F_SETFD, unix.FD_CLOEXEC); err != nil &&
			!errors.Is(err, unix.EBADF) {
			return fmt.Errorf("设置 fd=%d close-on-exec: %w", fd, err)
		}
	}
	return nil
}

func environmentValue(environ []string, name string) string {
	for index := len(environ) - 1; index >= 0; index-- {
		key, value, ok := strings.Cut(environ[index], "=")
		if ok && key == name {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

type stringListFlag []string

func (values *stringListFlag) String() string {
	return strings.Join(*values, ",")
}

func (values *stringListFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}
