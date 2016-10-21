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

package pluginManager

import (
	"asicd/pluginManager/pluginCommon"
	"errors"
	"fmt"
	"net"
	"syscall"
	"utils/commonDefs"
	"utils/logging"
)

const (
	INITIAL_NEXTHOP_MAP_SIZE = 2048
)

type NextHopIntf interface {
	ConvertIPStringToUint32(**pluginCommon.PluginIPNeighborInfo)
	NextHopExists(**pluginCommon.PluginIPNeighborInfo) bool
	UpdateNhDb(*pluginCommon.PluginIPNeighborInfo)
	NextHopId(**pluginCommon.PluginIPNeighborInfo)
	CheckRefCount(*pluginCommon.PluginIPNeighborInfo) bool
	GetRefCount(*pluginCommon.PluginIPNeighborInfo) int
	CollectInfoFromDb(**pluginCommon.PluginIPNeighborInfo)
}

type IPNextHopGroupInfo struct {
	members  map[interface{}]*pluginCommon.NextHopGroupMemberInfo
	refCount int
}

type NextHopManager struct {
	portMgr          *PortManager
	vlanMgr          *VlanManager
	ipv4NextHopDB    map[uint32]*IPv4NextHopInfo
	ipv6NextHopDB    map[string]*IPv6NextHopInfo
	ipNextHopGroupDB map[uint64]*IPNextHopGroupInfo
	plugins          []PluginIntf
	logger           *logging.Writer
	notifyChannel    chan<- []byte
}

//NextHop manager db
var NextHopMgr NextHopManager

//FIXME: Implement when adding support for WB
func UpdateIPv4NextHopDB() {
}

//FIXME: Implement when adding support for WB
func UpdateIPv4NextHopGroupDB() {
}

func (nMgr *NextHopManager) Init(rsrcMgrs *ResourceManagers, bootMode int, pluginList []PluginIntf, logger *logging.Writer, notifyChannel chan<- []byte) {
	nMgr.logger = logger
	nMgr.plugins = pluginList
	nMgr.vlanMgr = rsrcMgrs.VlanManager
	nMgr.notifyChannel = notifyChannel
	nMgr.portMgr = rsrcMgrs.PortManager
	nMgr.ipv4NextHopDB = make(map[uint32]*IPv4NextHopInfo, INITIAL_NEXTHOP_MAP_SIZE)
	nMgr.ipv6NextHopDB = make(map[string]*IPv6NextHopInfo, INITIAL_NEXTHOP_MAP_SIZE)
	nMgr.ipNextHopGroupDB = make(map[uint64]*IPNextHopGroupInfo, INITIAL_NEXTHOP_MAP_SIZE)
	if bootMode == pluginCommon.BOOT_MODE_WARMBOOT {
		//Restore nexthop db during warm boot
		/*FIXME: Implement when adding support for WB
		  for _, plugin := range nMgr.plugins {
		  			if isPluginAsicDriver(plugin) == true {
		  				rv := plugin.RestoreIPv4NextHopDB()
		  				if rv < 0 {
		  					nMgr.logger.Err("Failed to restore IPv4 next hop db during warm boot")
		  				}
		  				rv := plugin.RestoreIPv4NextHopGroupDB()
		  				if rv < 0 {
		  					nMgr.logger.Err("Failed to restore IPv4 next hop db during warm boot")
		  				}
		  				break
		  			}
		  		}
		*/
	}
}

func (nMgr *NextHopManager) Deinit() {
	/* Currently no-op */
}

