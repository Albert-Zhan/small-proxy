package main

import (
	"flag"
	"fmt"
	"hosts/conf"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var config conf.Config

type handle struct {

}

var start *bool=flag.Bool("start",false,"Start server.")
var stop *bool=flag.Bool("stop",false,"Stop server.")
var guard *bool=flag.Bool("d",false,"Start Daemon.")

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
		_, _ = fmt.Fprintln(w, "502 Bad Gateway")
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
		fmt.Println(err.Error())
		os.Exit(2)
	}
}

func main() {
	//开启多核处理
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()
	//开启守护进程
	if *start && *guard{
		cmd:=exec.Command(os.Args[0],"-start")
		err:=cmd.Start()
		if err!=nil {
			fmt.Println(err.Error())
			os.Exit(2)
		}

		_ = ioutil.WriteFile("./hosts.pid", []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0666)
		fmt.Println("Start success.")
		os.Exit(2)
	}
	//开启进程
	if *start{
		startServer()
	}
	//关闭守护进程
	if *stop{
		b, _ := ioutil.ReadFile("./hosts.pid")
		var command *exec.Cmd
		//结束守护进程
		if runtime.GOOS=="windows"{
			command = exec.Command("taskkill","/F","/PID",string(b))
		}else{
			command = exec.Command("kill","-9",string(b))
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
	fmt.Println("Please input parameters.")
}
