package main

import (
	"asicd/test/thrift"
	"asicdServices"
	"fmt"
	"git.apache.org/thrift.git/lib/go/thrift"
	"os"
	"strconv"
	"utils/ipcutils"
)

type AsicdClient struct {
	Address            string
	Transport          thrift.TTransport
	PtrProtocolFactory *thrift.TBinaryProtocolFactory
	ClientHdl          *asicdServices.ASICDServicesClient
}

var asicdclnt AsicdClient

func ConnectToServer() bool {
	asicdclnt.Address = "localhost:10000"
	asicdclnt.Transport, asicdclnt.PtrProtocolFactory, _ = ipcutils.CreateIPCHandles(asicdclnt.Address)
	if asicdclnt.Transport != nil && asicdclnt.PtrProtocolFactory != nil {
		asicdclnt.ClientHdl = asicdServices.NewASICDServicesClientFactory(asicdclnt.Transport, asicdclnt.PtrProtocolFactory)
		return true
	} else {
		return false
	}
}

func main() {
	rv := ConnectToServer()
	if rv == false {
		fmt.Println("Failed to connect to server. Aborting test suite execution.")
		return
	} else {
		fmt.Println("Successfully connected to server. Commencing test suite execution")
	}
	ops := os.Args[1:]
	if len(ops) == 1 {
		fmt.Println("ops[0]=", ops[0])
		if ops[0] == "scale" {
			if (1) == len(ops) {
				fmt.Println("Incorrect usage: should be ./main scale <number>")
				return
			}
			number, _ := strconv.Atoi(ops[1])
			testThrift.TestScale(asicdclnt.ClientHdl, number)
			return
		}
	}
	fmt.Println("Executing Vlan API test suite")
	rv = testThrift.TestVlanApis(asicdclnt.ClientHdl)
	if rv == false {
		fmt.Println("Vlan API test suite failed !")
	} else {
		fmt.Println("Vlan API test suite passed !")
	}
	fmt.Println("Executing Port API test suite")
	rv = testThrift.TestPortApis(asicdclnt.ClientHdl)
	if rv == false {
		fmt.Println("Port API test suite failed !")
	} else {
		fmt.Println("Port API test suite passed !")
	}
	fmt.Println("Executing IPv4Intf API test suite")
	rv = testThrift.TestIPv4IntfApis(asicdclnt.ClientHdl)
	if rv == false {
		fmt.Println("IPv4Intf API test suite failed !")
	} else {
		fmt.Println("IPv4Intf API test suite passed !")
	}
	fmt.Println("Executing LAG API test suite")
	rv = testThrift.TestLagApis(asicdclnt.ClientHdl)
	if rv == false {
		fmt.Println("Lag API test suite failed !")
	} else {
		fmt.Println("Lag API test suite passed !")
	}
	return
}
