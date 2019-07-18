package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

var start *bool=flag.Bool("start",false,"Start server.")
var stop *bool=flag.Bool("stop",false,"Stop server.")
var guard *bool=flag.Bool("d",false,"Start Daemon.")
var host *string = flag.String("h", "", "Server IP.")
var remotePort *string = flag.String("r", "", "Server Port.")
var localPort *string = flag.String("l", "127.0.0.1:80", "Local IP and Port.")

//与浏览器相关的连接
type browser struct {
	conn net.Conn
	er   chan bool
	writ chan bool
	recv chan []byte
	send chan []byte
}

//读取浏览器发送过来的数据
func (self browser) read() {
	for {
		var recv=make([]byte,10240)
		n, err := self.conn.Read(recv)
		if err != nil {
			self.writ <- true
			self.er <- true
			break
		}
		self.recv <- recv[:n]
	}
}

//把数据发送给浏览器
func (self browser) write() {
	for {
		var send=make([]byte, 10240)
		select {
		case send = <-self.send:
			_, _ = self.conn.Write(send)
		case <-self.writ:
			break
		}
	}
}

//与服务端相关的连接
type server struct {
	conn net.Conn
	er   chan bool
	writ chan bool
	recv chan []byte
	send chan []byte
}

//读取服务端发送过来的数据
func (self *server) read() {
	//isheart与timeout共同判断是不是自己设定的SetReadDeadline
	var isheart=false
	//20秒发一次心跳包
	_ = self.conn.SetReadDeadline(time.Now().Add(20*time.Second))
	for {
		var recv=make([]byte,10240)
		n, err := self.conn.Read(recv)
		if err != nil {
			if strings.Contains(err.Error(), "timeout") && !isheart {
				_, _ = self.conn.Write([]byte("hh"))
				//4秒时间收心跳包
				_ = self.conn.SetReadDeadline(time.Now().Add(4*time.Second))
				isheart = true
				continue
			}
			//浏览器有可能连接上不发消息就断开，此时就发一个0，为了与服务器一直有一条tcp通道
			self.recv <- []byte("0")
			self.er <- true
			self.writ <- true
			break
		}
		//收到心跳包
		if recv[0] == 'h' && recv[1] == 'h' {
			_ = self.conn.SetReadDeadline(time.Now().Add(time.Second * 20))
			isheart = false
			continue
		}
		self.recv <- recv[:n]
	}
}

//把数据发送给服务端
func (self server) write() {
	for {
		var send=make([]byte, 10240)
		select {
		case send = <-self.send:
			_, _ = self.conn.Write(send)
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
		cmd:=exec.Command(os.Args[0],"-start","-r",*remotePort,"-l",*localPort,"-h",*host)
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
	fmt.Println("Please input parameters.")
}

//开启内网穿透服务
func startServer() {
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
		go handle(server,next)
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
	var serverrecv=make([]byte,10240)
	//阻塞这里等待服务端传来数据再链接浏览器
	fmt.Println("等待server发来消息")
	serverrecv = <-server.recv
	next <- true
	var browse *browser
	//服务端发来数据，链接本地80端口
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
