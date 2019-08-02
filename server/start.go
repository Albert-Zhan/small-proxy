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
	local string
	remote string
	d bool
}

func (this *start) addFlags() {
	flag.StringVar(&this.local, "l", "5531", "Ports accessed by users.")
	flag.StringVar(&this.remote, "r", "2001", "Port to communicate with client.")
	flag.BoolVar(&this.d, "d", false, "Open daemon.")
}

func (this *start) printHelp(indent string) {
	fmt.Println(indent, "This command is used to open server intranet penetration.")
	fmt.Println(indent, "Command usage: proxy start [parameters]")
}

func (this *start) run(args []string) {
	if len(args)==0 {
		help()
	}

	if this.d==true {
		cmd:=exec.Command(os.Args[0],"-start","-l",this.local,"-r",this.remote)
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
	//监听端口
	c, err := net.Listen("tcp", ":"+this.remote)
	if err != nil {
		fmt.Printf("出现错误： %v\n", err)
	}
	u, err := net.Listen("tcp", ":"+this.local)
	if err != nil {
		fmt.Printf("出现错误： %v\n", err)
	}
TOP:
	//监听用户连接
	Uconn := make(chan net.Conn)
	go goaccept(u, Uconn)
	//一定要先接受client
	fmt.Println("准备好连接了")
	clientconnn := accept(c)
	recv := make(chan []byte)
	send := make(chan []byte)
	heart := make(chan bool, 1)
	//1个位置是为了防止两个读取线程一个退出后另一个永远卡住
	er := make(chan bool, 1)
	writ := make(chan bool)
	client := &client{clientconnn, er, heart, writ, recv, send}
	go client.read()
	go client.write()
	//这里可能需要处理心跳
	for {
		select {
		case <-client.heart:
			goto TOP
		case userconnn := <-Uconn:
			recv = make(chan []byte)
			send = make(chan []byte)
			//1个位置是为了防止两个读取线程一个退出后另一个永远卡住
			er = make(chan bool, 1)
			writ = make(chan bool)
			user := &user{userconnn, er, writ, recv, send}
			go user.read()
			go user.write()
			//当两个socket都创立后进入handle处理
			go handle(client, user)
			goto TOP
		}
	}
}

//监听端口函数
func accept(con net.Listener) net.Conn {
	CorU, err := con.Accept()
	if err != nil {
		runtime.Goexit()
	}
	return CorU
}

//在另一个进程监听端口函数
func goaccept(con net.Listener, Uconn chan net.Conn) {
	CorU, err := con.Accept()
	if err != nil {
		runtime.Goexit()
	}
	Uconn <- CorU
}

//两个socket衔接相关处理
func handle(client *client, user *user) {
	for {
		var clientrecv = make([]byte, 10240)
		var userrecv = make([]byte, 10240)
		select {
		case clientrecv = <-client.recv:
			user.send <- clientrecv
		case userrecv = <-user.recv:
			client.send <- userrecv
		case <-user.er:
			_ = client.conn.Close()
			_ = user.conn.Close()
			runtime.Goexit()
		case <-client.er:
			_ = user.conn.Close()
			_ = client.conn.Close()
			runtime.Goexit()
		}
	}
}
