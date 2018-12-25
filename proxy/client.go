package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"
	"path/filepath"
	"os/exec"
	"io/ioutil"
)

var start *bool=flag.Bool("start",false,"server start")
var stop *bool=flag.Bool("stop",false,"server stop")
var guard *bool=flag.Bool("d",false,"Daemon start")
var host *string = flag.String("h", "", "server ip")
var remotePort *string = flag.String("r", "", "server port")
var localPort *string = flag.String("l", "", "local ip and port")

//与browser相关的conn
type browser struct {
	conn net.Conn
	er   chan bool
	writ chan bool
	recv chan []byte
	send chan []byte
}

//读取browser过来的数据
func (self browser) read() {
	for {
		var recv []byte = make([]byte, 10240)
		n, err := self.conn.Read(recv)
		if err != nil {
			self.writ <- true
			self.er <- true
			break
		}
		self.recv <- recv[:n]
	}
}

//把数据发送给browser
func (self browser) write() {
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

//与server相关的conn
type server struct {
	conn net.Conn
	er   chan bool
	writ chan bool
	recv chan []byte
	send chan []byte
}

//读取server过来的数据
func (self *server) read() {
	//isheart与timeout共同判断是不是自己设定的SetReadDeadline
	var isheart bool = false
	//20秒发一次心跳包
	self.conn.SetReadDeadline(time.Now().Add(time.Second * 20))
	for {
		var recv []byte = make([]byte, 10240)
		n, err := self.conn.Read(recv)
		if err != nil {
			if strings.Contains(err.Error(), "timeout") && !isheart {
				self.conn.Write([]byte("hh"))
				//4秒时间收心跳包
				self.conn.SetReadDeadline(time.Now().Add(time.Second * 4))
				isheart = true
				continue
			}
			//浏览器有可能连接上不发消息就断开，此时就发一个0，为了与服务器一直有一条tcp通路
			self.recv <- []byte("0")
			self.er <- true
			self.writ <- true
			break
		}
		//收到心跳包
		if recv[0] == 'h' && recv[1] == 'h' {
			self.conn.SetReadDeadline(time.Now().Add(time.Second * 20))
			isheart = false
			continue
		}
		self.recv <- recv[:n]
	}
}

//把数据发送给server
func (self server) write() {
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
	//开启多核
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()
	//开启守护进程
	if *start && *guard{
		filePath, _ := filepath.Abs(os.Args[0])
		cmd:=exec.Command(filePath,"-start","-r",*remotePort,"-l",*localPort,"-h",*host)
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
	if *localPort=="" || *remotePort=="" || *host==""{
		flag.PrintDefaults()
		os.Exit(1)
	}
	target := net.JoinHostPort(*host, *remotePort)
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
		go handle(server, next)
		<-next
	}
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
		fmt.Printf("出现错误，退出线程： %v\n", err)
		runtime.Goexit()
	}
}

//显示错误并关闭链接，退出线程
func logClose(err error, conn net.Conn) {
	if err != nil {
		fmt.Println("对方已关闭", err)
		runtime.Goexit()
	}
}

//链接端口
func dail(hostport string) net.Conn {
	conn, err := net.Dial("tcp", hostport)
	logExit(err)
	return conn
}

//两个socket衔接相关处理
func handle(server *server, next chan bool) {
	var serverrecv = make([]byte, 10240)
	//阻塞这里等待server传来数据再链接browser
	fmt.Println("等待server发来消息")
	serverrecv = <-server.recv
	//连接上，下一个tcp连上服务器
	next <- true
	//fmt.Println("开始新的tcp链接，发来的消息是：", string(serverrecv))
	var browse *browser
	//server发来数据，链接本地80端口
	fmt.Println("server发来消息了")
	serverconn := dail(*localPort)
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
			server.conn.Close()
			browse.conn.Close()
			runtime.Goexit()
		case <-browse.er:
			server.conn.Close()
			browse.conn.Close()
			runtime.Goexit()
		}
	}
}