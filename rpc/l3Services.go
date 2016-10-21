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

// This file defines all interfaces provided for the L3 service
package rpc

import (
	"asicd/asicdCommonDefs"
	"asicdInt"
	"asicdServices"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"syscall"
	"utils/commonDefs"
	netUtils "utils/netUtils"
)

//Logical Intf related services
func (svcHdlr AsicDaemonServiceHandler) CreateLogicalIntf(confObj *asicdServices.LogicalIntf) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to create logical intf - ", confObj))
	rv, err = svcHdlr.pluginMgr.CreateLogicalIntfConfig(confObj)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift create logical intf returning - ", rv, err))
	return rv, err
}
func (svcHdlr AsicDaemonServiceHandler) UpdateLogicalIntf(oldConfIntfObj, newConfIntfObj *asicdServices.LogicalIntf, attrset []bool, op []*asicdServices.PatchOpInfo) (bool, error) {
	return false, errors.New("Update operation is not supported on LogicalIntf object")
}
func (svcHdlr AsicDaemonServiceHandler) DeleteLogicalIntf(confObj *asicdServices.LogicalIntf) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to delete logical intf - ", confObj))
	rv, err = svcHdlr.pluginMgr.DeleteLogicalIntfConfig(confObj)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift delete logical intf returning - ", rv, err))
	return rv, err
}
func (svcHdlr AsicDaemonServiceHandler) GetBulkLogicalIntfState(currMarker, count asicdServices.Int) (*asicdServices.LogicalIntfStateGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received for getbulk logical intf")
	end, listLen, more, liIntfList := svcHdlr.pluginMgr.GetBulkLogicalIntfState(int(currMarker), int(count))
	if end < 0 {
		return nil, errors.New("Failed to retrieve logicalIntf information")
	}
	bulkObj := asicdServices.NewLogicalIntfStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.LogicalIntfStateList = liIntfList
	return bulkObj, nil
}

func ipValidator(ipAddr string, ipType int) (bool, error) {
	switch ipType {
	case syscall.AF_INET:
		// check whether it is ipv4 address only
		if netUtils.IsIPv4Addr(ipAddr) {
			return true, nil
		}

		return false, errors.New(fmt.Sprintln("During ipv4 operation, ipv6 address:", ipAddr, "is not allowed"))

	case syscall.AF_INET6:
		// check whether it is ipv6 address only
		if netUtils.IsIPv6Addr(ipAddr) {
			return true, nil
		}
		return false, errors.New(fmt.Sprintln("During ipv6 operation, ipv4 address:", ipAddr, "is not allowed"))

	}
	return false, errors.New(fmt.Sprintln("invalid ip address:", ipAddr, " passed in"))
}

func (svcHdlr AsicDaemonServiceHandler) GetLogicalIntfState(name string) (*asicdServices.LogicalIntfState, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get logical intf - ", name))
	return svcHdlr.pluginMgr.GetLogicalIntfState(name)
}

//IPv4 interface related services
func (svcHdlr AsicDaemonServiceHandler) CreateIPv4Intf(ipv4IntfObj *asicdServices.IPv4Intf) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received for create ipv4intf - ", ipv4IntfObj))
	rv, err = ipValidator(ipv4IntfObj.IpAddr, syscall.AF_INET)
	if rv {
		rv, err = svcHdlr.pluginMgr.CreateIPIntf(ipv4IntfObj.IpAddr, ipv4IntfObj.IntfRef, ipv4IntfObj.AdminState)
	}
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift create ipv4intf returning - ", rv, err))
	return rv, err
}
func (svcHdlr AsicDaemonServiceHandler) UpdateIPv4Intf(oldIPv4IntfObj, newIPv4IntfObj *asicdServices.IPv4Intf,
	attrset []bool, op []*asicdServices.PatchOpInfo) (bool, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received for update ipv4intf - ", oldIPv4IntfObj, newIPv4IntfObj))
	rv, err := ipValidator(oldIPv4IntfObj.IpAddr, syscall.AF_INET)
	if rv != true {
		return rv, err
	}
	rv, err = ipValidator(newIPv4IntfObj.IpAddr, syscall.AF_INET)
	if rv != true {
		return rv, err
	}
	_, err = svcHdlr.pluginMgr.UpdateIPIntf(oldIPv4IntfObj.IpAddr, oldIPv4IntfObj.IntfRef, newIPv4IntfObj.IpAddr, newIPv4IntfObj.IntfRef, newIPv4IntfObj.AdminState)
	if err != nil {
		return false, err
	}
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift update ipv4intf returning - ", err))
	return true, nil
}
func (svcHdlr AsicDaemonServiceHandler) DeleteIPv4Intf(ipv4IntfObj *asicdServices.IPv4Intf) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received for delete ipv4intf - ", ipv4IntfObj))
	rv, err = ipValidator(ipv4IntfObj.IpAddr, syscall.AF_INET)
	if rv != true {
		return rv, err
	}
	rv, err = svcHdlr.pluginMgr.DeleteIPIntf(ipv4IntfObj.IpAddr, ipv4IntfObj.IntfRef)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift delete ipv4intf returning - ", rv, err))
	return rv, err
}
func (svcHdlr AsicDaemonServiceHandler) GetIPv4IntfState(intfRef string) (*asicdServices.IPv4IntfState, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get ipv4 intf state for intf - ", intfRef))
	ipv4IntfState, err := svcHdlr.pluginMgr.GetIPv4IntfState(intfRef)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift get ipv4intf for ifindex returning - ", ipv4IntfState, err))
	return ipv4IntfState, err
}
func (svcHdlr AsicDaemonServiceHandler) GetBulkIPv4IntfState(currMarker, count asicdServices.Int) (*asicdServices.IPv4IntfStateGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received to getbulk ipv4intfstate")
	end, listLen, more, ipv4IntfList := svcHdlr.pluginMgr.GetBulkIPv4IntfState(int(currMarker), int(count))
	if end < 0 {
		return nil, errors.New("Failed to retrieve IPv4Intf state information")
	}
	bulkObj := asicdServices.NewIPv4IntfStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.IPv4IntfStateList = ipv4IntfList
	return bulkObj, nil
}

