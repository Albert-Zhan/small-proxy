package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"
)

var commands map[string]command
var provider command

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
