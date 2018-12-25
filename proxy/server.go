package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"
	"io/ioutil"
	"os/exec"
	"path/filepath"
)

var start *bool=flag.Bool("start",false,"server start")
var stop *bool=flag.Bool("stop",false,"server stop")
var guard *bool=flag.Bool("d",false,"Daemon start")
var localPort *string = flag.String("l", "", "user visit port")
var remotePort *string = flag.String("r", "", "and client communication port")

//与client相关的conn
type client struct {
	conn net.Conn
	er   chan bool
	//未收到心跳包通道
	heart chan bool
	//暂未使用！！！原功能tcp连接已经接通，不在需要心跳包
	disheart bool
	writ     chan bool
	recv     chan []byte
	send     chan []byte
}

//读取client过来的数据
func (self *client) read() {
	for {
		//40秒没有数据传输则断开
		self.conn.SetReadDeadline(time.Now().Add(time.Second * 40))
		var recv []byte = make([]byte, 10240)
		n, err := self.conn.Read(recv)
		if err != nil {
			self.heart <- true
			self.er <- true
			self.writ <- true
		}
		//收到心跳包hh，原样返回回复
		if recv[0] == 'h' && recv[1] == 'h' {
			self.conn.Write([]byte("hh"))
			continue
		}
		self.recv <- recv[:n]
	}
}

//把数据发送给client
func (self client) write() {
	for {
		var send []byte = make([]byte, 10240)
		select {
		case send = <-self.send:
			self.conn.Write(send)
		case <-self.writ:
			break
		}
	}
}

//与user相关的conn
type user struct {
	conn net.Conn
	er   chan bool
	writ chan bool
	recv chan []byte
	send chan []byte
}

//读取user过来的数据
func (self user) read() {
	self.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 800))
	for {
		var recv []byte = make([]byte, 10240)
		n, err := self.conn.Read(recv)
		self.conn.SetReadDeadline(time.Time{})
		if err != nil {
			self.er <- true
			self.writ <- true
			break
		}
		self.recv <- recv[:n]
	}
}

//把数据发送给user
func (self user) write() {
	for {
		var send []byte = make([]byte, 10240)
		select {
		case send = <-self.send:
			self.conn.Write(send)
		case <-self.writ:
			break
		}
	}

}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()
	//开启守护进程
	if *start && *guard{
		filePath, _ := filepath.Abs(os.Args[0])
		cmd:=exec.Command(filePath,"-start","-r",*remotePort,"-l",*localPort)
		cmd.Start()
		ioutil.WriteFile("proxy.pid", []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0666)
		fmt.Println("start success")
		os.Exit(0)
	}
	//开启进程
	if *start{
		startServer()
	}
	//关闭守护进程
	if *stop{
		strb, _ := ioutil.ReadFile("proxy.pid")
		var command *exec.Cmd
		//结束守护进程
		if runtime.GOOS=="windows"{
			command = exec.Command("taskkill","/pid",string(strb))
		}else{
			command = exec.Command("kill","-9",string(strb))
		}
		command.Start()
		fmt.Println("stop success")
		os.Exit(0)
	}
	fmt.Println("Please input parameters")
}

//开启内网穿透服务
func startServer()  {
	if *localPort=="" || *remotePort==""{
		flag.PrintDefaults()
		os.Exit(1)
	}
	//监听端口
	c, err := net.Listen("tcp", ":"+*remotePort)
	log(err)
	u, err := net.Listen("tcp", ":"+*localPort)
	log(err)
	//第一条tcp关闭或者与浏览器建立tcp都要返回重新监听
TOP:
//监听user链接
	Uconn := make(chan net.Conn)
	go goaccept(u, Uconn)
	//一定要先接受client
	fmt.Println("准备好连接了")
	clientconnn := accept(c)
	fmt.Println("client已连接", clientconnn.LocalAddr().String())
	recv := make(chan []byte)
	send := make(chan []byte)
	heart := make(chan bool, 1)
	//1个位置是为了防止两个读取线程一个退出后另一个永远卡住
	er := make(chan bool, 1)
	writ := make(chan bool)
	client := &client{clientconnn, er, heart, false, writ, recv, send}
	go client.read()
	go client.write()
	//这里可能需要处理心跳
	for {
		select {
		case <-client.heart:
			goto TOP
		case userconnn := <-Uconn:
			//暂未使用
			client.disheart = true
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
	logExit(err)
	return CorU
}

//在另一个进程监听端口函数
func goaccept(con net.Listener, Uconn chan net.Conn) {
	CorU, err := con.Accept()
	logExit(err)
	Uconn <- CorU
}

//显示错误
func log(err error) {
	if err != nil {
		fmt.Printf("出现错误： %v\n", err)
	}
}

//显示错误并退出
func logExit(err error) {
	if err != nil {
		runtime.Goexit()
	}
}

//显示错误并关闭链接，退出线程
func logClose(err error, conn net.Conn) {
	if err != nil {
		runtime.Goexit()
	}
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
			client.conn.Close()
			user.conn.Close()
			runtime.Goexit()
		case <-client.er:
			user.conn.Close()
			client.conn.Close()
			runtime.Goexit()
		}
	}
}