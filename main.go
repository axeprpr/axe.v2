package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	configFile     = ".axe.v2.config.json"
	defaultTimeout = 15 * time.Second
	version        = "v0.4.0"
)

var debug = os.Getenv("axe_debug") != ""

type sshInfo struct {
	Address  string
	Port     string
	Username string
	Password string
}

type flexString string

func (s *flexString) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		*s = ""
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*s = flexString(str)
		return nil
	}
	var num json.Number
	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&num); err == nil {
		*s = flexString(num.String())
		return nil
	}
	return fmt.Errorf("unsupported string value: %s", trimmed)
}

func (s flexString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

func (s flexString) String() string {
	return string(s)
}

type tagEntry struct {
	Tag      flexString `json:"tag"`
	Address  flexString `json:"address"`
	Port     flexString `json:"port"`
	Username flexString `json:"username"`
	Password flexString `json:"password"`
}

type passwordEntry struct {
	Default  bool       `json:"default"`
	Port     flexString `json:"port"`
	Username flexString `json:"username"`
	Password flexString `json:"password"`
}

type config struct {
	Tags      []tagEntry      `json:"tags"`
	Passwords []passwordEntry `json:"passwords"`
}

type runOptions struct {
	Parallel int
	Timeout  time.Duration
	Retries  int
	DryRun   bool
	Verbose  bool
	JSON     bool
}

type opResult struct {
	Tag      string `json:"tag"`
	Address  string `json:"address"`
	Action   string `json:"action"`
	Attempts int    `json:"attempts"`
	Success  bool   `json:"success"`
	Duration string `json:"duration"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
}

func help() {
	fmt.Println("axe - lightweight multi-host SSH tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  axe [global flags] <tag>")
	fmt.Println("  axe [global flags] <tag1> <tag2> -c \"command\"")
	fmt.Println("  axe [global flags] <tag1> <tag2> -s <source> <dest>")
	fmt.Println("  axe tag list")
	fmt.Println("  axe tag add <tag> <address> [--port 22 --user root --password xxx]")
	fmt.Println("  axe tag edit <tag> [--address host --port 22 --user root --password xxx]")
	fmt.Println("  axe tag del <tag>")
	fmt.Println("  axe default show")
	fmt.Println("  axe default set [--port 22 --user root --password xxx]")
	fmt.Println("  axe version")
	fmt.Println()
	fmt.Println("Global flags:")
	fmt.Println("  --parallel N      最大并发数 (默认 5)")
	fmt.Println("  --timeout 20s     单机超时 (默认 15s)")
	fmt.Println("  --retries N       失败重试次数 (默认 0)")
	fmt.Println("  --dry-run         仅打印计划，不执行")
	fmt.Println("  --verbose         打印重试细节")
	fmt.Println("  --json            结果输出 JSON")
	fmt.Println()
	fmt.Println("Legacy:")
	fmt.Println("  axe -e | e | -l | l | -lp | lp    打开配置文件编辑")
}

func now() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func contains(item string, list []string) bool {
	for _, v := range list {
		if item == v {
			return true
		}
	}
	return false
}

func indexOf(item string, list []string) int {
	for k, v := range list {
		if item == v {
			return k
		}
	}
	return -1
}

func configTemplate() []byte {
	return []byte(`{
  "passwords": [
    {
      "default": true,
      "port": "22",
      "username": "root",
      "password": ""
    }
  ],
  "tags": [
    {
      "tag": "demo",
      "address": "127.0.0.1",
      "port": "22",
      "username": "root",
      "password": ""
    }
  ]
}
`)
}

func ensureConfigFile(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, configTemplate(), 0o644)
}

func openEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "vi"
		}
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func editConfig() int {
	if err := ensureConfigFile(configFile); err != nil {
		fmt.Fprintf(os.Stderr, "failed to prepare config: %v\n", err)
		return 1
	}
	if err := openEditor(configFile); err != nil {
		fmt.Fprintf(os.Stderr, "failed to open editor: %v\n", err)
		return 1
	}
	return 0
}

func loadConfig(path string) (config, error) {
	var cfg config
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if len(content) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(content, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func saveConfig(path string, cfg config) error {
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0o644)
}

func defaultPassword(cfg config) *passwordEntry {
	for i := range cfg.Passwords {
		if cfg.Passwords[i].Default {
			return &cfg.Passwords[i]
		}
	}
	if len(cfg.Passwords) > 0 {
		return &cfg.Passwords[0]
	}
	return nil
}

func sshConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".ssh", "config")
}

func loadSSHConfigAliases() map[string]sshInfo {
	result := make(map[string]sshInfo)
	path := sshConfigPath()
	if path == "" {
		return result
	}

	f, err := os.Open(path)
	if err != nil {
		return result
	}
	defer f.Close()

	currentHosts := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.ToLower(fields[0])
		value := strings.Join(fields[1:], " ")

		switch key {
		case "host":
			currentHosts = currentHosts[:0]
			for _, h := range fields[1:] {
				if strings.ContainsAny(h, "*?!") {
					continue
				}
				currentHosts = append(currentHosts, h)
				if _, ok := result[h]; !ok {
					result[h] = sshInfo{Address: h}
				}
			}
		case "hostname", "user", "port":
			for _, h := range currentHosts {
				entry := result[h]
				switch key {
				case "hostname":
					entry.Address = value
				case "user":
					entry.Username = value
				case "port":
					entry.Port = value
				}
				result[h] = entry
			}
		}
	}
	return result
}

func resolveSSHInfo(cfg config, sshAliases map[string]sshInfo, tag string) sshInfo {
	info := sshInfo{Address: tag, Port: "22", Username: "root"}

	if d := defaultPassword(cfg); d != nil {
		if d.Port.String() != "" {
			info.Port = d.Port.String()
		}
		if d.Username.String() != "" {
			info.Username = d.Username.String()
		}
		info.Password = d.Password.String()
	}

	if alias, ok := sshAliases[tag]; ok {
		if alias.Address != "" {
			info.Address = alias.Address
		}
		if alias.Port != "" {
			info.Port = alias.Port
		}
		if alias.Username != "" {
			info.Username = alias.Username
		}
	}

	for _, t := range cfg.Tags {
		if t.Tag.String() != tag {
			continue
		}
		if t.Address.String() != "" {
			info.Address = t.Address.String()
		}
		if t.Port.String() != "" {
			info.Port = t.Port.String()
		}
		if t.Username.String() != "" {
			info.Username = t.Username.String()
		}
		if t.Password.String() != "" {
			info.Password = t.Password.String()
		}
		break
	}

	if v := os.Getenv("axe_port"); v != "" {
		info.Port = v
	}
	if v := os.Getenv("axe_username"); v != "" {
		info.Username = v
	}
	if v := os.Getenv("axe_password"); v != "" {
		info.Password = v
	}

	return info
}

func parseGlobalOptions(args []string) (runOptions, []string, error) {
	opts := runOptions{
		Parallel: 5,
		Timeout:  defaultTimeout,
		Retries:  0,
	}
	rest := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--dry-run":
			opts.DryRun = true
		case "--verbose":
			opts.Verbose = true
		case "--json":
			opts.JSON = true
		case "--parallel", "--timeout", "--retries":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("missing value for %s", arg)
			}
			val := args[i+1]
			i++
			switch arg {
			case "--parallel":
				n, err := strconv.Atoi(val)
				if err != nil || n <= 0 {
					return opts, nil, fmt.Errorf("invalid --parallel: %s", val)
				}
				opts.Parallel = n
			case "--timeout":
				d, err := time.ParseDuration(val)
				if err != nil || d <= 0 {
					return opts, nil, fmt.Errorf("invalid --timeout: %s", val)
				}
				opts.Timeout = d
			case "--retries":
				n, err := strconv.Atoi(val)
				if err != nil || n < 0 {
					return opts, nil, fmt.Errorf("invalid --retries: %s", val)
				}
				opts.Retries = n
			}
		default:
			rest = append(rest, arg)
		}
	}
	return opts, rest, nil
}

func verbosef(opts runOptions, format string, args ...any) {
	if opts.Verbose || debug {
		fmt.Printf(format+"\n", args...)
	}
}

func execWithContext(ctx context.Context, cmd *exec.Cmd) (string, error) {
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("timeout")
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return string(output), fmt.Errorf("exit code %d", status.ExitStatus())
			}
		}
		return string(output), err
	}
	return string(output), nil
}

func buildSSHCommand(si sshInfo, command string) *exec.Cmd {
	if si.Password == "" {
		return exec.Command("ssh", fmt.Sprintf("%s@%s", si.Username, si.Address), "-p", si.Port, command)
	}
	if runtime.GOOS == "windows" {
		return exec.Command("ssh_expect.bat", si.Address, si.Port, si.Username, si.Password, command)
	}
	return exec.Command("ssh_expect.sh", si.Address, si.Port, si.Username, si.Password, command)
}

func buildSCPCommand(si sshInfo, sourcePath, destPath string) *exec.Cmd {
	if si.Password == "" {
		return exec.Command("scp", "-P", si.Port, sourcePath, fmt.Sprintf("%s@%s:%s", si.Username, si.Address, destPath))
	}
	if runtime.GOOS == "windows" {
		return exec.Command("scp_expect.bat", si.Address, si.Port, si.Username, si.Password, sourcePath, destPath)
	}
	return exec.Command("scp_expect.sh", si.Address, si.Port, si.Username, si.Password, sourcePath, destPath)
}

func runAttempt(si sshInfo, action, command, sourcePath, destPath string, opts runOptions) (string, error) {
	if opts.DryRun {
		switch action {
		case "command":
			return fmt.Sprintf("DRY-RUN ssh %s@%s -p %s %q", si.Username, si.Address, si.Port, command), nil
		case "scp":
			return fmt.Sprintf("DRY-RUN scp -P %s %s %s@%s:%s", si.Port, sourcePath, si.Username, si.Address, destPath), nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	var cmd *exec.Cmd
	switch action {
	case "command":
		cmd = buildSSHCommand(si, command)
	case "scp":
		cmd = buildSCPCommand(si, sourcePath, destPath)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}

	return execWithContext(ctx, cmd)
}

func runWithRetries(tag string, si sshInfo, action, command, sourcePath, destPath string, opts runOptions) opResult {
	start := time.Now()
	result := opResult{Tag: tag, Address: si.Address, Action: action}

	attempts := opts.Retries + 1
	for i := 1; i <= attempts; i++ {
		verbosef(opts, "[%s] %s attempt %d/%d", tag, action, i, attempts)
		output, err := runAttempt(si, action, command, sourcePath, destPath, opts)
		result.Attempts = i
		result.Output = strings.TrimSpace(output)
		if err == nil {
			result.Success = true
			result.Duration = time.Since(start).String()
			return result
		}
		result.Error = err.Error()
		if i < attempts {
			verbosef(opts, "[%s] retry after error: %v", tag, err)
		}
	}
	result.Duration = time.Since(start).String()
	return result
}

func printResults(results []opResult, opts runOptions) {
	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		for _, r := range results {
			_ = enc.Encode(r)
		}
		return
	}

	success := 0
	for _, r := range results {
		status := "OK"
		if !r.Success {
			status = "FAIL"
		}
		fmt.Printf("[%s] %s %s (%s, attempts=%d)\n", r.Tag, r.Action, status, r.Duration, r.Attempts)
		if r.Output != "" {
			fmt.Printf("--- output %s ---\n%s\n", r.Tag, r.Output)
		}
		if r.Error != "" {
			fmt.Printf("--- error %s ---\n%s\n", r.Tag, r.Error)
		}
		if r.Success {
			success++
		}
	}
	fmt.Printf("summary: total=%d success=%d failed=%d\n", len(results), success, len(results)-success)
}

func runBatch(tags []string, cfg config, sshAliases map[string]sshInfo, action, command, sourcePath, destPath string, opts runOptions) int {
	if len(tags) == 0 {
		fmt.Fprintln(os.Stderr, "no target tags")
		return 1
	}

	results := make([]opResult, len(tags))
	sem := make(chan struct{}, opts.Parallel)
	var wg sync.WaitGroup
	for i, tag := range tags {
		wg.Add(1)
		go func(idx int, t string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			si := resolveSSHInfo(cfg, sshAliases, t)
			results[idx] = runWithRetries(t, si, action, command, sourcePath, destPath, opts)
		}(i, tag)
	}
	wg.Wait()
	printResults(results, opts)

	for _, r := range results {
		if !r.Success {
			return 1
		}
	}
	return 0
}

func runInteractiveSSH(si sshInfo) {
	fmt.Printf("try to ssh to %s at %s...\n", si.Address, now())
	if debug {
		payload, _ := json.Marshal(si)
		fmt.Println(string(payload))
	}
	cmd := buildSSHCommand(si, "")
	if si.Password == "" {
		cmd = exec.Command("ssh", fmt.Sprintf("%s@%s", si.Username, si.Address), "-p", si.Port)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func findTagIndex(cfg config, tag string) int {
	for i := range cfg.Tags {
		if cfg.Tags[i].Tag.String() == tag {
			return i
		}
	}
	return -1
}

func handleTagCommand(cfg *config, args []string) (int, bool) {
	if len(args) == 0 {
		fmt.Println("usage: axe tag <list|add|edit|del>")
		return 1, false
	}

	switch args[0] {
	case "list":
		if len(cfg.Tags) == 0 {
			fmt.Println("no tags configured")
			return 0, false
		}
		for _, t := range cfg.Tags {
			fmt.Printf("- %s => %s (user=%s port=%s)\n", t.Tag.String(), t.Address.String(), t.Username.String(), t.Port.String())
		}
		return 0, false
	case "add":
		fs := flag.NewFlagSet("tag add", flag.ContinueOnError)
		port := fs.String("port", "", "port")
		user := fs.String("user", "", "user")
		password := fs.String("password", "", "password")
		fs.SetOutput(os.Stderr)
		if err := fs.Parse(args[1:]); err != nil {
			return 1, false
		}
		pos := fs.Args()
		if len(pos) < 2 {
			fmt.Println("usage: axe tag add <tag> <address> [--port 22 --user root --password xxx]")
			return 1, false
		}
		tag := pos[0]
		if findTagIndex(*cfg, tag) >= 0 {
			fmt.Fprintf(os.Stderr, "tag already exists: %s\n", tag)
			return 1, false
		}
		cfg.Tags = append(cfg.Tags, tagEntry{
			Tag:      flexString(tag),
			Address:  flexString(pos[1]),
			Port:     flexString(*port),
			Username: flexString(*user),
			Password: flexString(*password),
		})
		fmt.Printf("tag added: %s\n", tag)
		return 0, true
	case "edit":
		fs := flag.NewFlagSet("tag edit", flag.ContinueOnError)
		address := fs.String("address", "", "address")
		port := fs.String("port", "", "port")
		user := fs.String("user", "", "user")
		password := fs.String("password", "", "password")
		fs.SetOutput(os.Stderr)
		if err := fs.Parse(args[1:]); err != nil {
			return 1, false
		}
		pos := fs.Args()
		if len(pos) < 1 {
			fmt.Println("usage: axe tag edit <tag> [--address host --port 22 --user root --password xxx]")
			return 1, false
		}
		idx := findTagIndex(*cfg, pos[0])
		if idx < 0 {
			fmt.Fprintf(os.Stderr, "tag not found: %s\n", pos[0])
			return 1, false
		}
		if *address != "" {
			cfg.Tags[idx].Address = flexString(*address)
		}
		if *port != "" {
			cfg.Tags[idx].Port = flexString(*port)
		}
		if *user != "" {
			cfg.Tags[idx].Username = flexString(*user)
		}
		if *password != "" {
			cfg.Tags[idx].Password = flexString(*password)
		}
		fmt.Printf("tag updated: %s\n", pos[0])
		return 0, true
	case "del":
		if len(args) < 2 {
			fmt.Println("usage: axe tag del <tag>")
			return 1, false
		}
		idx := findTagIndex(*cfg, args[1])
		if idx < 0 {
			fmt.Fprintf(os.Stderr, "tag not found: %s\n", args[1])
			return 1, false
		}
		cfg.Tags = append(cfg.Tags[:idx], cfg.Tags[idx+1:]...)
		fmt.Printf("tag deleted: %s\n", args[1])
		return 0, true
	default:
		fmt.Println("usage: axe tag <list|add|edit|del>")
		return 1, false
	}
}

func ensureDefault(cfg *config) *passwordEntry {
	if len(cfg.Passwords) == 0 {
		cfg.Passwords = append(cfg.Passwords, passwordEntry{Default: true, Port: flexString("22"), Username: flexString("root")})
		return &cfg.Passwords[0]
	}
	for i := range cfg.Passwords {
		if cfg.Passwords[i].Default {
			return &cfg.Passwords[i]
		}
	}
	cfg.Passwords[0].Default = true
	return &cfg.Passwords[0]
}

func handleDefaultCommand(cfg *config, args []string) (int, bool) {
	if len(args) == 0 {
		fmt.Println("usage: axe default <show|set>")
		return 1, false
	}
	switch args[0] {
	case "show":
		d := ensureDefault(cfg)
		fmt.Printf("default: user=%s port=%s password_set=%t\n", d.Username.String(), d.Port.String(), d.Password.String() != "")
		return 0, false
	case "set":
		fs := flag.NewFlagSet("default set", flag.ContinueOnError)
		port := fs.String("port", "", "port")
		user := fs.String("user", "", "user")
		password := fs.String("password", "", "password")
		fs.SetOutput(os.Stderr)
		if err := fs.Parse(args[1:]); err != nil {
			return 1, false
		}
		d := ensureDefault(cfg)
		if *port != "" {
			d.Port = flexString(*port)
		}
		if *user != "" {
			d.Username = flexString(*user)
		}
		if *password != "" {
			d.Password = flexString(*password)
		}
		fmt.Println("default updated")
		return 0, true
	default:
		fmt.Println("usage: axe default <show|set>")
		return 1, false
	}
}

func run() int {
	opts, args, err := parseGlobalOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid flags: %v\n", err)
		return 1
	}

	if len(args) == 0 || contains(args[0], []string{"-h", "--help", "help"}) {
		help()
		return 0
	}
	if args[0] == "version" || args[0] == "--version" || args[0] == "-v" {
		fmt.Printf("axe %s\n", version)
		return 0
	}

	if contains(args[0], []string{"l", "-l", "lp", "-lp", "e", "-e"}) {
		return editConfig()
	}

	if err := ensureConfigFile(configFile); err != nil {
		fmt.Fprintf(os.Stderr, "failed to prepare config: %v\n", err)
		return 1
	}
	cfg, err := loadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config file %s: %v\n", configFile, err)
		return 1
	}

	switch args[0] {
	case "tag":
		code, changed := handleTagCommand(&cfg, args[1:])
		if changed {
			if err := saveConfig(configFile, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "failed to save config: %v\n", err)
				return 1
			}
		}
		return code
	case "default":
		code, changed := handleDefaultCommand(&cfg, args[1:])
		if changed {
			if err := saveConfig(configFile, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "failed to save config: %v\n", err)
				return 1
			}
		}
		return code
	}

	sshAliases := loadSSHConfigAliases()

	if len(args) == 1 {
		si := resolveSSHInfo(cfg, sshAliases, args[0])
		runInteractiveSSH(si)
		return 0
	}

	hasS := contains("-s", args)
	hasC := contains("-c", args)
	if hasS && hasC {
		help()
		return 1
	}

	if hasS {
		idx := indexOf("-s", args)
		tags := args[:idx]
		paths := args[idx+1:]
		if len(tags) == 0 || len(paths) != 2 {
			help()
			return 1
		}
		return runBatch(tags, cfg, sshAliases, "scp", "", paths[0], paths[1], opts)
	}

	if hasC {
		idx := indexOf("-c", args)
		tags := args[:idx]
		commands := args[idx+1:]
		if len(tags) == 0 || len(commands) != 1 {
			help()
			return 1
		}
		return runBatch(tags, cfg, sshAliases, "command", commands[0], "", "", opts)
	}

	help()
	return 1
}

func main() {
	if runtime.GOOS == "windows" {
		_ = exec.Command("chcp", "65001").Run()
	}
	os.Exit(run())
}
