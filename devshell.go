//
//Copyright [2016] [SnapRoute Inc]
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//	 Unless required by applicable law or agreed to in writing, software
//	 distributed under the License is distributed on an "AS IS" BASIS,
//	 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	 See the License for the specific language governing permissions and
//	 limitations under the License.
//
// _______  __       __________   ___      _______.____    __    ____  __  .___________.  ______  __    __
// |   ____||  |     |   ____\  \ /  /     /       |\   \  /  \  /   / |  | |           | /      ||  |  |  |
// |  |__   |  |     |  |__   \  V  /     |   (----` \   \/    \/   /  |  | `---|  |----`|  ,----'|  |__|  |
// |   __|  |  |     |   __|   >   <       \   \      \            /   |  |     |  |     |  |     |   __   |
// |  |     |  `----.|  |____ /  .  \  .----)   |      \    /\    /    |  |     |  |     |  `----.|  |  |  |
// |__|     |_______||_______/__/ \__\ |_______/        \__/  \__/     |__|     |__|      \______||__|  |__|
//

package main

import (
	"asicd/pluginManager"
	"bytes"
	"net"
	"os"
	"syscall"
)

func writer(ch chan<- int, conn *net.TCPConn, writeToDevShell *os.File, devShellNewStdout *os.File) {
	var buf []byte = make([]byte, 256)
	endToken := []byte{0x4}
	for {
		//Read input from tcp connection
		numBytes, err := conn.Read(buf)
		if err != nil {
			//Connection lost, quit
			break
		}
		cmd := bytes.TrimRight(buf[:numBytes], "\r\n")
		//Add termination
		cmd = buf[:len(cmd)+1]
		cmd[len(cmd)-1] = '\n'
		//Write to dev shell input
		_, err = writeToDevShell.Write(cmd)
		if err != nil {
			logger.Err("Failed to send command to driver shell")
		}
	}
	// Send connection end token
	devShellNewStdout.Write(endToken)
	//Signal exit
	ch <- 1
	return
}

func reader(conn *net.TCPConn, readFromDevShell *os.File) {
	var buf []byte = make([]byte, 1024)
	endToken := []byte{0x4}
	for {
		//Read output from dev shell
		numBytes, err := readFromDevShell.Read(buf)
		if err != nil {
			logger.Err("Failed to read response from driver shell")
		}
		// Connection lost, quit
		if numBytes == 1 && endToken[0] == buf[0] {
			break
		}
		//Write to tcp connection
		_, err = conn.Write(buf[:numBytes])
		if err != nil {
			//Connection lost, quit
			break
		}
	}
	return
}

func DevShell(pluginMgr *pluginManager.PluginManager) {
	var ch chan int = make(chan int, 1)
	tcpAddr, err := net.ResolveTCPAddr("tcp", ":40000")
	if err != nil {
		logger.Err("Failed to start device shell")
		return
	}
	ln, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		logger.Err("Failed to start device shell")
		return
	}
	readFromDevShell, devShellNewStdout, err := os.Pipe()
	devShellNewStdin, writeToDevShell, err := os.Pipe()
	err = syscall.Close(int((os.Stdin).Fd()))
	if err != nil {
		logger.Err("Failed to start device shell")
		return
	}
	err = syscall.Dup2(int(devShellNewStdin.Fd()), int((os.Stdin).Fd()))
	if err != nil {
		logger.Err("Failed to start device shell")
		return
	}
	err = syscall.Close(int((os.Stdout).Fd()))
	if err != nil {
		logger.Err("Failed to start device shell")
		return
	}
	err = syscall.Dup2(int(devShellNewStdout.Fd()), int((os.Stdout).Fd()))
	if err != nil {
		logger.Err("Failed to start device shell")
		return
	}
	go pluginMgr.DevShell()
	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			logger.Err("Failed to accept TCP connection")
		}
		go writer(ch, conn, writeToDevShell, devShellNewStdout)
		go reader(conn, readFromDevShell)
		//Read from channel should block until connection is lost
		_ = <-ch
	}
}