func (nMgr *NextHopManager) getNextHopFlagsL2IfIndex(info **pluginCommon.PluginIPNeighborInfo) {
	nbrInfo := *info
	if pluginCommon.GetTypeFromIfIndex(nbrInfo.IfIndex) == commonDefs.IfTypeLag {
		//Neighbor reachable via lag
		nbrInfo.NextHopFlags |= pluginCommon.NEXTHOP_L2_ACCESS_TYPE_LAG
		nbrInfo.L2IfIndex = int32(pluginCommon.GetIdFromIfIndex(nbrInfo.IfIndex))
	} else {
		//Neighbor reachable via port
		nbrInfo.NextHopFlags |= pluginCommon.NEXTHOP_L2_ACCESS_TYPE_PORT
		nbrInfo.L2IfIndex = nMgr.portMgr.GetPortNumFromIfIndex(nbrInfo.IfIndex)
	}
}

func (nMgr *NextHopManager) createNextHopIntf(nbrInfo *pluginCommon.PluginIPNeighborInfo) NextHopIntf {
	var nhObj NextHopIntf
	if nbrInfo.IpType == syscall.AF_INET {
		nhObj = &IPv4NextHopInfo{}
	} else {
		nhObj = &IPv6NextHopInfo{}
	}
	// during create nexthop ip address is populated by the caller which is neigborMgr
	if nbrInfo.OperationType != NBR_CREATE_OPERATION {
		nhObj.ConvertIPStringToUint32(&nbrInfo)
	}

	// during next hop group delete operation we need to get vlanId, ifIndex, macAddress from the next hop db
	if nbrInfo.OperationType == NH_GROUP_DELETE_OPERATION {
		nhObj.CollectInfoFromDb(&nbrInfo)
	}
	return nhObj
}

func (nMgr *NextHopManager) CreateNextHop(info **pluginCommon.PluginIPNeighborInfo) error {
	nbrInfo := *info
	nhObj := nMgr.createNextHopIntf(nbrInfo)
	// Check if next hop already exists, if so update nextHopId and return nil
	if nhObj.NextHopExists(&nbrInfo) {
		// if arp is sending create neighbor then we need to update next hop as ribd already created
		// nexthop entry
		if nbrInfo.OperationType == NBR_CREATE_OPERATION {
			//nMgr.logger.Debug("Updating next hop as entry already exists for create ip neigbor:", *nbrInfo)
			err := nMgr.UpdateNextHop(nbrInfo)
			return err
		}
		nMgr.logger.Debug("Next Hop exists and hence nothing to do for:", *nbrInfo)
		return nil
	}
	nMgr.getNextHopFlagsL2IfIndex(&nbrInfo)
	//Update HW state
	for _, plugin := range nMgr.plugins {
		rv := plugin.CreateIPNextHop(nbrInfo)
		if isPluginAsicDriver(plugin) == true {
			nbrInfo.NextHopId = rv
		}
		if rv < 0 {
			nbrInfo.NextHopId = pluginCommon.INVALID_NEXTHOP_ID
			return errors.New("Failed to create next hop entry")
		}
	}
	// Update SW cache
	nhObj.UpdateNhDb(nbrInfo)
	return nil
}

func (nMgr *NextHopManager) UpdateNextHop(nbrInfo *pluginCommon.PluginIPNeighborInfo) error {
	nhObj := nMgr.createNextHopIntf(nbrInfo)
	// Check if next hop already exists
	if !nhObj.NextHopExists(&nbrInfo) {
		return errors.New("IP next hop update failed, next hop does not exist")
	}
	//Retrieve next hop id which will be updated by this operation.
	nhObj.NextHopId(&nbrInfo)
	nMgr.getNextHopFlagsL2IfIndex(&nbrInfo)
	//Update HW state
	for _, plugin := range nMgr.plugins {
		rv := plugin.UpdateIPNextHop(nbrInfo)
		if rv < 0 {
			return errors.New("Plugin failed to update next hop")
		}
	}
	// Update SW cache
	nhObj.UpdateNhDb(nbrInfo)
	return nil
}

