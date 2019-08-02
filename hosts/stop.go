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
	fmt.Println(indent, "This command is used to close a reverse proxy.")
	fmt.Println(indent, "Command usage: hosts stop")
}

func (this *stop) run(args []string) {
	b,_:=ioutil.ReadFile("./hosts.pid")
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

	_ = os.Remove("./hosts.pid")
	fmt.Println("Stop success.")
	os.Exit(2)
}