//IPv4 Neighbor related services
func (svcHdlr AsicDaemonServiceHandler) CreateIPv4Neighbor(ipAddr string, macAddr string, vlanId, ifIndex int32) (rval int32, err error) {
	svcHdlr.logger.Debug("Thrift request received for create ipv4 neighbor - ", ipAddr, macAddr, vlanId, ifIndex)
	parsedMacAddr, err := net.ParseMAC(macAddr)
	if err != nil {
		return -1, errors.New("Invalid MAC Address")
	}
	rv, err := ipValidator(ipAddr, syscall.AF_INET)
	if rv != true {
		return -1, err
	}
	err = svcHdlr.pluginMgr.CreateIPNeighbor(ipAddr, int(vlanId), ifIndex, parsedMacAddr)
	svcHdlr.logger.Debug("Thrift create ipv4 neighbor returning - ", err)
	return (int32)(0), err
}
func (svcHdlr AsicDaemonServiceHandler) UpdateIPv4Neighbor(ipAddr string, macAddr string, vlanId, ifIndex int32) (rval int32, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to update ipv4 neighbor - ", ipAddr, macAddr, vlanId, ifIndex))
	parsedMacAddr, err := net.ParseMAC(macAddr)
	if err != nil {
		return -1, errors.New("Invalid MAC Address")
	}
	rv, err := ipValidator(ipAddr, syscall.AF_INET)
	if rv != true {
		return -1, err
	}
	err = svcHdlr.pluginMgr.UpdateIPNeighbor(ipAddr, int(vlanId), ifIndex, parsedMacAddr)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift update ipv4 neighbor returning - ", err))
	return (int32)(0), err
}
func (svcHdlr AsicDaemonServiceHandler) DeleteIPv4Neighbor(ipAddr string, macAddr string, vlanId, ifIndex int32) (rval int32, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to delete ipv4 neighbor - ", ipAddr, macAddr, vlanId, ifIndex))
	parsedMacAddr, err := net.ParseMAC(macAddr)
	if err != nil {
		return -1, errors.New("Invalid MAC Address")
	}
	rv, err := ipValidator(ipAddr, syscall.AF_INET)
	if rv != true {
		return -1, err
	}
	err = svcHdlr.pluginMgr.DeleteIPNeighbor(ipAddr, int(vlanId), ifIndex, parsedMacAddr)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift delete ipv4 neighbor returning - ", err))
	return (int32)(0), err
}
func (svcHdlr AsicDaemonServiceHandler) GetBulkArpEntryHwState(currMarker, count asicdServices.Int) (*asicdServices.ArpEntryHwStateGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received to getbulk arp hwstate")
	end, listLen, more, ipv4NbrList := svcHdlr.pluginMgr.GetBulkIPv4Neighbor(int(currMarker), int(count))
	if end < 0 {
		return nil, errors.New("Failed to retrieve arp hw state information")
	}
	bulkObj := asicdServices.NewArpEntryHwStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.ArpEntryHwStateList = ipv4NbrList
	return bulkObj, nil
}
func (svcHdlr AsicDaemonServiceHandler) GetArpEntryHwState(ipAddr string) (*asicdServices.ArpEntryHwState, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get arp entry hw state - ", ipAddr))
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return nil, errors.New("Invalid Neighbor IP Address in get request")
	}
	ip = ip.To4()
	parsedIP := uint32(ip[3]) | uint32(ip[2])<<8 | uint32(ip[1])<<16 | uint32(ip[0])<<24
	return svcHdlr.pluginMgr.GetIPv4Neighbor(parsedIP)
}

