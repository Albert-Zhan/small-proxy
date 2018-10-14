package main

import (
	"net/http"
	"fmt"
	"hosts/conf"
	"flag"
	"strings"
	"net/url"
	"net/http/httputil"
	"log"
	"runtime"
	"os"
	"os/exec"
	"io/ioutil"
	"path/filepath"
)

var config conf.Config

type handle struct {

}

var start *bool=flag.Bool("start",false,"server start")
var stop *bool=flag.Bool("stop",false,"server stop")
var guard *bool=flag.Bool("d",false,"Daemon start")


//反向代理服务实现方法
func (this *handle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//读取当前域名的配置信息
	data:=config.Read("hosts",r.Host)
	host:=strings.Split(data,":")
	//当前请求的域名在配置内
	if len(host)==2{
		remote, err := url.Parse("http://" + host[0] + ":" + host[1])
		if err != nil {
			panic(err)
		}
		proxy := httputil.NewSingleHostReverseProxy(remote)
		proxy.ServeHTTP(w,r)
	}else{
		//当前请求的域名不在配置内
		fmt.Fprintln(w,"502 Bad Gateway")
	}
}

//开启反向代理服务
func startServer() {
	//读取配置文件
	config=conf.Config{}
	config.InitConfig("./hosts.conf")
	//监听端口
	h := &handle{}
	err := http.ListenAndServe(":80", h)
	if err != nil {
		log.Fatalln("ListenAndServe: ", err)
	}
}

func main() {
	//开启多核处理
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()
	//开启守护进程
	if *start && *guard{
		filePath, _ := filepath.Abs(os.Args[0])
		cmd:=exec.Command(filePath,"-start")
		cmd.Start()
		ioutil.WriteFile("pid.pid", []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0666)
		fmt.Println("start success")
		os.Exit(0)
	}
	//开启进程
	if *start{
		startServer()
	}
	//关闭守护进程
	if *stop{
		strb, _ := ioutil.ReadFile("pid.pid")
		var command *exec.Cmd
		//windows系统特殊处理
		if runtime.GOOS=="windows"{
			command = exec.Command("tskill", string(strb))
		}else{
			command = exec.Command("kill", string(strb))
		}
		command.Start()
		fmt.Println("stop success")
		os.Exit(0)
	}
	fmt.Println("Please input parameters")
}