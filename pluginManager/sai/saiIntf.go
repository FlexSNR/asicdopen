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

package sai

import (
	"asicd/pluginManager/pluginCommon"
	"asicdInt"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"unsafe"
	"utils/commonDefs"
	"utils/logging"
)

/*
#include <stdint.h>
#include <stdlib.h>
#include "saiChip.h"
#include "../pluginCommon/pluginCommon.h"
*/
import "C"

type SaiPlugin struct {
	linkStateCB             pluginCommon.ProcessLinkStateChangeCB
	portConfigCB            pluginCommon.InitPortConfigDBCB
	portStateInitCB         pluginCommon.InitPortStateDBCB
	portStateCB             pluginCommon.UpdatePortStateDBCB
	lagCB                   pluginCommon.UpdateLagDBCB
	ipNeighborCB            pluginCommon.UpdateIPNeighborDBCB
	ipv4RouteCB             pluginCommon.UpdateIPv4RouteDBCB
	ipv4NextHopCB           pluginCommon.UpdateIPv4NextHopDBCB
	ipv4NextHopGroupCB      pluginCommon.UpdateIPv4NextHopGroupDBCB
	macEntryTableCB         pluginCommon.MacEntryTableCB
	bufferPortStateInitCB   pluginCommon.InitBufferPortStateDBCB
	bufferPortStateCB       pluginCommon.UpdateBufferPortStateDBCB
	bufferGlobalStateInitCB pluginCommon.InitBufferGlobalStateDBCB
	bufferGlobalStateCB     pluginCommon.UpdateBufferGlobalStateDBCB
	//updateCoppStatStateCB   pluginCommon.UpdateCoppStatStateDBCB
	routeChannel chan *pluginCommon.PluginIPRouteInfo
}

var plugin SaiPlugin
var notifyChannel chan<- []byte
var logger *logging.Writer

func NewSaiPlugin(
	log *logging.Writer,
	notifyChan chan<- []byte,
	linkStateCB pluginCommon.ProcessLinkStateChangeCB,
	portConfigCB pluginCommon.InitPortConfigDBCB,
	portStateInitCB pluginCommon.InitPortStateDBCB,
	portStateCB pluginCommon.UpdatePortStateDBCB,
	lagCB pluginCommon.UpdateLagDBCB,
	ipNeighborCB pluginCommon.UpdateIPNeighborDBCB,
	ipv4RouteCB pluginCommon.UpdateIPv4RouteDBCB,
	ipv4NextHopCB pluginCommon.UpdateIPv4NextHopDBCB,
	ipv4NextHopGroupCB pluginCommon.UpdateIPv4NextHopGroupDBCB,
	macEntryTableCB pluginCommon.MacEntryTableCB,
	bufferPortStateInitCB pluginCommon.InitBufferPortStateDBCB,
	bufferPortStateCB pluginCommon.UpdateBufferPortStateDBCB,
	bufferGlobalStateInitCB pluginCommon.InitBufferGlobalStateDBCB,
	bufferGlobalStateCB pluginCommon.UpdateBufferGlobalStateDBCB,

) *SaiPlugin {
	logger = log
	notifyChannel = notifyChan
	plugin.linkStateCB = linkStateCB
	plugin.portConfigCB = portConfigCB
	plugin.portStateInitCB = portStateInitCB
	plugin.portStateCB = portStateCB
	plugin.lagCB = lagCB
	plugin.ipNeighborCB = ipNeighborCB
	plugin.ipv4RouteCB = ipv4RouteCB
	plugin.ipv4NextHopCB = ipv4NextHopCB
	plugin.ipv4NextHopGroupCB = ipv4NextHopGroupCB
	plugin.macEntryTableCB = macEntryTableCB
	plugin.bufferPortStateInitCB = bufferPortStateInitCB
	plugin.bufferPortStateCB = bufferPortStateCB
	plugin.bufferGlobalStateInitCB = bufferGlobalStateInitCB
	plugin.bufferGlobalStateCB = bufferGlobalStateCB
	plugin.routeChannel = make(chan *pluginCommon.PluginIPRouteInfo, 100000)
	return (&plugin)
}

func (p *SaiPlugin) InstallPluginDeps(baseDir string) {
	_, _ = exec.Command("dvs_stop.sh").CombinedOutput()
}