//IPv4 Route related services
func (svcHdlr AsicDaemonServiceHandler) OnewayCreateIPv4Route(ipv4RouteList []*asicdInt.IPv4Route) error {
	var failedRoutes []*asicdInt.IPv4Route
	for idx := 0; idx < len(ipv4RouteList); idx++ {
		svcHdlr.logger.Debug("Thrift request received to create ipv4 route - ", ipv4RouteList[idx].DestinationNw, ipv4RouteList[idx].NetworkMask)
		_, parsedPrefixIP, _, err := IPAddrStringToU8List(ipv4RouteList[idx].DestinationNw)
		if err != nil {
			svcHdlr.logger.Err("Error:", err, " getting parsedPrefixIP")
			failedRoutes = append(failedRoutes, ipv4RouteList[idx])
			continue
		}
		svcHdlr.logger.Debug("parsedPrefixIP:", parsedPrefixIP, " err:", err)
		maskIP, parsedPrefixMask, _, err := IPAddrStringToU8List(ipv4RouteList[idx].NetworkMask)
		if err != nil {
			svcHdlr.logger.Err("Error:", err, " getting parsedPrefixMask")
			failedRoutes = append(failedRoutes, ipv4RouteList[idx])
			continue
		}
		prefixLen, err := netUtils.GetPrefixLen(maskIP)
		if err != nil {
			svcHdlr.logger.Err("Error:", err, " getting prefixLen for mask:", maskIP)
			failedRoutes = append(failedRoutes, ipv4RouteList[idx])
			continue
		}
		nwAddr := ((net.IP(parsedPrefixIP)).Mask(net.IPMask(parsedPrefixMask))).String() + "/" + strconv.Itoa(prefixLen)
		svcHdlr.logger.Debug("parsedPrefixMask:", parsedPrefixMask, " err:", err, " maskIP:", maskIP, "prefixLen:", prefixLen, " nwAddr:", nwAddr)
		for idx1 := 0; idx1 < len(ipv4RouteList[idx].NextHopList); idx1++ {
			_, parsedNextHopIp, nhIpType, err := IPAddrStringToU8List(ipv4RouteList[idx].NextHopList[idx1].NextHopIp)
			if err != nil {
				svcHdlr.logger.Err("Error:", err, " getting parsedNextHopIp")
				failedRoutes = append(failedRoutes, ipv4RouteList[idx])
				break
			}
			if ipv4RouteList[idx].NextHopList[idx1].NextHopIp == "255.255.255.255" {
				ipv4RouteList[idx].NextHopList[idx1].NextHopIfType = commonDefs.IfTypeNull
			}
			svcHdlr.logger.Debug("parsedNextHopIp:", parsedNextHopIp, "nhIpType:", nhIpType, " err:", err)
			svcHdlr.logger.Debug("calling createiproute for nwaddr:", nwAddr)
			/*			err = svcHdlr.pluginMgr.CreateIPRoute(nwAddr, parsedPrefixIP, parsedPrefixMask, parsedNextHopIp,
							syscall.AF_INET, nhIpType, int(ipv4RouteList[idx].NextHopList[idx1].Weight),
							int(ipv4RouteList[idx].NextHopList[idx1].NextHopIfType))
						if err != nil {
							svcHdlr.logger.Err("Error:", err, " creating route")
							failedRoutes = append(failedRoutes, ipv4RouteList[idx])
							break
						}*/
			svcHdlr.pluginMgr.CreateIPRoute(nwAddr, parsedPrefixIP, parsedPrefixMask, parsedNextHopIp,
				syscall.AF_INET, nhIpType, int(ipv4RouteList[idx].NextHopList[idx1].Weight),
				int(ipv4RouteList[idx].NextHopList[idx1].NextHopIfType))
		}
	}
	//Notify route add failures
	if len(failedRoutes) != 0 {
		svcHdlr.logger.Debug("non zero failed routes")
		msgBuf, err := json.Marshal(failedRoutes)
		if err != nil {
			return errors.New("Failed to marshal route create failure message")
		}
		notification := asicdCommonDefs.AsicdNotification{
			MsgType: uint8(asicdCommonDefs.NOTIFY_IPV4_ROUTE_CREATE_FAILURE),
			Msg:     msgBuf,
		}
		notificationBuf, err := json.Marshal(notification)
		if err != nil {
			return errors.New("Failed to marshal route create failure message")
		}
		svcHdlr.notifyChan.Ribd <- notificationBuf
	}
	return nil
}
func (svcHdlr AsicDaemonServiceHandler) OnewayDeleteIPv4Route(ipv4RouteList []*asicdInt.IPv4Route) error {
	var failedRoutes []*asicdInt.IPv4Route
	for idx := 0; idx < len(ipv4RouteList); idx++ {
		svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to delete ipv4 route - ", ipv4RouteList[idx].DestinationNw, ipv4RouteList[idx].NetworkMask))
		_, parsedPrefixIP, _, err := IPAddrStringToU8List(ipv4RouteList[idx].DestinationNw)
		if err != nil {
			svcHdlr.logger.Err(fmt.Sprintln("Error when getting parsedPrefixIP, err:", err))
			failedRoutes = append(failedRoutes, ipv4RouteList[idx])
			continue
		}
		maskIP, parsedPrefixMask, _, err := IPAddrStringToU8List(ipv4RouteList[idx].NetworkMask)
		if err != nil {
			svcHdlr.logger.Err(fmt.Sprintln("Error when getting parsedPrefixMask, err:", err))
			failedRoutes = append(failedRoutes, ipv4RouteList[idx])
			continue
		}
		prefixLen, err := netUtils.GetPrefixLen(maskIP)
		if err != nil {
			svcHdlr.logger.Err(fmt.Sprintln("Error:", err, " getting prefixLen for mask:", maskIP))
			continue
		}
		nwAddr := ((net.IP(parsedPrefixIP)).Mask(net.IPMask(parsedPrefixMask))).String() + "/" + strconv.Itoa(prefixLen)
		for idx1 := 0; idx1 < len(ipv4RouteList[idx].NextHopList); idx1++ {
			_, parsedNextHopIp, nhIpType, err := IPAddrStringToU8List(ipv4RouteList[idx].NextHopList[idx1].NextHopIp)
			if err != nil {
				svcHdlr.logger.Err(fmt.Sprintln("Error when getting parsedNextHopIP, err:", err))
				failedRoutes = append(failedRoutes, ipv4RouteList[idx])
				break
			}
			if ipv4RouteList[idx].NextHopList[idx1].NextHopIp == "255.255.255.255" {
				svcHdlr.logger.Debug("OnewatDeleteIPv4Route: setting nexthopiftype to commondefs.iftypenull")
				ipv4RouteList[idx].NextHopList[idx1].NextHopIfType = commonDefs.IfTypeNull
			}
			/*err = svcHdlr.pluginMgr.DeleteIPRoute(nwAddr, parsedPrefixIP, parsedPrefixMask, parsedNextHopIp,
				syscall.AF_INET, nhIpType, int(ipv4RouteList[idx].NextHopList[idx1].NextHopIfType))
			if err != nil {
				svcHdlr.logger.Err(fmt.Sprintln("Error when calling deleteIPRoute, err:", err))
				failedRoutes = append(failedRoutes, ipv4RouteList[idx])
				break
			}*/
			svcHdlr.pluginMgr.DeleteIPRoute(nwAddr, parsedPrefixIP, parsedPrefixMask, parsedNextHopIp,
				syscall.AF_INET, nhIpType, int(ipv4RouteList[idx].NextHopList[idx1].NextHopIfType))
		}
	}
	//Notify route delete failures
	if len(failedRoutes) != 0 {
		msgBuf, err := json.Marshal(failedRoutes)
		if err != nil {
			return errors.New("Failed to marshal route delete failure message")
		}
		notification := asicdCommonDefs.AsicdNotification{
			MsgType: uint8(asicdCommonDefs.NOTIFY_IPV4_ROUTE_DELETE_FAILURE),
			Msg:     msgBuf,
		}
		notificationBuf, err := json.Marshal(notification)
		if err != nil {
			return errors.New("Failed to marshal route delete failure message")
		}
		svcHdlr.notifyChan.Ribd <- notificationBuf
	}
	return nil
}