func (nMgr *NextHopManager) DeleteNextHop(nbrInfo *pluginCommon.PluginIPNeighborInfo) error {
	//nMgr.logger.Debug("Inside DeleteNextHop for nbrInfo:", nbrInfo)
	nhObj := nMgr.createNextHopIntf(nbrInfo)
	// Check if next hop already exists
	if !nhObj.NextHopExists(&nbrInfo) {
		nMgr.logger.Err("IP next hop update failed, next hop does not exist", *nbrInfo)
		return errors.New("IP next hop update failed, next hop does not exist")
	}
	//Delete nexthop if refcount reaches 0
	delete := nhObj.CheckRefCount(nbrInfo)
	//nMgr.logger.Debug("Check refcount for the next hop nbrInfo")
	if delete {
		nhObj.NextHopId(&nbrInfo)
		//	nMgr.logger.Debug("deleting next hop as refCount is 0 for:", nbrInfo)
		//Update HW state
		for _, plugin := range nMgr.plugins {
			rv := plugin.DeleteIPNextHop(nbrInfo)
			if rv < 0 {
				nMgr.logger.Err("Failed to delete next hop", rv, *nbrInfo)
				return errors.New(fmt.Sprintln("Failed to delete next hop", rv))
			}
		}
		//Update SW cache
		nhObj.UpdateNhDb(nbrInfo)
	}
	return nil
}

func (nMgr *NextHopManager) GetNextHopRefCount(nbrInfo *pluginCommon.PluginIPNeighborInfo) int {

	nhObj := nMgr.createNextHopIntf(nbrInfo)

	return nhObj.GetRefCount(nbrInfo)
}

func (nMgr *NextHopManager) GetIPv4AddrFromNextHopId(nextHopId uint64) uint32 {
	for key, val := range nMgr.ipv4NextHopDB {
		if val.nextHopId == nextHopId {
			return key
		}
	}
	return 0
}

func (nMgr *NextHopManager) GetNextHopIdForIPv4Addr(ipAddr uint32, nextHopIfType int) uint64 {
	if val, ok := nMgr.ipv4NextHopDB[ipAddr]; ok {
		nMgr.logger.Debug(fmt.Sprintln("Nexthop id for IP = ", ipAddr, val.nextHopId))
		return val.nextHopId
	} else {
		return pluginCommon.INVALID_NEXTHOP_ID
	}
}

func getIpInformation(ip interface{}) (string, int) {
	var ipType int
	var ipAddr string
	switch ip.(type) {
	case uint32:
		// ipv4 object
		ipType = syscall.AF_INET
		ip1 := ip.(uint32)
		ipAddr = net.IPv4(byte((ip1>>24)&0xff), byte((ip1>>16)&0xff), byte((ip1>>8)&0xff),
			byte(ip1&0xff)).String()

	case string:
		// ipv6 object
		ipType = syscall.AF_INET6
		ipAddr = ip.(string)
	}
	return ipAddr, ipType
}

func (nMgr *NextHopManager) CheckForExistingNextHopGroup(members []*pluginCommon.NextHopGroupMemberInfo) (uint64, error) {
	for nhGrpId, info := range nMgr.ipNextHopGroupDB {
		var found bool = true
		//nMgr.logger.Debug("searching for next hop groupId:", nhGrpId)
		for _, member := range members {
			//nMgr.logger.Debug("key:", member.IpAddr)
			dbMember, ok := info.members[member.IpAddr]
			if !ok || dbMember.IpAddr != member.IpAddr {
				//nMgr.logger.Debug("member:", *member, "is not equal to dbMember:", *dbMember)
				found = false
				break
			}
		}
		if found == false || len(nMgr.ipNextHopGroupDB[nhGrpId].members) != len(members) {
			continue
		} else {
			nMgr.logger.Debug("Found an exact match:", nhGrpId)
			//Found an exact match
			return nhGrpId, nil
		}
	}
	//nMgr.logger.Debug("No matching next hop groups found")
	return uint64(0), errors.New("No matching next hop groups found")
}

