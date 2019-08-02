package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"
)

var commands map[string]command
var provider command

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

func printUsage() {
	fmt.Println("Usage: proxy [command] [parameters], where command is one of:")
	fmt.Print("  ")

	i := 0
	for id := range commands {
		fmt.Print(id)

		if i < len(commands)-1 {
			fmt.Print(", ")
		}
		i++
	}

	fmt.Println(" or help")
	fmt.Println()

	if provider != nil {
		provider.printHelp(" ")
	} else {
		for id, provider := range commands {
			fmt.Printf("  %s\n", id)
			provider.printHelp("   ")
			fmt.Printf("    For more information run \"proxy help %s\"\n", id)
			fmt.Println("")
		}
	}
	flag.PrintDefaults()
}

func help() {
	printUsage()
	os.Exit(2)
}

func loadCommands() {
	commands = make(map[string]command)
	commands["start"]=&start{}
	commands["stop"] = &stop{}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	loadCommands()
	flag.Usage = printUsage

	args := os.Args[1:]
	if len(args) < 1 {
		help()
	}

	command := args[0]
	if command[0] == '-' {
		help()
	}

	if command == "help" {
		if len(args) >= 2 {
			provider = commands[args[1]]
			provider.addFlags()
		}
		help()
	} else if command == "version" {
		fmt.Printf("proxy version %s","0.3.3")
	}else {
		provider = commands[command]
		if provider == nil {
			fmt.Println("Unsupported command", command)
			return
		}

		provider.addFlags()
		if command=="start" {
			_ = flag.CommandLine.Parse(args[1:])
		}
		provider.run(os.Args[2:])
	}
}