//General plugin functions
func (p *SaiPlugin) Init(primaryPlugin bool, baseDir string, bootMode int, ifMap []pluginCommon.IfMapInfo) int {
	var intfMapInfo []C.ifMapInfo_t

	//Install plugin dependencies
	p.InstallPluginDeps(baseDir)

	ifMapCount := len(ifMap)
	/* If count==1 and port==-1, then map all ports using interface name prefix specified */
	if (ifMapCount == 1) && (ifMap[0].Port == -1) {
		intfMapInfo = make([]C.ifMapInfo_t, 1)
		ifName := C.CString(ifMap[0].IfName)
		defer C.free(unsafe.Pointer(ifName))
		intfMapInfo[0] = C.ifMapInfo_t{
			ifName: ifName,
			port:   C.int(0),
		}
		ifMapCount = -1
	} else {
		/* Only map list of ports specified in config file to linux interfaes */
		intfMapInfo = make([]C.ifMapInfo_t, 0)
		for _, elem := range ifMap {
			ifName := C.CString(elem.IfName)
			defer C.free(unsafe.Pointer(ifName))
			intfMapInfo = append(intfMapInfo, C.ifMapInfo_t{
				ifName: ifName,
				port:   C.int(elem.Port),
			})
		}
	}
	go p.StartRouteServer()
	return int(C.SaiInit(C.int(bootMode), C.int(ifMapCount), &intfMapInfo[0], (*C.uint8_t)(&pluginCommon.SwitchMacAddr[0])))
}
func (p *SaiPlugin) StartRouteServer() {
	for {
		select {
		case routeConf := <-p.routeChannel:
			logger.Debug("SaiIntf routeServer received msg on routeConf channel with op:", routeConf.Op)
			if routeConf.Op == pluginCommon.PluginOp_Add {
				ret := p.CreateIPRoute(routeConf)
				if routeConf.DoneChannel != nil {
					routeConf.DoneChannel <- int(ret)
				}
				logger.Debug("SaiIntf: CreateIpRoute returned:", ret, "for:", *routeConf)
				/*
					if routeConf.RetHdlrFunc != nil {
						logger.Debug("rethdlr func not nil")
						routeConf.RetHdlrFunc(routeConf, routeConf.RouteManager, p, ret)
					} else {
						logger.Debug("rethdlr func nil")
					} //routeConf.DoneChannel <- int(ret)
				*/
			} else if routeConf.Op == pluginCommon.PluginOp_Add {
				ret := p.DeleteIPRoute(routeConf)
				if routeConf.DoneChannel != nil {
					routeConf.DoneChannel <- int(ret)
				}
				/*
					if routeConf.RetHdlrFunc != nil {
						logger.Debug("rethdlr func not nil")
						routeConf.RetHdlrFunc(routeConf, routeConf.RouteManager, p, ret)
					} else {
						logger.Debug("rethdlr func nil")
					}
				*/
			}
		}
	}
}
func (p *SaiPlugin) Deinit(saveState bool) int {
	var cacheSwState int

	if saveState == true {
		cacheSwState = 1
	} else {
		cacheSwState = 0
	}
	return (int(C.SaiDeinit(C.int(cacheSwState))))
}
func (p *SaiPlugin) DevShell() {
	C.SaiDevShell()
}

//Vlan related functions
func (p *SaiPlugin) CreateVlan(rsvd bool, vlanId int, portList, untagPortList []int32) int {
	rv := C.SaiCreateVlan(C.int(vlanId))
	if rv < 0 {
		return int(rv)
	}
	var pList, uList *C.int
	if len(portList) == 0 {
		pList = nil
	} else {
		pList = (*C.int)(&portList[0])
	}
	if len(untagPortList) == 0 {
		uList = nil
	} else {
		uList = (*C.int)(&untagPortList[0])
	}
	rv = C.SaiUpdateVlanAddPorts(C.int(vlanId), C.int(len(portList)), C.int(len(untagPortList)), pList, uList)
	return int(rv)
}
func (p *SaiPlugin) DeleteVlan(rsvd bool, vlanId int, portList, untagPortList []int32) int {
	logger.Info("Delete vlan " + strconv.Itoa(vlanId))
	return (int(C.SaiDeleteVlan(C.int(vlanId))))
}
func (p *SaiPlugin) UpdateVlan(vlanId int, oldPortList, oldUntagPortList, newPortList, newUntagPortList []int32) int {
	deletePortList := pluginCommon.ComputeSetDifference(oldPortList, newPortList)
	deleteUntagPortList := pluginCommon.ComputeSetDifference(oldUntagPortList, newUntagPortList)
	if len(deletePortList) != 0 || len(deleteUntagPortList) != 0 {
		rv := C.SaiUpdateVlanDeletePorts(C.int(vlanId), C.int(len(deletePortList)), C.int(len(deleteUntagPortList)), (*C.int)(&deletePortList[0]), (*C.int)(&deleteUntagPortList[0]))
		if rv < 0 {
			return int(rv)
		}
	}
	addPortList := pluginCommon.ComputeSetDifference(newPortList, oldPortList)
	addUntagPortList := pluginCommon.ComputeSetDifference(newUntagPortList, oldUntagPortList)
	var pList, uList *C.int
	if len(addPortList) == 0 {
		pList = nil
	} else {
		pList = (*C.int)(&addPortList[0])
	}
	if len(addUntagPortList) == 0 {
		uList = nil
	} else {
		uList = (*C.int)(&addUntagPortList[0])
	}
	rv := C.SaiUpdateVlanAddPorts(C.int(vlanId), C.int(len(addPortList)), C.int(len(addUntagPortList)), pList, uList)
	return int(rv)
}