func getIpTypeFromString(ipAddr string) int {
	ip := net.ParseIP(ipAddr)
	if ip.To4() == nil {
		return syscall.AF_INET6
	}
	return syscall.AF_INET
}

func (nMgr *NextHopManager) CreateNextHopGroup(nhGroupMembers []*pluginCommon.NextHopGroupMemberInfo) (uint64, error) {
	nMgr.logger.Debug("Creating Next Hop Group called")
	var nextHopGroupId uint64
	if nhGroupMembers == nil {
		return uint64(0), errors.New("Empty member list provided to next hop group create")
	}
	nMgr.logger.Debug("Check if we can share an existing nh group")
	//Check if we can share an existing nh group
	if len(nhGroupMembers) >= 1 {
		nextHopGroupId, err := nMgr.CheckForExistingNextHopGroup(nhGroupMembers)
		if err == nil {
			nMgr.logger.Debug("Reuse group:", nextHopGroupId, "and update refCount")
			//Reuse group and update refcount
			info := nMgr.ipNextHopGroupDB[nextHopGroupId]
			info.refCount += 1
			nMgr.logger.Debug("nMgr.ipNextHopGroupDB for nextHopGroupId:", nextHopGroupId, "is:", *info)
			return nextHopGroupId, nil
		}
	}
	nMgr.logger.Debug("Cannot share, create new next hop group")
	//Cannot share, create new next hop group
	for _, member := range nhGroupMembers {
		nbrInfo := &pluginCommon.PluginIPNeighborInfo{
			MacAddr:       net.HardwareAddr{0, 0, 0, 0, 0, 0},
			NextHopFlags:  uint32(pluginCommon.NEXTHOP_TYPE_COPY_TO_CPU),
			VlanId:        pluginCommon.GetIdFromIfIndex(member.RifId),
			OperationType: NH_GROUP_CREATE_OPERATION,
			IfIndex:       int32(pluginCommon.INVALID_IFINDEX),
			Address:       member.IpAddr.(string),
		}
		nbrInfo.IpType = getIpTypeFromString(member.IpAddr.(string))
		nMgr.logger.Debug("CreateNextHop called for:", *nbrInfo)
		err := nMgr.CreateNextHop(&nbrInfo)
		if err != nil {
			return uint64(0), errors.New("Failed to create next hop, when creating next hop group")
		}
		member.NextHopId = nbrInfo.NextHopId
		nextHopGroupId = nbrInfo.NextHopId
	}
	//if len(nhGroupMembers) > 1 {
	nMgr.logger.Debug("Create a multipath group")
	//Create a multipath group
	for _, plugin := range nMgr.plugins {
		rv := plugin.CreateIPNextHopGroup(nhGroupMembers)
		if isPluginAsicDriver(plugin) == true {
			nextHopGroupId = rv
		}
		if rv < 0 {
			return uint64(0), errors.New("Failed to create next hop group entry")
		}
	}
	nMgr.logger.Debug("after creating IP next hops nextHopGroupId is:", nextHopGroupId)
	nMgr.logger.Debug("Update SW cache")
	//nMgr.logger.Debug("Update SW cache")
	// Update SW cache
	nhGrpInfo := new(IPNextHopGroupInfo)
	nhGrpInfo.members = make(map[interface{}]*pluginCommon.NextHopGroupMemberInfo)
	for idx, member := range nhGroupMembers {
		memberInfo := *nhGroupMembers[idx]
		nhGrpInfo.members[member.IpAddr] = &memberInfo
	}
	nhGrpInfo.refCount = 1
	nMgr.ipNextHopGroupDB[nextHopGroupId] = nhGrpInfo
	//}
	nMgr.logger.Debug("nextHopGroupId created:", nextHopGroupId)
	return nextHopGroupId, nil
}

func (nMgr *NextHopManager) GetNextHopGroupRefCount(groupId uint64) int {
	if val, ok := nMgr.ipNextHopGroupDB[groupId]; !ok {
		return 0
	} else {
		return val.refCount
	}
}

