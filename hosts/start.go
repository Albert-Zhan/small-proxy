package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"proxy/conf"
)

type start struct {
	d bool
}

func (this *start) addFlags() {
	flag.BoolVar(&this.d, "d", false, "Open daemon.")
}

func (this *start) printHelp(indent string) {
	fmt.Println(indent, "This command is used to open a reverse proxy.")
	fmt.Println(indent, "Command usage: hosts start [parameters]")
}

func (this *start) run(args []string) {
	if this.d==true {
		cmd:=exec.Command(os.Args[0],"start")
		err:=cmd.Start()
		if err!=nil {
			fmt.Println(err.Error())
			os.Exit(2)
		}

		_ = ioutil.WriteFile("./hosts.pid", []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0666)
		fmt.Println("Start success.")
		os.Exit(2)
	}else{
		this.startServer()
	}
}

//开启反向代理
func (this *start) startServer() {
	if !IsFile("./hosts.conf") {
		fmt.Println("Reverse Proxy Profile does not exist.")
		os.Exit(2)
	}

	//读取配置文件
	config=conf.Config{}
	config.InitConfig("./hosts.conf")
	//监听端口
	h := &handle{}
	err := http.ListenAndServe(":80",h)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}
}

//判断路径是否为文件
func IsFile(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}
