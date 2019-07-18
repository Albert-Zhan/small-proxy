package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"runtime"
	"time"
)

var start *bool=flag.Bool("start",false,"Start server.")
var stop *bool=flag.Bool("stop",false,"Stop server.")
var guard *bool=flag.Bool("d",false,"Start Daemon.")
var localPort *string = flag.String("l", "5531", "Ports accessed by users.")
var remotePort *string = flag.String("r", "2001", "Port to communicate with client.")

//与客户端相关的连接
type client struct {
	conn net.Conn
	er   chan bool
	//未收到心跳包通道
	heart chan bool
	writ  chan bool
	recv  chan []byte
	send  chan []byte
}

//读取客户端发送过来的数据
func (self *client) read() {
	for {
		//40秒没有数据传输则断开
		_ = self.conn.SetReadDeadline(time.Now().Add(40*time.Second))
		var recv=make([]byte,10240)
		n, err := self.conn.Read(recv)
		if err != nil {
			self.heart <- true
			self.er <- true
			self.writ <- true
		}
		//收到心跳包hh，原样返回回复
		if recv[0] == 'h' && recv[1] == 'h' {
			_, _ = self.conn.Write([]byte("hh"))
			continue
		}
		self.recv <- recv[:n]
	}
}

//把数据发送给客户端
func (self client) write() {
	for {
		var send=make([]byte,10240)
		select {
		case send = <-self.send:
			_, _ = self.conn.Write(send)
		case <-self.writ:
			break
		}
	}
}

//与用户相关的连接
type user struct {
	conn net.Conn
	er   chan bool
	writ chan bool
	recv chan []byte
	send chan []byte
}

//读取用户发送过来的数据
func (self user) read() {
	_ = self.conn.SetReadDeadline(time.Now().Add(800*time.Millisecond))
	for {
		var recv = make([]byte,10240)
		n, err := self.conn.Read(recv)
		_ = self.conn.SetReadDeadline(time.Time{})
		if err != nil {
			self.er <- true
			self.writ <- true
			break
		}
		self.recv <- recv[:n]
	}
}

//把数据发送给用户
func (self user) write() {
	for {
		var send=make([]byte,10240)
		select {
		case send = <-self.send:
			_, _ = self.conn.Write(send)
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
		cmd:=exec.Command(os.Args[0],"-start","-r",*remotePort,"-l",*localPort)
		err:=cmd.Start()
		if err!=nil {
			fmt.Println(err.Error())
			os.Exit(2)
		}
		
		_ = ioutil.WriteFile("./proxy.pid", []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0666)
		fmt.Println("Start success.")
		os.Exit(2)
	}
	//开启进程
	if *start{
		startServer()
	}
	//关闭守护进程
	if *stop{
		b, _ := ioutil.ReadFile("./proxy.pid")
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

		_ = os.Remove("./proxy.pid")
		fmt.Println("Stop success.")
		os.Exit(2)
	}
	fmt.Println("Please input parameters.")
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
