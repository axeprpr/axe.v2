package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
)

const configFile = ".axe.v2.config.json"

var debug = os.Getenv("axe_debug") != ""

type sshInfo struct {
	Address  string
	Port     string
	Username string
	Password string
}

type tagEntry struct {
	Tag      string `json:"tag"`
	Address  string `json:"address"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type passwordEntry struct {
	Default  bool   `json:"default"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type config struct {
	Tags      []tagEntry      `json:"tags"`
	Passwords []passwordEntry `json:"passwords"`
}

func help() {
	fmt.Println("help:")
	fmt.Println("  axe <tag>                                  ssh 到配置中的主机，或把 <tag> 当作主机地址")
	fmt.Println("  axe <tag1> <tag2> -c \"ls -lrt\"           在多个主机上执行命令")
	fmt.Println("  axe <tag1> <tag2> -s ./local /remote/path   复制文件到多个主机")
	fmt.Println("conf:")
	fmt.Println("  axe -l | l                                  编辑 tags 配置")
	fmt.Println("  axe -lp | lp                                编辑默认 ssh 配置")
	fmt.Println("  axe -e | e                                  直接编辑配置文件")
	fmt.Println("env:")
	fmt.Println("  axe_debug                                   开启调试输出")
	fmt.Println("  axe_port axe_username axe_password          覆盖默认端口/用户名/密码")
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

func editConfig() {
	if err := ensureConfigFile(configFile); err != nil {
		fmt.Fprintf(os.Stderr, "failed to prepare config: %v\n", err)
		return
	}
	if err := openEditor(configFile); err != nil {
		fmt.Fprintf(os.Stderr, "failed to open editor: %v\n", err)
	}
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

func resolveSSHInfo(cfg config, tag string) sshInfo {
	info := sshInfo{Address: tag, Port: "22", Username: "root"}

	if d := defaultPassword(cfg); d != nil {
		if d.Port != "" {
			info.Port = d.Port
		}
		if d.Username != "" {
			info.Username = d.Username
		}
		info.Password = d.Password
	}

	for _, t := range cfg.Tags {
		if t.Tag != tag {
			continue
		}
		if t.Address != "" {
			info.Address = t.Address
		}
		if t.Port != "" {
			info.Port = t.Port
		}
		if t.Username != "" {
			info.Username = t.Username
		}
		if t.Password != "" {
			info.Password = t.Password
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

func runInteractiveCommand(cmd *exec.Cmd) {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func sshTo(si sshInfo) {
	fmt.Printf("try to ssh to %s at %s...\n", si.Address, now())
	if debug {
		payload, _ := json.Marshal(si)
		fmt.Println(string(payload))
	}

	if si.Password == "" {
		runInteractiveCommand(exec.Command("ssh", fmt.Sprintf("%s@%s", si.Username, si.Address), "-p", si.Port))
		return
	}
	if runtime.GOOS == "windows" {
		runInteractiveCommand(exec.Command("ssh_expect.bat", si.Address, si.Port, si.Username, si.Password))
		return
	}
	runInteractiveCommand(exec.Command("ssh_expect.sh", si.Address, si.Port, si.Username, si.Password))
}

func scpTo(si sshInfo, sourcePath, destPath string) {
	fmt.Printf("try to scp %s to %s@%s:%s at %s...\n", sourcePath, si.Username, si.Address, destPath, now())
	if debug {
		payload, _ := json.Marshal(si)
		fmt.Printf("%s {\"sourcePath\":\"%s\",\"destPath\":\"%s\"}\n", string(payload), sourcePath, destPath)
	}

	if si.Password == "" {
		runInteractiveCommand(exec.Command("scp", "-P", si.Port, sourcePath, fmt.Sprintf("%s@%s:%s", si.Username, si.Address, destPath)))
		return
	}
	if runtime.GOOS == "windows" {
		runInteractiveCommand(exec.Command("scp_expect.bat", si.Address, si.Port, si.Username, si.Password, sourcePath, destPath))
		return
	}
	runInteractiveCommand(exec.Command("scp_expect.sh", si.Address, si.Port, si.Username, si.Password, sourcePath, destPath))
}

func executeCommand(si sshInfo, command string) {
	fmt.Printf("try to execute command %q on %s at %s...\n", command, si.Address, now())
	if debug {
		payload, _ := json.Marshal(si)
		fmt.Printf("%s {\"command\":\"%s\"}\n", string(payload), command)
	}

	if si.Password == "" {
		runInteractiveCommand(exec.Command("ssh", fmt.Sprintf("%s@%s", si.Username, si.Address), "-p", si.Port, command))
		return
	}
	if runtime.GOOS == "windows" {
		runInteractiveCommand(exec.Command("ssh_expect.bat", si.Address, si.Port, si.Username, si.Password, command))
		return
	}
	runInteractiveCommand(exec.Command("ssh_expect.sh", si.Address, si.Port, si.Username, si.Password, command))
}

func run() int {
	args := os.Args[1:]
	if len(args) == 0 || contains(args[0], []string{"-h", "--help", "help"}) {
		help()
		return 0
	}

	cfg, err := loadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config file %s: %v\n", configFile, err)
		return 1
	}

	if len(args) == 1 {
		switch args[0] {
		case "l", "-l", "lp", "-lp", "e", "-e":
			editConfig()
		default:
			sshTo(resolveSSHInfo(cfg, args[0]))
		}
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
		for _, t := range tags {
			scpTo(resolveSSHInfo(cfg, t), paths[0], paths[1])
		}
		return 0
	}

	if hasC {
		idx := indexOf("-c", args)
		tags := args[:idx]
		commands := args[idx+1:]
		if len(tags) == 0 || len(commands) != 1 {
			help()
			return 1
		}
		for _, t := range tags {
			executeCommand(resolveSSHInfo(cfg, t), commands[0])
		}
		return 0
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