//IPv6 Route related services
func (svcHdlr AsicDaemonServiceHandler) OnewayCreateIPv6Route(ipv6RouteList []*asicdInt.IPv6Route) error {
	var failedRoutes []*asicdInt.IPv6Route
	//svcHdlr.logger.Debug("In OnewayCreateIPv6Route")
	for idx := 0; idx < len(ipv6RouteList); idx++ {
		//	svcHdlr.logger.Debug("Thrift request received to create ipv6 route - ", ipv6RouteList[idx].DestinationNw, ipv6RouteList[idx].NetworkMask)
		_, parsedPrefixIP, _, err := IPAddrStringToU8List(ipv6RouteList[idx].DestinationNw)
		if err != nil {
			svcHdlr.logger.Err(fmt.Sprintln("err not nil while parsing dest ip: ", ipv6RouteList[idx].DestinationNw, " err: ", err))
			failedRoutes = append(failedRoutes, ipv6RouteList[idx])
			continue
		}
		maskIP, parsedPrefixMask, _, err := IPAddrStringToU8List(ipv6RouteList[idx].NetworkMask)
		if err != nil {
			svcHdlr.logger.Err(fmt.Sprintln("err not nil while parsing mask: ", ipv6RouteList[idx].NetworkMask, " err: ", err))
			failedRoutes = append(failedRoutes, ipv6RouteList[idx])
			continue
		}
		prefixLen, err := netUtils.GetPrefixLen(maskIP)
		if err != nil {
			svcHdlr.logger.Err(fmt.Sprintln("Error:", err, " getting prefixLen for mask:", maskIP))
			failedRoutes = append(failedRoutes, ipv6RouteList[idx])
			continue
		}
		nwAddr := ((net.IP(parsedPrefixIP)).Mask(net.IPMask(parsedPrefixMask))).String() + "/" + strconv.Itoa(prefixLen)
		for idx1 := 0; idx1 < len(ipv6RouteList[idx].NextHopList); idx1++ {

			_, parsedNextHopIp, nhIpType, err := IPAddrStringToU8List(ipv6RouteList[idx].NextHopList[idx1].NextHopIp)
			if err != nil {
				svcHdlr.logger.Err(fmt.Sprintln("err not nil while parsing next op ip: ",
					ipv6RouteList[idx].NextHopList[idx1].NextHopIp, " err: ", err))
				failedRoutes = append(failedRoutes, ipv6RouteList[idx])
				break
			}
			if ipv6RouteList[idx].NextHopList[idx1].NextHopIp == "255.255.255.255" {
				ipv6RouteList[idx].NextHopList[idx1].NextHopIfType = commonDefs.IfTypeNull
			}
			/*			err = svcHdlr.pluginMgr.CreateIPRoute(nwAddr, parsedPrefixIP, parsedPrefixMask, parsedNextHopIp,
							syscall.AF_INET6, nhIpType, int(ipv6RouteList[idx].NextHopList[idx1].Weight),
							int(ipv6RouteList[idx].NextHopList[idx1].NextHopIfType))
						if err != nil {
							svcHdlr.logger.Err(fmt.Sprintln("err not nil after pluginMgr.CreateIPv6Route call err: ", err))
							failedRoutes = append(failedRoutes, ipv6RouteList[idx])
							break
						}*/
			svcHdlr.pluginMgr.CreateIPRoute(nwAddr, parsedPrefixIP, parsedPrefixMask, parsedNextHopIp,
				syscall.AF_INET6, nhIpType, int(ipv6RouteList[idx].NextHopList[idx1].Weight),
				int(ipv6RouteList[idx].NextHopList[idx1].NextHopIfType))
		}
	}
	//Notify route add failures
	if len(failedRoutes) != 0 {
		msgBuf, err := json.Marshal(failedRoutes)
		if err != nil {
			return errors.New("Failed to marshal route create failure message")
		}
		notification := asicdCommonDefs.AsicdNotification{
			MsgType: uint8(asicdCommonDefs.NOTIFY_IPV6_ROUTE_CREATE_FAILURE),
			Msg:     msgBuf,
		}
		notificationBuf, err := json.Marshal(notification)
		if err != nil {
			return errors.New("Failed to marshal route create failure message")
		}
		svcHdlr.notifyChan.Ribd <- notificationBuf
	}
	return nil
}
func (svcHdlr AsicDaemonServiceHandler) OnewayDeleteIPv6Route(ipv6RouteList []*asicdInt.IPv6Route) error {
	var failedRoutes []*asicdInt.IPv6Route
	for idx := 0; idx < len(ipv6RouteList); idx++ {
		svcHdlr.logger.Debug("Thrift request received to delete ipv6 route - ", ipv6RouteList[idx].DestinationNw, ipv6RouteList[idx].NetworkMask)
		_, parsedPrefixIP, _, err := IPAddrStringToU8List(ipv6RouteList[idx].DestinationNw)
		if err != nil {
			failedRoutes = append(failedRoutes, ipv6RouteList[idx])
			continue
		}
		maskIP, parsedPrefixMask, _, err := IPAddrStringToU8List(ipv6RouteList[idx].NetworkMask)
		if err != nil {
			failedRoutes = append(failedRoutes, ipv6RouteList[idx])
			continue
		}
		prefixLen, err := netUtils.GetPrefixLen(maskIP)
		if err != nil {
			svcHdlr.logger.Err(fmt.Sprintln("Error:", err, " getting prefixLen for mask:", maskIP))
			continue
		}
		nwAddr := ((net.IP(parsedPrefixIP)).Mask(net.IPMask(parsedPrefixMask))).String() + "/" + strconv.Itoa(prefixLen)
		for idx1 := 0; idx1 < len(ipv6RouteList[idx].NextHopList); idx1++ {
			_, parsedNextHopIp, nhIpType, err := IPAddrStringToU8List(ipv6RouteList[idx].NextHopList[idx1].NextHopIp)
			if err != nil {
				failedRoutes = append(failedRoutes, ipv6RouteList[idx])
				break
			}
			if ipv6RouteList[idx].NextHopList[idx1].NextHopIp == "255.255.255.255" {
				ipv6RouteList[idx].NextHopList[idx1].NextHopIfType = commonDefs.IfTypeNull
			}
			/*err = svcHdlr.pluginMgr.DeleteIPRoute(nwAddr, parsedPrefixIP, parsedPrefixMask, parsedNextHopIp,
				syscall.AF_INET6, nhIpType, int(ipv6RouteList[idx].NextHopList[idx1].NextHopIfType))
			if err != nil {
				failedRoutes = append(failedRoutes, ipv6RouteList[idx])
				break
			}*/
			svcHdlr.pluginMgr.DeleteIPRoute(nwAddr, parsedPrefixIP, parsedPrefixMask, parsedNextHopIp,
				syscall.AF_INET6, nhIpType, int(ipv6RouteList[idx].NextHopList[idx1].NextHopIfType))
		}
	}
	//Notify route delete failures
	if len(failedRoutes) != 0 {
		msgBuf, err := json.Marshal(failedRoutes)
		if err != nil {
			return errors.New("Failed to marshal route delete failure message")
		}
		notification := asicdCommonDefs.AsicdNotification{
			MsgType: uint8(asicdCommonDefs.NOTIFY_IPV6_ROUTE_DELETE_FAILURE),
			Msg:     msgBuf,
		}
		notificationBuf, err := json.Marshal(notification)
		if err != nil {
			return errors.New("Failed to marshal route delete failure message")
		}
		svcHdlr.notifyChan.Ribd <- notificationBuf
	}
	return nil
}
func (svcHdlr AsicDaemonServiceHandler) GetBulkIPRouteHwState(currMarker, count asicdInt.Int) (*asicdInt.IPRouteHwStateGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received to getbulk iproute hwstate")
	end, listLen, more, ipRouteList := svcHdlr.pluginMgr.GetBulkIPRoute(int(currMarker), int(count))
	if end < 0 {
		return nil, errors.New("Failed to retrieve IP route hw state information")
	}
	bulkObj := asicdInt.NewIPRouteHwStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdInt.Int(listLen)
	bulkObj.StartIdx = asicdInt.Int(currMarker)
	bulkObj.EndIdx = asicdInt.Int(end)
	bulkObj.IPRouteHwStateList = ipRouteList
	return bulkObj, nil
}
func (svcHdlr AsicDaemonServiceHandler) GetIPRouteHwState(destinationNw string) (*asicdInt.IPRouteHwState, error) {
	svcHdlr.logger.Debug("Thrift request received to get ip route - ", destinationNw)
	return svcHdlr.pluginMgr.GetIPRoute(destinationNw)
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkIPv4RouteHwState(currMarker, count asicdServices.Int) (*asicdServices.IPv4RouteHwStateGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received to getbulk ipv4route hwstate")
	end, listLen, more, ipRouteList := svcHdlr.pluginMgr.GetBulkIPv4Route(int(currMarker), int(count))
	if end < 0 {
		return nil, errors.New("Failed to retrieve IP v4 route hw state information")
	}
	bulkObj := asicdServices.NewIPv4RouteHwStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.IPv4RouteHwStateList = ipRouteList
	return bulkObj, nil
}
func (svcHdlr AsicDaemonServiceHandler) GetIPv4RouteHwState(destinationNw string) (*asicdServices.IPv4RouteHwState, error) {
	svcHdlr.logger.Debug("Thrift request received to get ip v4 route - ", destinationNw)
	return svcHdlr.pluginMgr.GetIPv4Route(destinationNw)
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkIPv6RouteHwState(currMarker, count asicdServices.Int) (*asicdServices.IPv6RouteHwStateGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received to getbulk ipv6route hwstate")
	end, listLen, more, ipRouteList := svcHdlr.pluginMgr.GetBulkIPv6Route(int(currMarker), int(count))
	if end < 0 {
		return nil, errors.New("Failed to retrieve IPf6 route hw state information")
	}
	bulkObj := asicdServices.NewIPv6RouteHwStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.IPv6RouteHwStateList = ipRouteList
	return bulkObj, nil
}
func (svcHdlr AsicDaemonServiceHandler) GetIPv6RouteHwState(destinationNw string) (*asicdServices.IPv6RouteHwState, error) {
	svcHdlr.logger.Debug("Thrift request received to get ipv6 route - ", destinationNw)
	return svcHdlr.pluginMgr.GetIPv6Route(destinationNw)
}

func (svcHdlr AsicDaemonServiceHandler) CreateSubIPv4Intf(subipv4IntfObj *asicdServices.SubIPv4Intf) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received for create sub ipv4intf - ", subipv4IntfObj))
	rv, err = ipValidator(subipv4IntfObj.IpAddr, syscall.AF_INET)
	if rv != true {
		return rv, err
	}
	rv, err = svcHdlr.pluginMgr.CreateSubIPv4Intf(subipv4IntfObj)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift create sub ipv4intf returning - ", rv, err))
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) UpdateSubIPv4Intf(oldSubIPv4IntfObj,
	newSubIPv4IntfObj *asicdServices.SubIPv4Intf, attrset []bool, op []*asicdServices.PatchOpInfo) (rv bool, err error) {
	rv, err = ipValidator(oldSubIPv4IntfObj.IpAddr, syscall.AF_INET)
	if rv != true {
		return rv, err
	}
	rv, err = ipValidator(newSubIPv4IntfObj.IpAddr, syscall.AF_INET)
	if rv != true {
		return rv, err
	}
	rv, err = svcHdlr.pluginMgr.UpdateSubIPv4Intf(oldSubIPv4IntfObj, newSubIPv4IntfObj, attrset)
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) DeleteSubIPv4Intf(subipv4IntfObj *asicdServices.SubIPv4Intf) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received for delete ipv4intf - ", subipv4IntfObj))
	rv, err = ipValidator(subipv4IntfObj.IpAddr, syscall.AF_INET)
	if rv != true {
		return rv, err
	}
	rv, err = svcHdlr.pluginMgr.DeleteSubIPv4Intf(subipv4IntfObj)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift delete ipv4intf returning - ", rv, err))
	return rv, err
}