//Port related functions
func (p *SaiPlugin) GetMaxSysPorts() int {
	logger.Info("Calling sai get max ports")
	return (int(C.SaiGetMaxSysPorts()))
}
func (p *SaiPlugin) InitPortConfigDB() int {
	return (int(C.SaiInitPortConfigDB()))
}
func (p *SaiPlugin) InitPortStateDB(m map[int32]string) int {
	return (int(C.SaiInitPortStateDB()))
}
func (p *SaiPlugin) UpdatePortStateDB(startPort, endPort int32) int {
	return (int(C.SaiUpdatePortStateDB(C.int(startPort), C.int(endPort))))
}
func (p *SaiPlugin) InitBufferPortStateDB(m map[int32]string) int {
	return 0
}
func (p *SaiPlugin) UpdateBufferPortStateDB(startPort, endPort int32) int {
	return 0
}
func (p *SaiPlugin) InitBufferGlobalStateDB(deviceId int) int {
	return 0
}
func (p *SaiPlugin) UpdateBufferGlobalStateDB(startId, endId int) int {
	return (int(C.SaiUpdateBufferGlobalStatDB(C.int(startId), C.int(endId))))
}

func (p *SaiPlugin) UpdatePortConfig(flags int32, newPortObj *pluginCommon.PortConfig, breakOutPortsList []int32) int {
	var portCfg C.portConfig
	portCfg.portNum = C.uint8_t(newPortObj.PortNum)
	portCfg.adminState = C.uint8_t(0xFF)
	for key, val := range pluginCommon.UpDownState {
		if val == newPortObj.AdminState {
			portCfg.adminState = C.uint8_t(key)
		}
	}
	if portCfg.adminState == C.uint8_t(0xFF) {
		logger.Err(fmt.Sprintf("Admin State doesn't match 0x%x != 0xFF", portCfg.adminState))
		return -1
	}
	portCfg.portSpeed = C.uint32_t(newPortObj.Speed)
	portCfg.duplex = C.DuplexCount
	for key, val := range pluginCommon.DuplexType {
		if val == newPortObj.Duplex {
			portCfg.duplex = uint32(key)
		}
	}
	if portCfg.duplex == C.DuplexCount {
		logger.Err(fmt.Sprintf("Port Duplex Count doesn't match 0x%x != %d", portCfg.duplex, C.DuplexCount))
		return -1
	}
	portCfg.autoneg = C.uint8_t(0xFF)
	for key, val := range pluginCommon.OnOffState {
		if val == newPortObj.Autoneg {
			portCfg.autoneg = C.uint8_t(key)
		}
	}
	if portCfg.autoneg == C.uint8_t(0xFF) {
		logger.Err(fmt.Sprintf("Autoneg doesn't match 0x%x != C.uint8_t(0xFF)", portCfg.autoneg))
		return -1
	}
	portCfg.mtu = C.int(newPortObj.Mtu)
	return (int(C.SaiUpdatePortConfig(C.int(flags), &portCfg)))
}

//export SaiIntfInitPortConfigDB
func SaiIntfInitPortConfigDB(pCfg *C.portConfig) {
	var portCfg pluginCommon.PortConfig
	var macAddr net.HardwareAddr = make(net.HardwareAddr, 6)

	for i := 0; i < pluginCommon.MAC_ADDR_LEN; i++ {
		macAddr[i] = byte(pCfg.macAddr[i])
	}
	portCfg.PortNum = int32(pCfg.portNum)
	portCfg.Description = ""
	portCfg.PhyIntfType = pluginCommon.IfType[int(pCfg.ifType)]
	portCfg.AdminState = pluginCommon.UpDownState[int(pCfg.adminState)]
	portCfg.MacAddr = macAddr.String()
	portCfg.Speed = int32(pCfg.portSpeed)
	portCfg.Duplex = pluginCommon.DuplexType[int(pCfg.duplex)]
	portCfg.Autoneg = pluginCommon.OnOffState[int(pCfg.autoneg)]
	portCfg.MediaType = "Media Type"
	portCfg.Mtu = int32(pCfg.mtu)
	portCfg.BreakOutSupported = false
	portCfg.MappedToHw = true
	portCfg.LogicalPortInfo = false
	plugin.portConfigCB(&portCfg)
}

//export SaiIntfInitPortStateDB
func SaiIntfInitPortStateDB(portNum C.int, portName *C.char) {
	plugin.portStateInitCB(int32(portNum), C.GoString(portName))
}

