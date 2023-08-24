/*
 * @Description: 
 * @Date: 2023-02-14 17:42:51
 * @LastEditTime: 2023-08-11 15:54:55
 * @FilePath: \testgo\axe2\main.go
 */
package main

import (
	"os"
	"os/exec"
	"io/ioutil"
	"time"
	"fmt"
	"encoding/json"
	"runtime"
	"github.com/fatih/color"
	"github.com/tidwall/gjson"
	"axe.v2/ui"
)

var DEBUG = false 
var ConfigFile = ".axe.v2.config.json"

var Title = color.New(color.FgCyan).Add(color.Bold)
var Green = color.New(color.FgGreen)
var Yellow = color.New(color.FgYellow)
var Red = color.New(color.FgRed)

// ssh information
type SI struct {
	Address string
	Port string
	Username string
	Password string
}

func help() {
	Title.Println("help:")
	Green.Println(`  axe <tag> ==> ssh to tagged host by stored information or ssh to any host accessible with default name/pwd/port;
  axe <tag1> <tag2> -c 'ls -lrt' ==> run command on <tag1>/<tag2> and show result;
  axe <tag1> <tag2> -s './test' '/root' ==> scp file to <tag1>/<tag2> on purpose;`)	
  	Title.Println("conf:")
  	Green.Println(`  axe -l/l ==> list/edit/delete ssh information of tagged hosts;
  axe -lp/lp ==> list/edit/delete the default ssh information;	
  axe -e/e ==> use system editor to edit the config file;`)
	Title.Println("env:")
	Green.Println(`  axe_debug ==> to enable debug mode, set it to any value not null;
  axe_port/axe_username/axe_password ==> overwrite stored default ssh information (the result of 'axe -lp');`)
}

func now() string {
	return time.Now().Format("2006-01-02 15:04:05")
} 

func in(item string, list []string) bool {
    for _, v := range list {
        if item == v {
            return true
        }
    }
    return false
}

func indexOf(item string, list []string) (int) {
	for k, v := range list {
		if item == v {
			return k
		}
	}
	return -1
 }

func callDefaultEditor(file string) {
	if runtime.GOOS == "windows" {
		exec.Command("notepad", file).Run()
	} else {
		exec.Command("vi", file).Run()
	} 
}

func config_tags() {
	// todo
	callDefaultEditor(ConfigFile)
}

func config_default_ssh_informations() {
	// todo
	callDefaultEditor(ConfigFile)
}

func edit_config_file() {
	callDefaultEditor(ConfigFile)
}

func ssh_to(si SI) {
	Green.Printf("try to ssh to %s at %s...\n", si.Address, now())
	if DEBUG {
		si_json, _ := json.Marshal(si)
		Yellow.Println(string(si_json))
	}
	var cmd *exec.Cmd
	// no password
	if si.Password == "" {
		cmd = exec.Command("ssh", fmt.Sprintf(`%s@%s`, si.Username, si.Address), "-p", si.Port)
	} else if runtime.GOOS == "windows" {
	    cmd = exec.Command("ssh_expect.bat", si.Address, si.Port, si.Username, si.Password)
	} else {
		cmd = exec.Command("ssh_expect.sh", si.Address, si.Port, si.Username, si.Password)
	}
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout	
	if err := cmd.Run(); err != nil {
		Red.Println(err)
	}
}

func scp_to(si SI, sourceDir string, destDir string) {
	Green.Printf("try to scp %s to %s@%s:%s at %s...\n", sourceDir, si.Username, si.Address, destDir, now())
	if DEBUG {
		si_json, _ := json.Marshal(si)
		Yellow.Println(string(si_json), Yellow.Sprintf(`{"sourceDir":'%s',"destDir":'%s'}`,sourceDir, destDir))
	}
	var cmd *exec.Cmd
	// no password
	if si.Password == "" {
		cmd = exec.Command("scp", "-P", si.Port, sourceDir, fmt.Sprintf(`%s@%s:%s`, si.Username, si.Address, destDir))
	} else if runtime.GOOS == "windows" {
	    cmd = exec.Command("scp_expect.bat", si.Address, si.Port, si.Username, si.Password)
	} else {
		cmd = exec.Command("scp_expect.sh", si.Address, si.Port, si.Username, si.Password, sourceDir, destDir)
	}
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		Red.Println(err)
	}	
}

