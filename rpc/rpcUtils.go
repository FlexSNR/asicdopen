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

package rpc

import (
	"errors"
	"net"
	"syscall"
)

func IPv4AddrStringToU32(ipAddr string) (uint32, error) {
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return 0, errors.New("Invalid IPv4 Address")
	}
	ip = ip.To4()
	parsedIP := uint32(ip[3]) | uint32(ip[2])<<8 | uint32(ip[1])<<16 | uint32(ip[0])<<24
	return parsedIP, nil
}

func IPv6AddrStringToU32(ipAddr string) (uint32, error) {
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return 0, errors.New("Invalid IPv4 Address")
	}
	ip = ip.To16()
	parsedIP := uint32(ip[15]) | uint32(ip[14])<<8 | uint32(ip[13])<<16 | uint32(ip[12])<<24 |
		uint32(ip[11])<<32 | uint32(ip[10])<<40 | uint32(ip[9])<<48 | uint32(ip[8])<<56 |
		uint32(ip[7])<<64 | uint32(ip[6])<<72 | uint32(ip[5])<<80 | uint32(ip[4])<<88 |
		uint32(ip[3])<<96 | uint32(ip[2])<<104 | uint32(ip[1])<<112 | uint32(ip[0])<<120
	return parsedIP, nil
}

func IPAddrStringToU8List(ipAddr string) ([]uint8, []uint8, int, error) {
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return ip, ip, -1, errors.New("Invalid IP Address")
	}
	ipv4 := ip.To4()
	if ipv4 != nil {
		//this is an ip address
		return ip, ipv4, syscall.AF_INET, nil
	}
	ipv6 := ip.To16()
	return ip, ipv6, syscall.AF_INET6, nil
}