// IPv6 Interface Create/Update/Delete API's
func (svcHdlr AsicDaemonServiceHandler) CreateIPv6Intf(ipv6IntfObj *asicdServices.IPv6Intf) (rv bool, err error) {
	svcHdlr.logger.Debug("Thrift request received for create ipv6intf - ", ipv6IntfObj)
	// do global scope create only if the ip address is specified
	if ipv6IntfObj.IpAddr != "" {
		rv, err = ipValidator(ipv6IntfObj.IpAddr, syscall.AF_INET6)
		if rv != true {
			return rv, err
		}
		rv, err = svcHdlr.pluginMgr.CreateIPIntf(ipv6IntfObj.IpAddr, ipv6IntfObj.IntfRef, ipv6IntfObj.AdminState)
		if err != nil {
			svcHdlr.logger.Err("CreateIpIntf failed with err:", err)
			return rv, err
		}
	}
	// do link local create only if the link-local bit is set to true
	if ipv6IntfObj.LinkIp == true {
		rv, err = svcHdlr.pluginMgr.CreateIPv6LinkLocalIP(ipv6IntfObj.IntfRef)
		if err != nil {
			// then delete global scope ip address if created
			if ipv6IntfObj.IpAddr != "" {
				rv1, err1 := svcHdlr.pluginMgr.DeleteIPIntf(ipv6IntfObj.IpAddr, ipv6IntfObj.IntfRef)
				return rv1, err1
			}
		}
	}

	svcHdlr.logger.Debug("Thrift create ipv6intf returning - ", rv, err)
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) UpdateIPv6Intf(oldIPv6IntfObj, newIPv6IntfObj *asicdServices.IPv6Intf, attrset []bool,
	op []*asicdServices.PatchOpInfo) (rv bool, err error) {
	svcHdlr.logger.Debug("Thrift request received for update ipv6intf - ", oldIPv6IntfObj, newIPv6IntfObj)
	if (oldIPv6IntfObj.IpAddr != newIPv6IntfObj.IpAddr) || (oldIPv6IntfObj.LinkIp != newIPv6IntfObj.LinkIp) {
		if oldIPv6IntfObj.IpAddr != "" {
			rv, err := ipValidator(oldIPv6IntfObj.IpAddr, syscall.AF_INET6)
			if rv != true {
				return rv, err
			}
			_, err = svcHdlr.pluginMgr.DeleteIPIntf(oldIPv6IntfObj.IpAddr, oldIPv6IntfObj.IntfRef)
			if err != nil {
				return false, err
			}
		}
		// do link local create only if the link-local bit is set to true
		if oldIPv6IntfObj.LinkIp == true {
			rv, err = svcHdlr.pluginMgr.DeleteIPv6LinkLocalIP(oldIPv6IntfObj.IntfRef)
			if err != nil {
				svcHdlr.logger.Err("need to impelement this case where delete of global went through",
					"but delete of local failed on Update IPv6Intf", oldIPv6IntfObj.IpAddr, "to",
					newIPv6IntfObj.IpAddr)
			}
		}
		if newIPv6IntfObj.IpAddr != "" {
			rv, err = ipValidator(newIPv6IntfObj.IpAddr, syscall.AF_INET6)
			if rv != true {
				return rv, err
			}
			_, err = svcHdlr.pluginMgr.CreateIPIntf(newIPv6IntfObj.IpAddr, newIPv6IntfObj.IntfRef, newIPv6IntfObj.AdminState)
			if err != nil {
				svcHdlr.logger.Debug("Error creating ipIntf,err:", err)
				return false, err
			}
		}
		if newIPv6IntfObj.LinkIp == true {
			rv, err = svcHdlr.pluginMgr.CreateIPv6LinkLocalIP(newIPv6IntfObj.IntfRef)
			if err != nil {
				// then delete global scope ip address if created
				if newIPv6IntfObj.IpAddr != "" {
					rv1, err1 := svcHdlr.pluginMgr.DeleteIPIntf(newIPv6IntfObj.IpAddr, newIPv6IntfObj.IntfRef)
					return rv1, err1
				}
			}
		}
	} else {
		//This is an Adminstate only change
		_, err = svcHdlr.pluginMgr.UpdateIPIntf(oldIPv6IntfObj.IpAddr, oldIPv6IntfObj.IntfRef, newIPv6IntfObj.IpAddr, newIPv6IntfObj.IntfRef, newIPv6IntfObj.AdminState)
		if err != nil {
			svcHdlr.logger.Debug("Error updating ipv6Intf,err:", err)
			return false, err
		}
		if newIPv6IntfObj.LinkIp == true {
			_, err = svcHdlr.pluginMgr.UpdateIPv6LinkLocalIP(newIPv6IntfObj.IntfRef, newIPv6IntfObj.AdminState)
			if err != nil {
				svcHdlr.logger.Debug("Error updating ipv6Intf link local IP,err:", err)
				return false, err
			}
		}
	}
	svcHdlr.logger.Debug("Thrift update ipv6intf returning - ", err)
	return true, nil
}

