package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"runtime"
)

type start struct {
	ip string
	port string
	local string
	d bool
}

func (this *start) addFlags() {
	flag.StringVar(&this.ip, "h", "", "Server IP.")
	flag.StringVar(&this.port, "r", "2001", "Server Port.")
	flag.StringVar(&this.local, "l", "127.0.0.1:80", "Local IP and Port.")
	flag.BoolVar(&this.d, "d", false, "Open daemon.")
}

func (this *start) printHelp(indent string) {
	fmt.Println(indent, "This command is used to open intranet penetration connecting the server.")
	fmt.Println(indent, "Command usage: proxy start [parameters]")
}

func (this *start) run(args []string) {
	if len(args)==0 || this.ip=="" {
		help()
	}

	if this.d==true {
		cmd:=exec.Command(os.Args[0],"start","-h",this.ip,"-r",this.port,"-l",this.local)
		err:=cmd.Start()
		if err!=nil {
			fmt.Println(err.Error())
			os.Exit(2)
		}

		_ = ioutil.WriteFile("./proxy.pid", []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0666)
		fmt.Println("Start success.")
		os.Exit(2)
	}else{
		this.startServer()
	}
}

//开启内网穿透
func (this *start) startServer() {
	target := net.JoinHostPort(this.ip,this.port)
	for {
		//链接端口
		serverconn := dail(target)
		recv := make(chan []byte)
		send := make(chan []byte)
		//1个位置是为了防止两个读取线程一个退出后另一个永远卡住
		er := make(chan bool, 1)
		writ := make(chan bool)
		next := make(chan bool)
		server := &server{serverconn, er, writ, recv, send}
		go server.read()
		go server.write()
		go handle(this.local,server,next)
		<-next
	}
}

//链接端口
func dail(hostport string) net.Conn {
	conn, err := net.Dial("tcp", hostport)
	if err!=nil {
		fmt.Printf("出现错误，退出线程： %v\n", err)
		runtime.Goexit()
	}
	return conn
}

//两个socket衔接相关处理
func handle(localPort string,server *server, next chan bool) {
	var serverrecv=make([]byte,10240)
	//阻塞这里等待服务端传来数据再链接浏览器
	fmt.Println("等待server发来消息")
	serverrecv = <-server.recv
	next <- true
	var browse *browser
	//服务端发来数据，链接本地80端口
	serverconn := dail(localPort)
	recv := make(chan []byte)
	send := make(chan []byte)
	er := make(chan bool, 1)
	writ := make(chan bool)
	browse = &browser{serverconn, er, writ, recv, send}
	go browse.read()
	go browse.write()
	browse.send <- serverrecv
	for {
		var serverrecv = make([]byte, 10240)
		var browserrecv = make([]byte, 10240)
		select {
		case serverrecv = <-server.recv:
			if serverrecv[0] != '0' {
				browse.send <- serverrecv
			}
		case browserrecv = <-browse.recv:
			server.send <- browserrecv
		case <-server.er:
			_ = server.conn.Close()
			_ = browse.conn.Close()
			runtime.Goexit()
		case <-browse.er:
			_ = server.conn.Close()
			_ = browse.conn.Close()
			runtime.Goexit()
		}
	}
}