func execute_command(si SI, command string) {
	Green.Printf("try to execute command '%s' on %s at %s...\n",  command, si.Address, now())
	if DEBUG {
		si_json, _ := json.Marshal(si)
		Yellow.Println(string(si_json), Yellow.Sprintf(`{"command":'%s'}`,command))
	}
	var cmd *exec.Cmd
	// no password
	if si.Password == "" {
		cmd = exec.Command("ssh", fmt.Sprintf(`%s@%s`, si.Username, si.Address), "-p", si.Port, fmt.Sprintf(`'%s'`, command))
	} else if runtime.GOOS == "windows" {
		cmd = exec.Command("ssh_expect.bat", si.Address, si.Port, si.Username, si.Password, fmt.Sprintf(`'%s'`, command))
	} else {
		cmd = exec.Command("scp_expect.sh", si.Address, si.Port, si.Username, si.Password, fmt.Sprintf(`'%s'`, command))
	}
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		Red.Println(err)
	}
}

func get_ssh_info_by_tag(content []byte, tag string) SI {
	address := gjson.Get(string(content), `tags.#(tag="`+tag+`").address`).String()
	port := gjson.Get(string(content), `tags.#(tag="`+tag+`").port`).String()
	username := gjson.Get(string(content), `tags.#(tag="`+tag+`").username`).String()
	password := gjson.Get(string(content), `tags.#(tag="`+tag+`").password`).String()
	if address == "" {
		// use tag as address, use the default password set as password
		address = tag
		if os.Getenv("axe_password") != "" {
			password = os.Getenv("axe_password")
		} else {
			password = gjson.Get(string(content), `passwords.#(default="true").password`).String()
		}
	}
	if port == "" {
		if os.Getenv("axe_port") != "" {
			port = os.Getenv("axe_port")
		} else if gjson.Get(string(content), `passwords.#(default="true").port`).String() != "" {
			port = gjson.Get(string(content), `passwords.#(default="true").port`).String()
		} else {
			port = "22"
		}
	}
	if username == "" {
		if os.Getenv("axe_username") != "" {
			username = os.Getenv("axe_username")
		} else if gjson.Get(string(content), `passwords.#(default="true").username`).String() != "" {
			username = gjson.Get(string(content), `passwords.#(default="true").username`).String()
		} else {
			username = "root"
		}
	}
	return SI {
		Address: address,
		Port: port,
		Username: username,
		Password: password,
	}
}

func main() {
	if os.Getenv("axe_debug") != "" {
		DEBUG = true
	}
	// set to utf-8
	if runtime.GOOS == "windows" {
		exec.Command("chcp", "65001").Run()
	}
	// read file from 
	content, err := ioutil.ReadFile(ConfigFile)
	if err != nil {
		Red.Printf("failed to read config file cause %v\n", err)	
	}
	
	var parameter = os.Args[1:]
	if len(parameter) == 0 {
		help()
	}else if len(parameter) == 1 {
		if in(parameter[0], []string{"l", "-l"}){
        	config_tags()
		}else if in(parameter[0], []string{"lp", "-lp"}){
			config_default_ssh_informations()
		}else if in(parameter[0], []string{"e", "-e"}){
        	edit_config_file()
		}else{
			si := get_ssh_info_by_tag(content, parameter[0])
        	ssh_to(si)
		}
	}else if in("-s", parameter) && !in("-c", parameter){
		// scp 
		flag_position := indexOf("-s", parameter)
		tags := parameter[:flag_position]
    	commands := parameter[(flag_position+1):]
		
		if len(tags) == 0 || len(commands) != 2 {
			help()
			os.Exit(0)
		}
		for _, t := range tags {
			si := get_ssh_info_by_tag(content, t)
			scp_to(si, commands[0], commands[1])
		}
	}else if in("-c", parameter) && !in("-s", parameter){
		// ssh and run cmd
		flag_position := indexOf("-c", parameter)
		tags := parameter[:flag_position]
    	commands := parameter[(flag_position+1):]
		
		if len(tags) == 0 || len(commands) != 1 {
			help()
			os.Exit(0)
		}
		for _, t := range tags {
			si := get_ssh_info_by_tag(content, t)
			execute_command(si, commands[0])
		}
	}
}