//export SaiIntfUpdatePortStateDB
func SaiIntfUpdatePortStateDB(pState *C.portState) {
	var portState pluginCommon.PortState

	portState.PortNum = int32(pState.portNum)
	portState.OperState = pluginCommon.UpDownState[int(pState.operState)]
	portState.Name = C.GoString(&pState.portName[0])
	pStat := (*[pluginCommon.MAX_PORT_STAT_TYPES]int64)(unsafe.Pointer(&pState.stats[0]))
	portState.IfInOctets = pStat[C.IfInOctets]
	portState.IfInUcastPkts = pStat[C.IfInUcastPkts]
	portState.IfInDiscards = pStat[C.IfInDiscards]
	portState.IfInErrors = pStat[C.IfInErrors]
	portState.IfInUnknownProtos = pStat[C.IfInUnknownProtos]
	portState.IfOutOctets = pStat[C.IfOutOctets]
	portState.IfOutUcastPkts = pStat[C.IfOutUcastPkts]
	portState.IfOutDiscards = pStat[C.IfOutDiscards]
	portState.IfOutErrors = pStat[C.IfOutErrors]
	portState.IfEtherUnderSizePktCnt = pStat[C.IfEtherUnderSizePktCnt]
	portState.IfEtherOverSizePktCnt = pStat[C.IfEtherOverSizePktCnt]
	portState.IfEtherFragments = pStat[C.IfEtherFragments]
	portState.IfEtherCRCAlignError = pStat[C.IfEtherCRCAlignError]
	portState.IfEtherJabber = pStat[C.IfEtherJabber]
	portState.IfEtherPkts = pStat[C.IfEtherPkts]
	portState.IfEtherMCPkts = pStat[C.IfEtherMCPkts]
	portState.IfEtherBcastPkts = pStat[C.IfEtherBcastPkts]
	portState.IfEtherPkts64OrLessOctets = pStat[C.IfEtherPkts64OrLessOctets]
	portState.IfEtherPkts65To127Octets = pStat[C.IfEtherPkts65To127Octets]
	portState.IfEtherPkts128To255Octets = pStat[C.IfEtherPkts128To255Octets]
	portState.IfEtherPkts256To511Octets = pStat[C.IfEtherPkts256To511Octets]
	portState.IfEtherPkts512To1023Octets = pStat[C.IfEtherPkts512To1023Octets]
	portState.IfEtherPkts1024To1518Octets = pStat[C.IfEtherPkts1024To1518Octets]
	plugin.portStateCB(&portState)
}

//Lag related functions
func (p *SaiPlugin) RestoreLagDB() int {
	return (int(C.SaiRestoreLagDB()))
}
func (p *SaiPlugin) CreateLag(obj *pluginCommon.PluginLagInfo) int {
	var list *C.int
	if len(obj.MemberList) == 0 {
		list = nil
	} else {
		list = (*C.int)(&obj.MemberList[0])
	}
	*obj.HwId = (int(C.SaiCreateLag(C.int(obj.HashType), C.int(len(obj.MemberList)), list)))
	return 0
}
func (p *SaiPlugin) DeleteLag(obj *pluginCommon.PluginLagInfo) int {
	return (int(C.SaiDeleteLag(C.int(*obj.HwId))))
}
func (p *SaiPlugin) UpdateLag(obj *pluginCommon.PluginUpdateLagInfo) int {
	var oldList, newList *C.int
	if len(obj.OldMemberList) == 0 {
		oldList = nil
	} else {
		oldList = (*C.int)(&obj.OldMemberList[0])
	}
	if len(obj.MemberList) == 0 {
		newList = nil
	} else {
		newList = (*C.int)(&obj.MemberList[0])
	}
	return (int(C.SaiUpdateLag(C.int(*obj.HwId), C.int(obj.HashType), C.int(len(obj.OldMemberList)), oldList, C.int(len(obj.MemberList)), newList)))
}

//export SaiUpdateLagDB
func SaiUpdateLagDB(lagId, hashType, portCount C.int, portList *C.int) {
	var pList []int32 = make([]int32, 0)

	p := (*[pluginCommon.MAX_SYS_PORTS]int32)(unsafe.Pointer(portList))
	for idx := 0; idx < int(portCount); idx++ {
		pList = append(pList, p[idx])
	}
	plugin.lagCB(int(lagId), int(hashType), pList)
}

//IP Neighbor related functions
func (p *SaiPlugin) RestoreIPNeighborDB() int {
	return (int(C.SaiRestoreIPNeighborDB()))
}

func (p *SaiPlugin) CreateIPNeighbor(nbrInfo *pluginCommon.PluginIPNeighborInfo) int {
	return (int(C.SaiCreateIPNeighbor((*C.uint32_t)(&nbrInfo.IpAddr[0]), C.uint32_t(nbrInfo.NeighborFlags),
		(*C.uint8_t)(&nbrInfo.MacAddr[0]), C.int(nbrInfo.VlanId), C.int(nbrInfo.IpType))))
}

func (p *SaiPlugin) DeleteIPNeighbor(nbrInfo *pluginCommon.PluginIPNeighborInfo) int {
	return (int(C.SaiDeleteIPNeighbor((*C.uint32_t)(&nbrInfo.IpAddr[0]),
		(*C.uint8_t)(&nbrInfo.MacAddr[0]), C.int(nbrInfo.VlanId), C.int(nbrInfo.IpType))))
}

func (p *SaiPlugin) UpdateIPNeighbor(nbrInfo *pluginCommon.PluginIPNeighborInfo) int {
	return (int(C.SaiUpdateIPNeighbor((*C.uint32_t)(&nbrInfo.IpAddr[0]), (*C.uint8_t)(&nbrInfo.MacAddr[0]),
		C.int(nbrInfo.VlanId), C.int(nbrInfo.IpType))))
}

