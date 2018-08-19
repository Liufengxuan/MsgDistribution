package main

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type Client struct {
	C    chan string
	Name string
	addr string
}

var onlineMap map[string]Client //保存在线用户
var message = make(chan string)

func Manager() {
	onlineMap = make(map[string]Client)
	for {
		msg := <-message // 没有消息前，这里会阻塞。
		//遍历map。给map每个成员都发送此消息
		for _, cli := range onlineMap {
			cli.C <- msg
		}
	}
}
func WriteMsgToClient(cli Client, conn net.Conn) {
	for msg := range cli.C { //给当前客户端发送信息
		conn.Write([]byte(msg))
	}
}
func MakeMsg(cli Client, msg string) (buf string) {
	buf = "[" + cli.addr + "]" + cli.Name + ":" + msg + "\n"
	return buf
}

func HandleConn(conn net.Conn) {
	defer conn.Close()
	//获取client的网络地址
	cliaddr := conn.RemoteAddr().String()

	//创建一个结构体
	cli := Client{make(chan string), cliaddr, cliaddr} //名字默认和网络地址一样。
	onlineMap[cliaddr] = cli

	//新开一个协程，专门给当前客户端发送信息
	go WriteMsgToClient(cli, conn)
	//广播某个人在线
	message <- MakeMsg(cli, "login")
	//message <-"["+cli.addr+"]"+cli.Name+" Is Online"
	//提示我是谁
	message <- MakeMsg(cli, "I am T")

	//新建一个协程，接收用户发送过来的数据
	isQuit := make(chan bool)
	isData := make(chan bool)
	go func() {
		buf := make([]byte, 2048)
		for {
			n, err := conn.Read(buf)
			if n == 0 { //对方断开或者出问题。
				isQuit <- true
				fmt.Println("对方可能断开了连接 。详细信息：conn.Read err=", err)
				return
			}
			msg := string(buf[:n-1]) //减去换行符
			if len(msg) == 8 && msg == "userlist" {
				conn.Write([]byte("user list:\n"))
				for _, tmp := range onlineMap {
					msg := tmp.addr + ":" + tmp.Name + "\n"
					conn.Write([]byte(msg))
				}
			} else if len(msg) >= 8 && msg[:6] == "rename" {
				//rename mike
				name := strings.Split(msg, " ")[1]
				cli.Name = name
				onlineMap[cliaddr] = cli
				conn.Write([]byte("Rename ok"))

			} else {
				//转发此内容
				message <- MakeMsg(cli, msg)
			}
			isData <- true //代表有数据

		}
	}()

	for {
		//通过select检测channal的流动。
		select {
		case <-isQuit:
			delete(onlineMap, cliaddr)
			message <- MakeMsg(cli, "login out") //广播谁下线了。
			return
		case <-isData:
		case <-time.After(30 * time.Second):
			delete(onlineMap, cliaddr)
			message <- MakeMsg(cli, "time out") //广播谁下线了。
			return
		}

	}
}

func main() {
	//监听
	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		fmt.Println("net.Listen err=", err)
		return
	}
	defer listener.Close()

	//新开一个协程，转发消息，只要有消息来了，就遍历map，给map每个成员都发送消息
	go Manager()
	//主协程
	for {
		conn, err1 := listener.Accept()
		if err1 != nil {
			fmt.Println("net.Listen err=", err1)
			continue
		}

		go HandleConn(conn) //处理用户连接
	}
}