func (nMgr *NextHopManager) DeleteNextHopGroup(groupId uint64) error {
	nMgr.logger.Debug("Inside DeleteNextHopGroup for groupId:", groupId)
	// Check if next hop group exists
	if _, ok := nMgr.ipNextHopGroupDB[groupId]; !ok {
		nMgr.logger.Err("IP next hop group delete failed, group does not exist")
		return errors.New("IP next hop group delete failed, group does not exist")
	}
	info := nMgr.ipNextHopGroupDB[groupId]
	nMgr.logger.Debug("info retrieved from nMgr.ipNextHopGroupDB is", *info)
	//Decrement ref count and delete group when ref count = 0
	info.refCount -= 1
	nMgr.logger.Debug("Decrement ref count to:", info.refCount)
	if info.refCount == 0 {
		nMgr.logger.Debug("RefCount became 0 for infor:", *info, " calling DeleteIPNextHopGroup in HW for groupId:", groupId)
		//Update HW state
		for _, plugin := range nMgr.plugins {
			rv := plugin.DeleteIPNextHopGroup(groupId)
			if rv < 0 {
				return errors.New("Plugin failed to delete next hop group")
			}
		}
		nMgr.logger.Debug("Next hop group delete successfull for groupId:", groupId, "calling next hop delete for all members")
		for nhIp, _ := range info.members {
			nbrInfo := &pluginCommon.PluginIPNeighborInfo{
				Address: nhIp.(string),
			}
			nbrInfo.IpType = getIpTypeFromString(nhIp.(string))
			// Specify the operation and when nextHopIntf Obj is created we will automatically get
			// missing pieces
			nMgr.logger.Debug("Delete next hop member:", nbrInfo.Address)
			nbrInfo.OperationType = NH_GROUP_DELETE_OPERATION
			err := nMgr.DeleteNextHop(nbrInfo)
			if err != nil {
				nMgr.logger.Err(fmt.Sprintln("Delete nexthop call failed when deleting nexthop group - ", nbrInfo))
			}
		}
		nMgr.logger.Debug("Members are deleted success deleting sw state for groupId:", groupId)
		//Update SW cache
		nMgr.ipNextHopGroupDB[groupId] = nil
		delete(nMgr.ipNextHopGroupDB, groupId)
	}
	return nil
}

func (nMgr *NextHopManager) UpdateNextHopGroup(groupId uint64, nhGroupMembers []*pluginCommon.NextHopGroupMemberInfo) (uint64, error) {
	var nhGroupId uint64 = groupId
	var err error
	nMgr.logger.Debug("Update Next Hop Group called for nhGroupId:", nhGroupId)
	//If refCount == 1, NhGroup will be explicitly deleted by routemgr after route is updated
	if nMgr.ipNextHopGroupDB[groupId].refCount > 1 {
		nMgr.logger.Debug("Decrement current group refcount and create new group:", groupId)
		//Decrement current group refcount and create new group
		err = nMgr.DeleteNextHopGroup(groupId)
		if err != nil {
			return pluginCommon.INVALID_NEXTHOP_ID,
				errors.New(fmt.Sprintln("Failed to delete old group, during update operation", groupId, nhGroupMembers))
		}
		nMgr.logger.Debug("DeleteNextHopGroup for groupId:", groupId, "done. Now calling CreateNextHopGroup for all members")
	}
	nhGroupId, err = nMgr.CreateNextHopGroup(nhGroupMembers)
	if err != nil {
		return pluginCommon.INVALID_NEXTHOP_ID,
			errors.New(fmt.Sprintln("Failed to create new group during update operation", groupId, nhGroupMembers))
	}
	nMgr.logger.Debug("UpdateNextHopGroup returning with nhGroupId:", nhGroupId)
	return nhGroupId, nil
}
