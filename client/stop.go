package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
)

type stop struct {}

func (this *stop) addFlags() {

}

func (this *stop) printHelp(indent string) {
	fmt.Println(indent, "This command is used to shut down client intranet penetration.")
	fmt.Println(indent, "Command usage: proxy stop")
}

func (this *stop) run(args []string) {
	b,_:=ioutil.ReadFile("./proxy.pid")
	var command *exec.Cmd
	//结束守护进程
	if runtime.GOOS=="windows"{
		command=exec.Command("taskkill","/F","/PID",string(b))
	}else{
		command=exec.Command("kill","-9",string(b))
	}
	err:=command.Start()
	if err!=nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}

	_ = os.Remove("./proxy.pid")
	fmt.Println("Stop success.")
	os.Exit(2)
}
