package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"proxy/conf"
	"runtime"
	"strings"
)

var commands map[string]command
var provider command
var config conf.Config

type handle struct {}

//反向代理服务实现方法
func (this *handle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//读取当前域名的配置信息
	data:=config.Read("hosts",r.Host)
	host:=strings.Split(data,":")

	//当前请求的域名在配置内
	if len(host)==2{
		remote, err := url.Parse("http://" + host[0] + ":" + host[1])
		if err == nil {
			proxy := httputil.NewSingleHostReverseProxy(remote)
			proxy.ServeHTTP(w,r)
		}
	}else{
		//当前请求的域名不在配置内
		_, _ = fmt.Fprintln(w, "502 Bad Gateway")
	}
}

func printUsage() {
	fmt.Println("Usage: hosts [command] [parameters], where command is one of:")
	fmt.Print("  ")

	i := 0
	for id := range commands {
		fmt.Print(id)

		if i < len(commands)-1 {
			fmt.Print(", ")
		}
		i++
	}

	fmt.Println(" or help")
	fmt.Println()

	if provider != nil {
		provider.printHelp(" ")
	} else {
		for id, provider := range commands {
			fmt.Printf("  %s\n", id)
			provider.printHelp("   ")
			fmt.Printf("    For more information run \"hosts help %s\"\n", id)
			fmt.Println("")
		}
	}
	flag.PrintDefaults()
}

func help() {
	printUsage()
	os.Exit(2)
}

func loadCommands() {
	commands = make(map[string]command)
	commands["start"]=&start{}
	commands["stop"] = &stop{}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	loadCommands()
	flag.Usage = printUsage

	args := os.Args[1:]
	if len(args) < 1 {
		help()
	}

	command := args[0]
	if command[0] == '-' {
		help()
	}

	if command == "help" {
		if len(args) >= 2 {
			provider = commands[args[1]]
			provider.addFlags()
		}
		help()
	} else if command == "version" {
		fmt.Printf("hosts version %s","0.3.3")
	}else {
		provider = commands[command]
		if provider == nil {
			fmt.Println("Unsupported command", command)
			return
		}

		provider.addFlags()
		if command=="start" {
			_ = flag.CommandLine.Parse(args[1:])
		}
		provider.run(os.Args[2:])
	}
}