//export SaiUpdateIPNeighborDB
func SaiUpdateIPNeighborDB(ipAddr, neighborFlags C.uint32_t, macAddr *C.uint8_t, phyIntf, nextHopId C.int) {
	// @TODO: change this to support Modular IPV4 and IPV6... same as opennsl & bcm
	/*
		var ifIndex int32
		var mac net.HardwareAddr

		m := (*[pluginCommon.MAC_ADDR_LEN]byte)(unsafe.Pointer(macAddr))
		for idx := 0; idx < pluginCommon.MAC_ADDR_LEN; idx++ {
			mac[idx] = m[idx]
		}
		if neighborFlags&pluginCommon.NEIGHBOR_L2_ACCESS_TYPE_PORT == pluginCommon.NEIGHBOR_L2_ACCESS_TYPE_PORT {
			ifIndex = pluginCommon.GetIfIndexFromIdType(int(phyIntf), commonDefs.IfTypePort)
		} else {
			ifIndex = pluginCommon.GetIfIndexFromIdType(int(phyIntf), commonDefs.IfTypeLag)
		}
		plugin.ipv4NeighborCB(uint32(ipAddr), mac, ifIndex, uint64(nextHopId))
	*/
}

//IP Route related functions
//IP Route related functions
func (p *SaiPlugin) AddToRouteChannel(ipInfo *pluginCommon.PluginIPRouteInfo) {
	p.routeChannel <- ipInfo
}
func (p *SaiPlugin) RestoreIPv6RouteDB() int {
	return (int(C.SaiRestoreIPv6RouteDB()))
}
func (p *SaiPlugin) RestoreIPv4RouteDB() int {
	return (int(C.SaiRestoreIPv4RouteDB()))
}
func (p *SaiPlugin) CreateIPRoute(ipInfo *pluginCommon.PluginIPRouteInfo) int {
	return (int(C.SaiCreateIPRoute((*C.uint8_t)(&ipInfo.PrefixIp[0]), (*C.uint8_t)(&ipInfo.Mask[0]), C.uint32_t(ipInfo.RouteFlags), C.uint64_t(ipInfo.NextHopId), C.int(ipInfo.RifId))))
}
func (p *SaiPlugin) DeleteIPRoute(ipInfo *pluginCommon.PluginIPRouteInfo) int {
	ret := C.SaiDeleteIPRoute((*C.uint8_t)(&ipInfo.PrefixIp[0]), (*C.uint8_t)(&ipInfo.Mask[0]), C.uint32_t(ipInfo.RouteFlags))
	return int(ret)
}

//export SaiUpdateIPv4RouteDB
func SaiUpdateIPv4RouteDB(prefixIP, prefixMask, nextHopIp C.uint32_t) {
	plugin.ipv4RouteCB(uint32(prefixIP), uint32(prefixMask), uint32(nextHopIp))
}

//IPv4 NextHop related functions
func (p *SaiPlugin) RestoreIPv4NextHopDB() {
}

func (p *SaiPlugin) CreateIPNextHop(nbrInfo *pluginCommon.PluginIPNeighborInfo) uint64 {
	return (uint64(C.SaiCreateIPNextHop((*C.uint32_t)(&nbrInfo.IpAddr[0]), C.uint32_t(nbrInfo.NextHopFlags),
		C.int(nbrInfo.VlanId), C.int(nbrInfo.L2IfIndex), (*C.uint8_t)(&nbrInfo.MacAddr[0]),
		C.int(nbrInfo.IpType))))
}

func (p *SaiPlugin) DeleteIPNextHop(nbrInfo *pluginCommon.PluginIPNeighborInfo) int {
	return (int(C.SaiDeleteIPNextHop(C.uint64_t(nbrInfo.NextHopId))))
}

func (p *SaiPlugin) UpdateIPNextHop(nbrInfo *pluginCommon.PluginIPNeighborInfo) int {
	// commented out as SaiUpdateIPNextHop is NO-OP
	return 1 //(int(C.SaiUpdateIPNextHop(C.uint32_t(ipAddr), C.uint64_t(nextHopId), C.int(vlanId), C.int(routerPhyIntf), (*C.uint8_t)(&macAddr[0]))))
}

//export SaiUpdateIPv4NextHopDB
func SaiUpdateIPv4NextHopDB() {
}

//IPv4 NextHopGroup related functions
func (p *SaiPlugin) RestoreIPv4NextHopGroupDB() {
}
func (p *SaiPlugin) CreateIPNextHopGroup(nhGrpMembers []*pluginCommon.NextHopGroupMemberInfo) uint64 {
	var nhMemberIds []C.uint64_t
	for _, val := range nhGrpMembers {
		nhMemberIds = append(nhMemberIds, C.uint64_t(val.NextHopId))
	}
	return (uint64(C.SaiCreateIPNextHopGroup(C.int(len(nhGrpMembers)), (*C.uint64_t)(&nhMemberIds[0]))))
}
func (p *SaiPlugin) DeleteIPNextHopGroup(groupId uint64) int {
	return (int(C.SaiDeleteIPNextHopGroup(C.uint64_t(groupId))))
}
func (p *SaiPlugin) UpdateIPNextHopGroup(groupId uint64, nhGrpMembers []*pluginCommon.NextHopGroupMemberInfo) int {
	var nhMemberIds []C.uint64_t
	for _, val := range nhGrpMembers {
		nhMemberIds = append(nhMemberIds, C.uint64_t(val.NextHopId))
	}
	return (int(C.SaiUpdateIPNextHopGroup(C.int(len(nhGrpMembers)), (*C.uint64_t)(&nhMemberIds[0]), C.uint64_t(groupId))))
}