func (svcHdlr AsicDaemonServiceHandler) DeleteIPv6Intf(ipv6IntfObj *asicdServices.IPv6Intf) (rv bool, err error) {
	svcHdlr.logger.Debug("Thrift request received for delete ipv6intf - ", ipv6IntfObj)
	if ipv6IntfObj.IpAddr != "" {
		rv, err = ipValidator(ipv6IntfObj.IpAddr, syscall.AF_INET6)
		if rv != true {
			return rv, err
		}
		rv, err = svcHdlr.pluginMgr.DeleteIPIntf(ipv6IntfObj.IpAddr, ipv6IntfObj.IntfRef)
	}
	// do link local create only if the link-local bit is set to true
	if ipv6IntfObj.LinkIp == true {
		rv, err = svcHdlr.pluginMgr.DeleteIPv6LinkLocalIP(ipv6IntfObj.IntfRef)
	}

	svcHdlr.logger.Debug("Thrift delete ipv6intf returning - ", rv, err)
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) GetIPv6IntfState(intfRef string) (ipv6IntfState *asicdServices.IPv6IntfState, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get ipv6 intf state for intf - ", intfRef))
	ipv6IntfState, err = svcHdlr.pluginMgr.GetIPv6IntfState(intfRef)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift get ipv6intf for ifindex returning - ", ipv6IntfState, err))
	return ipv6IntfState, err
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkIPv6IntfState(currMarker, count asicdServices.Int) (*asicdServices.IPv6IntfStateGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received to getbulk ipv6intfstate")
	end, listLen, more, ipv6IntfList := svcHdlr.pluginMgr.GetBulkIPv6IntfState(int(currMarker), int(count))
	if end < 0 {
		return nil, errors.New("Failed to retrieve IPv6Intf state information")
	}
	bulkObj := asicdServices.NewIPv6IntfStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.IPv6IntfStateList = ipv6IntfList
	svcHdlr.logger.Debug("Thrift returning getBulk ipv6IntfState")
	return bulkObj, nil
}

//IPv6 Neighbor related services
func (svcHdlr AsicDaemonServiceHandler) CreateIPv6Neighbor(ipAddr string, macAddr string, vlanId, ifIndex int32) (rval int32, err error) {
	svcHdlr.logger.Debug("Thrift request received for create ipv6 neighbor - ", ipAddr, macAddr, vlanId, ifIndex)
	parsedMacAddr, err := net.ParseMAC(macAddr)
	if err != nil {
		return -1, errors.New("Invalid MAC Address")
	}
	rv, err := ipValidator(ipAddr, syscall.AF_INET6)
	if rv != true {
		return -1, err
	}
	err = svcHdlr.pluginMgr.CreateIPNeighbor(ipAddr, int(vlanId), ifIndex, parsedMacAddr)
	svcHdlr.logger.Debug("Thrift create ipv6 neighbor returning - ", err)
	return (int32)(0), err
}
func (svcHdlr AsicDaemonServiceHandler) UpdateIPv6Neighbor(ipAddr string, macAddr string, vlanId, ifIndex int32) (rval int32, err error) {
	svcHdlr.logger.Debug("Thrift request received to update ipv6 neighbor - ", ipAddr, macAddr, vlanId, ifIndex)
	parsedMacAddr, err := net.ParseMAC(macAddr)
	if err != nil {
		return -1, errors.New("Invalid MAC Address")
	}
	rv, err := ipValidator(ipAddr, syscall.AF_INET6)
	if rv != true {
		return -1, err
	}
	err = svcHdlr.pluginMgr.UpdateIPNeighbor(ipAddr, int(vlanId), ifIndex, parsedMacAddr)
	svcHdlr.logger.Debug("Thrift update ipv6 neighbor returning - ", err)
	return (int32)(0), err
}
func (svcHdlr AsicDaemonServiceHandler) DeleteIPv6Neighbor(ipAddr string, macAddr string, vlanId,
	ifIndex int32) (rval int32, err error) {
	svcHdlr.logger.Debug("Thrift request received to delete ipv6 neighbor - ", ipAddr, macAddr, vlanId, ifIndex)
	parsedMacAddr, err := net.ParseMAC(macAddr)
	if err != nil {
		return -1, errors.New("Invalid MAC Address")
	}
	rv, err := ipValidator(ipAddr, syscall.AF_INET6)
	if rv != true {
		return -1, err
	}
	err = svcHdlr.pluginMgr.DeleteIPNeighbor(ipAddr, int(vlanId), ifIndex, parsedMacAddr)
	svcHdlr.logger.Debug("Thrift delete ipv6 neighbor returning - ", err)
	return (int32)(0), err
}

// Sub IPv6 Interface Create/Delete/Update API's
func (svcHdlr AsicDaemonServiceHandler) CreateSubIPv6Intf(subipv6IntfObj *asicdServices.SubIPv6Intf) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received for create sub ipv6intf - ", subipv6IntfObj))
	//rv, err = svcHdlr.pluginMgr.CreateSubIPv6Intf(subipv6IntfObj)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift create sub ipv6intf returning - ", rv, err))
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) UpdateSubIPv6Intf(oldSubIPv6IntfObj,
	newSubIPv6IntfObj *asicdServices.SubIPv6Intf, attrset []bool, op []*asicdServices.PatchOpInfo) (rv bool, err error) {
	//rv, err = svcHdlr.pluginMgr.UpdateSubIPv6Intf(oldSubIPv6IntfObj, newSubIPv6IntfObj, attrset)
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) DeleteSubIPv6Intf(subipv6IntfObj *asicdServices.SubIPv6Intf) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received for delete ipv6intf - ", subipv6IntfObj))
	//rv, err = svcHdlr.pluginMgr.DeleteSubIPv6Intf(subipv6IntfObj)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift delete ipv6intf returning - ", rv, err))
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkNdpEntryHwState(currMarker, count asicdServices.Int) (*asicdServices.NdpEntryHwStateGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received to getbulk ndp hwstate")
	end, listLen, more, ipv6NbrList := svcHdlr.pluginMgr.GetBulkIPv6Neighbor(int(currMarker), int(count))
	if end < 0 {
		return nil, errors.New("Failed to retrieve ndp hw state information")
	}
	bulkObj := asicdServices.NewNdpEntryHwStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.NdpEntryHwStateList = ipv6NbrList
	return bulkObj, nil
}
func (svcHdlr AsicDaemonServiceHandler) GetNdpEntryHwState(ipAddr string) (*asicdServices.NdpEntryHwState, error) {
	svcHdlr.logger.Debug("Thrift request received to get ndp entry hw state - ", ipAddr)
	return svcHdlr.pluginMgr.GetIPv6Neighbor(ipAddr)
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkLinkScopeIpState(currMarker, count asicdServices.Int) (*asicdServices.LinkScopeIpStateGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received to getbulk Link Scope Ip Address")
	end, listLen, more, ipv6LSList := svcHdlr.pluginMgr.GetBulkIPv6LinkScope(int(currMarker), int(count))
	if end < 0 {
		return nil, errors.New("Failed to retrieve ndp hw state information")
	}
	bulkObj := asicdServices.NewLinkScopeIpStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.LinkScopeIpStateList = ipv6LSList
	return bulkObj, nil
}

func (svcHdlr AsicDaemonServiceHandler) GetLinkScopeIpState(lsIpAddr string) (*asicdServices.LinkScopeIpState, error) {
	svcHdlr.logger.Debug("Thrift request received to get Link Scope Ip Address:", lsIpAddr)
	return svcHdlr.pluginMgr.GetIPv6LinkScope(lsIpAddr)
}