//export SaiUpdateIPv4NextHopGroupDB
func SaiUpdateIPv4NextHopGroupDB() {
}

//Logical interface related functions
func (p *SaiPlugin) CreateLogicalIntfConfig(name string, ifType int) int {
	return 0
}
func (p *SaiPlugin) DeleteLogicalIntfConfig(name string, ifType int) int {
	return 0
}

//IP interface related functions
func (p *SaiPlugin) CreateIPIntf(ipInfo *pluginCommon.PluginIPInfo) int {
	if ipInfo.RefCount == 0 { // if 0 refCount then only do init
		rv := (int(C.SaiCreateIPIntf((*C.uint32_t)(&ipInfo.IpAddr[0]), C.int(ipInfo.MaskLen),
			C.int(ipInfo.VlanId))))
		if rv < 0 {
			return rv
		}
	}
	return (int(C.SaiRouteAddDel((*C.uint32_t)(&ipInfo.IpAddr[0]), C.int(ipInfo.IpType), C.bool(true))))
}

func (p *SaiPlugin) UpdateIPIntf(ipInfo *pluginCommon.PluginIPInfo) int {
	if ipInfo.AdminState == pluginCommon.INTF_STATE_UP {
		return (int(C.SaiRouteAddDel((*C.uint32_t)(&ipInfo.IpAddr[0]),
			C.int(ipInfo.IpType), C.bool(true))))
	} else {
		return (int(C.SaiRouteAddDel((*C.uint32_t)(&ipInfo.IpAddr[0]),
			C.int(ipInfo.IpType), C.bool(false))))
	}
}

func (p *SaiPlugin) DeleteIPIntf(ipInfo *pluginCommon.PluginIPInfo) int {
	if ipInfo.RefCount == 1 {
		// If only '1' IP then delete the intf otherwise do not
		rv := int(C.SaiDeleteIPIntf((*C.uint32_t)(&ipInfo.IpAddr[0]), C.int(ipInfo.MaskLen),
			C.int(ipInfo.VlanId)))
		if rv < 0 {
			return rv
		}
	}
	return (int(C.SaiRouteAddDel((*C.uint32_t)(&ipInfo.IpAddr[0]), C.int(ipInfo.IpType), C.bool(false))))
}

//Sub IPv4 interface related functions
func (p *SaiPlugin) CreateSubIPv4Intf(obj pluginCommon.SubIntfPluginObj,
	ifName *string) int {
	return 1
}
func (p *SaiPlugin) DeleteSubIPv4Intf(ifName string) int {
	return 1
}

func (p *SaiPlugin) UpdateSubIPv4Intf(ifName string, mac net.HardwareAddr,
	ipAddr uint32, stateUp bool) int {
	return (int(C.SaiUpdateSubIPv4Intf(C.uint32_t(ipAddr), C.bool(stateUp))))
}

//Plugin notifications that need to be published
//export SaiNotifyLinkStateChange
func SaiNotifyLinkStateChange(portNum, operState, speed, duplex C.int) {
	msg := pluginCommon.L2IntfStateNotifyMsg{
		IfIndex: pluginCommon.GetIfIndexFromIdType(int(portNum), commonDefs.IfTypePort),
		IfState: uint8(operState),
	}
	msgBuf, err := json.Marshal(msg)
	if err != nil {
		logger.Err("Error in marshalling Json, in opennsl NotifyLinkStateChange")
	}
	notification := pluginCommon.AsicdNotification{
		MsgType: uint8(pluginCommon.NOTIFY_L2INTF_STATE_CHANGE),
		Msg:     msgBuf,
	}
	notificationBuf, err := json.Marshal(notification)
	if err != nil {
		logger.Err("Failed to marshal vlan create message")
	}
	notifyChannel <- notificationBuf
	logger.Info(fmt.Sprintln("Sent notification for port link state change - ", portNum, operState, speed))
	plugin.linkStateCB(int32(portNum), int32(speed), pluginCommon.DuplexType[int(duplex)], pluginCommon.UpDownState[int(operState)])
}

//STP related functions
func (p *SaiPlugin) CreateStg(vlanList []int32) int {
	var vList *C.int
	if len(vlanList) == 0 {
		vList = nil
	} else {
		vList = (*C.int)(&vlanList[0])
	}
	stgId := (int(C.SaiCreateStg(C.int(len(vlanList)), vList)))
	return stgId
}
func (p *SaiPlugin) DeleteStg(stgId int32) int {
	rv := (int(C.SaiDeleteStg(C.int(stgId))))
	return rv
}
func (p *SaiPlugin) SetPortStpState(stgId, port, stpState int32) int {
	return (int(C.SaiSetPortStpState(C.int(stgId), C.int(port), C.int(stpState))))
}
func (p *SaiPlugin) GetPortStpState(stgId, port int32) int {
	return (int(C.SaiGetPortStpState(C.int(stgId), C.int(port))))
}
func (p *SaiPlugin) UpdateStgVlanList(stgId int32, oldVlanList, newVlanList []int32) int {
	var oldList, newList *C.int
	if len(oldVlanList) == 0 {
		oldList = nil
	} else {
		oldList = (*C.int)(&oldVlanList[0])
	}
	if len(newVlanList) == 0 {
		newList = nil
	} else {
		newList = (*C.int)(&newVlanList[0])
	}
	rv := (int(C.SaiUpdateStgVlanList(C.int(stgId), C.int(len(oldVlanList)), oldList, C.int(len(newVlanList)), newList)))
	if len(oldVlanList) > 0 {
		for _, vlan := range oldVlanList {
			// lets flush after deletion
			C.SaiFlushFdbByPortVlan(C.int(vlan), -1)
		}
	}
	return rv
}

func (p *SaiPlugin) FlushFdbStgGroup(vlanList []int32, port int32) {
	if len(vlanList) > 0 {
		for _, vlan := range vlanList {
			C.SaiFlushFdbByPortVlan(C.int(vlan), C.int(port))
		}
	} else if port != -1 {
		C.SaiFlushFdbByPortVlan(-1, C.int(port))
	}
}

//export SaiNotifyFDBEvent
func SaiNotifyFDBEvent(op C.int, l2IfIndex, vlanId C.int, macAddr *C.uint8_t) {
	var mac net.HardwareAddr = make(net.HardwareAddr, 6)
	m := (*[pluginCommon.MAC_ADDR_LEN]byte)(unsafe.Pointer(macAddr))
	for idx := 0; idx < pluginCommon.MAC_ADDR_LEN; idx++ {
		mac[idx] = m[idx]
	}
	plugin.macEntryTableCB(int(op), int32(l2IfIndex), int32(vlanId), mac)
	logger.Info(fmt.Sprintln("Received FDB event :op, port, vlan, mac - ", int(op), int32(l2IfIndex), int32(vlanId), mac))
}

func (p *SaiPlugin) AddProtocolMacEntry(macAddr string, mask string,
	VlanId int32) int {
	rsvdMac, err := net.ParseMAC(macAddr)
	if err != nil {
		logger.Err("Parsing mac " + macAddr + " failed")
		return -1
	}
	macMask, err := net.ParseMAC(mask)
	if err != nil {
		logger.Err("Parsing mask " + mask + " failed")
		return -1
	}
	logger.Info("successully parsed mac:" + macAddr +
		" mask:" + mask)
	return (int(C.SaiAddRsvdProtoMacEntry((*C.uint8_t)(&rsvdMac[0]),
		(*C.uint8_t)(&macMask[0]), C.int(VlanId))))
}

func (p *SaiPlugin) DeleteProtocolMacEntry(macAddr string, mask string,
	VlanId int32) int {
	rsvdMac, err := net.ParseMAC(macAddr)
	if err != nil {
		logger.Err("Parsing mac " + macAddr + " failed")
		return -1
	}
	macMask, err := net.ParseMAC(mask)
	if err != nil {
		logger.Err("Parsing mask " + mask + " failed")
		return -1
	}
	logger.Info("successully parsed mac:" + macAddr +
		" mask:" + mask)
	return (int(C.SaiDeleteRsvdProtoMacEntry((*C.uint8_t)(&rsvdMac[0]),
		(*C.uint8_t)(&macMask[0]), C.int(VlanId))))
}

func (p *SaiPlugin) PortEnableSet(portNum int32, adminState string) int {
	/*
		portadmin := C.uint8_t(0xFF)
		for key, val := range pluginCommon.UpDownState {
			if val == adminState {
				portadmin = C.uint8_t(key)
			}
		}
		if portadmin == C.uint8_t(0xFF) {
			return -1
		}
		return int(C.SaiAdminStateSet(C.int(portNum), C.int(portadmin)))
	*/
	return 0
}

func (p *SaiPlugin) CreateVxlan(config *asicdInt.Vxlan) {
}

func (p *SaiPlugin) DeleteVxlan(config *asicdInt.Vxlan) {
}

func (p SaiPlugin) AddInterfaceToVxlanBridge(vni uint32, ifIndexList, untagIfIndexList []int32) {
}

func (p SaiPlugin) DelInterfaceFromVxlanBridge(vni uint32, ifIndexList, untagIfIndexList []int32) {
}

func (p *SaiPlugin) CreateVtepIntf(vtepIfName string, ifIndex int, nextHopIfIndex int,
	vni int32, dstIpAddr uint32, srcIpAddr uint32, macAddr net.HardwareAddr,
	vlanId int32, ttl int32, udp int32, nextHopIndex uint64) {
}

func (p *SaiPlugin) DeleteVtepIntf(vtepIfName string, ifIndex int, nextHopIfIndex int,
	vni int32, dstIpAddr uint32, srcIpAddr uint32, macAddr net.HardwareAddr,
	VlanId int32, ttl int32, udp int32) {

}

func (p *SaiPlugin) LearnFdbVtep(mac net.HardwareAddr, vtepname string, ifindex int32) {
}

func (v *SaiPlugin) VxlanPortEnable(portNum int32) {

}
func (v *SaiPlugin) VxlanPortDisable(portNum int32) {

}

//export SaiIntfUpdateBufferGlobalStateDB
func SaiIntfUpdateBufferGlobalStateDB(bState *C.bufferGlobalState) {
	var bufferGlobalState pluginCommon.BufferGlobalState
	bStat := (*[pluginCommon.MAX_BUFFER_GLOBAL_STAT_TYPES]int64)(unsafe.Pointer(&bState.stats[0]))
	bufferGlobalState.DeviceId = int32(bState.deviceId)
	bufferGlobalState.BufferStat = uint64(bStat[C.BufferStat])
	bufferGlobalState.EgressBufferStat = uint64(bStat[C.EgressBufferStat])
	bufferGlobalState.IngressBufferStat = uint64(bStat[C.IngressBufferStat])
	plugin.bufferGlobalStateCB(&bufferGlobalState)

}

func (v *SaiPlugin) CreateAclConfig(aclName string, aclType string, aclRule pluginCommon.AclRule, portList []int32, direction string) int {
	var nportList *C.int
	var ar C.aclRule
	var dir C.int
	var at C.int

	ar.ruleName = C.CString(aclRule.RuleName)
	defer C.free(unsafe.Pointer(ar.ruleName))
	if aclRule.SourceMac != nil {
		ar.sourceMac = (*C.uint8_t)(&aclRule.SourceMac[0])
		defer C.free(unsafe.Pointer(ar.sourceMac))
	}
	if aclRule.DestMac != nil {
		ar.destMac = (*C.uint8_t)(&aclRule.DestMac[0])
		defer C.free(unsafe.Pointer(ar.destMac))
	}
	if aclRule.SourceIp != nil {
		ar.sourceIp = (*C.uint8_t)(&aclRule.SourceIp[0])
		defer C.free(unsafe.Pointer(ar.sourceIp))
	}

	if aclRule.DestIp != nil {
		ar.destIp = (*C.uint8_t)(&aclRule.DestIp[0])
		defer C.free(unsafe.Pointer(ar.destIp))
	}
	if aclRule.SourceMask != nil {
		ar.sourceMask = (*C.uint8_t)(&aclRule.SourceMask[0])
		defer C.free(unsafe.Pointer(ar.sourceMask))
	}
	if aclRule.DestMask != nil {
		ar.destMask = (*C.uint8_t)(&aclRule.DestMask[0])
		defer C.free(unsafe.Pointer(ar.destMask))
	}
	ar.proto = C.int(aclRule.Proto)
	ar.srcport = C.int(aclRule.SrcPort)
	ar.dstport = C.int(aclRule.DstPort)
	ar.l4SrcPort = C.int(aclRule.L4SrcPort)
	ar.l4DstPort = C.int(aclRule.L4DstPort)
	ar.l4MinPort = C.int(aclRule.L4MinPort)
	ar.l4MaxPort = C.int(aclRule.L4MaxPort)

	if len(portList) == 0 {
		return -1
	}
	switch aclRule.Action {
	case "DENY":
		ar.action = C.int(C.AclDeny)
		break

	case "ALLOW":
		ar.action = C.int(C.AclAllow)
		break

	default:
		logger.Err("ACL : No action defiend. Rule wil not be applied ", aclName)
		return -1
	}
	switch aclType {
	case pluginCommon.ACL_TYPE_IP:
		at = C.int(C.AclIp)
		break
	case pluginCommon.ACL_TYPE_MAC:
		at = C.int(C.AclMac)
		break
	case pluginCommon.ACL_TYPE_MLAG:
		at = C.int(C.AclMlag)
		break
	default:
		logger.Err("Acl : Acl type is invalid. Make sure it is IP/MAC/MLAG ")
		return 0
		break
	}

	switch direction {
	case "IN":
		dir = C.int(C.AclIn)
		break
	case "OUT":
		dir = C.int(C.AclOut)
		break
	default:
		logger.Info("Acl : No direction set for the acl.")
	}
	switch aclRule.L4PortMatch {
	case "EQ":
		ar.l4PortMatch = C.int(C.AclEq)
		break
	case "NEQ":
		ar.l4PortMatch = C.int(C.AclNeq)
		break
	case "LT", "GT", "RANGE":
		ar.l4PortMatch = C.int(C.AclRange)
		break
	default:
		ar.l4PortMatch = C.int(C.AclNeq)
	}

	nportList = (*C.int)(&portList[0])

	return int(C.SaiProcessAcl(C.CString(aclName), C.int(at), ar, C.int(len(portList)), nportList, C.int(dir)))
	return 0
}

func (p *SaiPlugin) ClearPortStat(portNum int32) int {
	return 0
}

func (p *SaiPlugin) UpdateCoppStatStateDB(startId, endId int) int {
	return 0
}
func (p *SaiPlugin) GetModuleTemperature() float64 {
	//TO-DO
	return 0
}

func (p *SaiPlugin) GetModuleInventory() *pluginCommon.Inventory {
	//TO-DO
	return nil
}

func (p *SaiPlugin) UpdateAclStateDB(ruleName string) int {
	return 0
}
