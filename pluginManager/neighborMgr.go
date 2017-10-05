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
	"asicdServices"
	"errors"
	"fmt"
	"net"
	"strconv"
	"syscall"
	"utils/logging"
)

const (
	INITIAL_NBR_MAP_SIZE = 2048
)

type NeighborIntf interface {
	ConvertIPStringToUint32(**pluginCommon.PluginIPNeighborInfo, string)
	NeighborExists(*pluginCommon.PluginIPNeighborInfo) bool
	RestoreNeighborDB(*pluginCommon.PluginIPNeighborInfo)
	UpdateNbrDb(*pluginCommon.PluginIPNeighborInfo)
	GetNextHopId(**pluginCommon.PluginIPNeighborInfo)
	GetMacAddr(**pluginCommon.PluginIPNeighborInfo)
	IPType() int
	CreateMacMoveNotificationMsg(*pluginCommon.PluginIPNeighborInfo) ([]byte, error)
}

type NeighborManager struct {
	nextHopMgr         *NextHopManager
	vlanMgr            *VlanManager
	l2Mgr              *L2Manager
	ipv4NeighborDB     map[uint32]IPv4NeighborInfo
	ipv4NeighborDBKeys []uint32
	ipv6NeighborDB     map[string]IPv6NeighborInfo
	ipv6NeighborDBKeys []string
	macAddrToIPNbrXref map[string][]interface{}
	plugins            []PluginIntf
	logger             *logging.Writer
	notifyChannel      chan<- []byte
}

//Neighbor manager db
var NeighborMgr NeighborManager

func UpdateIPNeighborDB(nbrInfo *pluginCommon.PluginIPNeighborInfo) {
	var nbrObj NeighborIntf
	switch nbrInfo.IpType {
	case syscall.AF_INET:
		nbrObj = &IPv4NeighborInfo{}
	case syscall.AF_INET6:
		nbrObj = &IPv6NeighborInfo{}
	}
	nbrObj.RestoreNeighborDB(nbrInfo)
}

func (nMgr *NeighborManager) Init(rsrcMgrs *ResourceManagers, bootMode int, pluginList []PluginIntf, logger *logging.Writer, notifyChannel chan<- []byte) {
	nMgr.logger = logger
	nMgr.plugins = pluginList
	nMgr.vlanMgr = rsrcMgrs.VlanManager
	nMgr.nextHopMgr = rsrcMgrs.NextHopManager
	nMgr.l2Mgr = rsrcMgrs.L2Manager
	nMgr.notifyChannel = notifyChannel
	nMgr.ipv4NeighborDB = make(map[uint32]IPv4NeighborInfo, INITIAL_NBR_MAP_SIZE)
	nMgr.ipv6NeighborDB = make(map[string]IPv6NeighborInfo, INITIAL_NBR_MAP_SIZE)
	nMgr.macAddrToIPNbrXref = make(map[string][]interface{}, INITIAL_NBR_MAP_SIZE)
	if bootMode == pluginCommon.BOOT_MODE_WARMBOOT {
		//Restore neighbor db during warm boot
		for _, plugin := range nMgr.plugins {
			if isPluginAsicDriver(plugin) == true {
				rv := plugin.RestoreIPNeighborDB()
				if rv < 0 {
					nMgr.logger.Err("Failed to restore IPv4 neighbor db during warm boot")
				}
				break
			}
		}
	}
}

func (nMgr *NeighborManager) Deinit() {
	/* Currently no-op */
}

func (nMgr *NeighborManager) GetNumV4Adjs() int {
	return len(nMgr.ipv4NeighborDB)
}

func (nMgr *NeighborManager) GetNumV6Adjs() int {
	return len(nMgr.ipv6NeighborDB)
}

func (nMgr *NeighborManager) createNeighborObject(ipAddr string, vlanId int, ifIndex int32,
	macAddr net.HardwareAddr, operation int) (NeighborIntf, *pluginCommon.PluginIPNeighborInfo, error) {
	var nbrObj NeighborIntf
	nbrInfo := &pluginCommon.PluginIPNeighborInfo{
		NeighborFlags: pluginCommon.NEIGHBOR_TYPE_FULL_SPEC_NEXTHOP,
		NextHopFlags:  uint32(0),
		IfIndex:       ifIndex,
		MacAddr:       macAddr,
		VlanId:        vlanId,
		OperationType: operation,
		Address:       ipAddr,
	}
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return nbrObj, nbrInfo, errors.New("Cannot Parse IP Address:" + ipAddr)
	}
	if ip.To4() != nil {
		nbrObj = &IPv4NeighborInfo{}
	} else {
		nbrObj = &IPv6NeighborInfo{}
	}
	// Convert IP Addr string to []uint32
	nbrObj.ConvertIPStringToUint32(&nbrInfo, ipAddr)
	// Get Ip Type
	nbrInfo.IpType = nbrObj.IPType()

	return nbrObj, nbrInfo, nil
}

func (nMgr *NeighborManager) CreateIPNeighbor(ipAddr string, vlanId int, ifIndex int32, macAddr net.HardwareAddr) error {
	// Create Neighbor Object for Addressing the modularity between ipv4 and ipv6
	nbrObj, nbrInfo, err := nMgr.createNeighborObject(ipAddr, vlanId, ifIndex, macAddr, NBR_CREATE_OPERATION)
	if err != nil {
		nMgr.logger.Err("Failed Creating Neighbor Object:", err)
		return err
	}
	// Check if neighbor already exists
	if nbrObj.NeighborExists(nbrInfo) {
		nMgr.logger.Debug("IP Neighbor", nbrInfo, "already exists. Performing an update")
		nbrInfo.OperationType = NBR_UPDATE_OPERATION
		return nMgr.UpdateIPNeighborInternal(nbrObj, &nbrInfo)
	}
	// Check if neighbor already exists
	if nbrInfo.VlanId == pluginCommon.SYS_RSVD_VLAN {
		// Adding neighbor entry on a physical port, lookup internal vlan id
		nbrInfo.VlanId = nMgr.vlanMgr.GetSysVlanContainingIf(nbrInfo.IfIndex)
		if nbrInfo.VlanId < 0 {
			return errors.New("Abort neighbor add. Failed to retrieve system reserved vlan id")
		}
	}

	// We exchange CreateNextHop and CreateIPNeighbor
	// becasue CreateNextHop needs host table but add host to
	// host table in CreateIPNeighbor
	// Get nexthop id for neighbor
	//err = nMgr.nextHopMgr.CreateNextHop(&nbrInfo)
	//if err != nil {
	//	return errors.New("Failed to create next hop for neighbor")
	//}

	//Update HW state
	for _, plugin := range nMgr.plugins {
		rv := plugin.CreateIPNeighbor(nbrInfo)
		if rv < 0 {
			return errors.New("Plugin failed to create neighbor entry")
		}
	}

	//Get nexthop id for neighbor
	err = nMgr.nextHopMgr.CreateNextHop(&nbrInfo)
	if err != nil {
		return errors.New("Failed to create next hop for neighbor")
	}

	nbrObj.UpdateNbrDb(nbrInfo)
	return nil
}

func (nMgr *NeighborManager) UpdateIPNeighborInternal(nbrObj NeighborIntf, info **pluginCommon.PluginIPNeighborInfo) error {
	nbrInfo := *info
	// Check if neighbor already exists
	if !nbrObj.NeighborExists(nbrInfo) {
		return errors.New("IP neighbor update failed, neighbor does not exist")
	}
	if nbrInfo.VlanId == pluginCommon.SYS_RSVD_VLAN {
		// Adding neighbor entry on a physical port, lookup internal vlan id
		nbrInfo.VlanId = nMgr.vlanMgr.GetSysVlanContainingIf(nbrInfo.IfIndex)
		if nbrInfo.VlanId < 0 {
			return errors.New("Abort neighbor add. Failed to retrieve system reserved vlan id")
		}
	}
	err := nMgr.nextHopMgr.UpdateNextHop(nbrInfo)
	if err != nil {
		return errors.New("Failed to update next hop for neighbor")
	}
	//Update HW state
	for _, plugin := range nMgr.plugins {
		rv := plugin.UpdateIPNeighbor(nbrInfo)
		if rv < 0 {
			return errors.New("Failed to update neighbor")
		}
	}
	// Update SW cache
	nbrObj.UpdateNbrDb(nbrInfo)
	return nil
}

func (nMgr *NeighborManager) UpdateIPNeighbor(ipAddr string, vlanId int, ifIndex int32, macAddr net.HardwareAddr) error {
	// Create Neighbor Object for Addressing the modularity between ipv4 and ipv6
	nbrObj, nbrInfo, err := nMgr.createNeighborObject(ipAddr, vlanId, ifIndex, macAddr, NBR_UPDATE_OPERATION)
	if err != nil {
		return err
	}
	return nMgr.UpdateIPNeighborInternal(nbrObj, &nbrInfo)
}

func (nMgr *NeighborManager) DeleteIPNeighbor(ipAddr string, vlanId int, ifIndex int32, macAddr net.HardwareAddr) error {
	// Create Neighbor Object for Addressing the modularity between ipv4 and ipv6
	nbrObj, nbrInfo, err := nMgr.createNeighborObject(ipAddr, vlanId, ifIndex, macAddr, NBR_DELETE_OPERATION)
	if err != nil {
		return err
	}
	// Check if neighbor already exists
	if !nbrObj.NeighborExists(nbrInfo) {
		return errors.New(fmt.Sprintln("IP", ipAddr, "neighbor delete failed, neighbor does not exist"))
	}
	nbrObj.GetNextHopId(&nbrInfo)
	nbrObj.GetMacAddr(&nbrInfo)
	//Update HW state
	for _, plugin := range nMgr.plugins {
		rv := plugin.DeleteIPNeighbor(nbrInfo)
		if rv < 0 {
			return errors.New("Failed to delete neighbor")
		}
	}
	//Delete nexthop
	err = nMgr.nextHopMgr.DeleteNextHop(nbrInfo)
	if err != nil {
		return errors.New("Failed to delete next hop for neighbor")
	}
	//Update SW cache
	nbrObj.UpdateNbrDb(nbrInfo)

	return nil
}

/*
 *  When mac move happens ProcessMacMove for NeighborManager will go ahead and reprogram neighbor table
 *  with the necessary changes.
 *  On 1 mac address we can have multiple ip address learned and we need to move all these ip's.
 */
func (nMgr *NeighborManager) ProcessMacMove(macAddr net.HardwareAddr, l2IfIndex, vlanId int32) {
	learnedIps := nMgr.macAddrToIPNbrXref[macAddr.String()]
	for _, ip := range learnedIps {
		var ipAddr string
		var nbrObj NeighborIntf
		nbrInfo := &pluginCommon.PluginIPNeighborInfo{}
		switch ip.(type) {
		case uint32:
			// ipv4 object
			nbrObj = &IPv4NeighborInfo{}
			ip1 := ip.(uint32)
			ipAddr = net.IPv4(byte((ip1>>24)&0xff), byte((ip1>>16)&0xff), byte((ip1>>8)&0xff),
				byte(ip1&0xff)).String()
		case string:
			// ipv6 object
			nbrObj = &IPv6NeighborInfo{}
			ipAddr = ip.(string)
		}
		nbrInfo.Address = ipAddr
		nbrInfo.L2IfIndex = l2IfIndex
		nbrInfo.VlanId = int(vlanId)
		//Process mac move for a directly attached IP neighbor
		nMgr.logger.Info("Updating l2 adjacency information for IP neighbor due to mac move - ",
			ip, macAddr.String(), l2IfIndex, vlanId)
		//Update Neighbor entry
		nMgr.UpdateIPNeighbor(ipAddr, int(vlanId), l2IfIndex, macAddr)
		//Send notification to update ARPd's/ND's cache
		notificationBuf, err := nbrObj.CreateMacMoveNotificationMsg(nbrInfo)
		if err != nil {
			nMgr.logger.Err("Failed to marshal IPv4 Nbr mac move message")
		}
		nMgr.notifyChannel <- notificationBuf
	}
}

func (nMgr *NeighborManager) GetIPv4Neighbor(ipAddr uint32) (*asicdServices.ArpEntryHwState, error) {
	ip := fmt.Sprintf("%d.%d.%d.%d", byte(ipAddr>>24), byte(ipAddr>>16), byte(ipAddr>>8), byte(ipAddr))
	if val, ok := nMgr.ipv4NeighborDB[ipAddr]; ok {
		nbrObj := new(asicdServices.ArpEntryHwState)
		nbrObj.IpAddr = ip
		nbrObj.MacAddr = val.MacAddr.String()
		nbrObj.Vlan = strconv.Itoa(val.VlanId)
		nbrObj.Port = strconv.Itoa(int(val.IfIndex))
		return nbrObj, nil
	} else {
		return nil, errors.New(fmt.Sprintln("Unable to locate Arp entry for neighbor -", ip))
	}
}

//FIXME: Add logic to garbage collect keys slice maintained for getbulk
func (nMgr *NeighborManager) GetBulkIPv4Neighbor(start, count int) (end, listLen int, more bool, arpList []*asicdServices.ArpEntryHwState) {
	var idx, numEntries int = 0, 0
	if len(nMgr.ipv4NeighborDB) == 0 {
		return 0, 0, false, nil
	}
	for idx = start; idx < len(nMgr.ipv4NeighborDBKeys); idx++ {
		if val, ok := nMgr.ipv4NeighborDB[nMgr.ipv4NeighborDBKeys[idx]]; ok {
			var arpObj asicdServices.ArpEntryHwState
			if numEntries == count {
				more = true
				break
			}
			ipAddr := nMgr.ipv4NeighborDBKeys[idx]
			arpObj.IpAddr = fmt.Sprintf("%d.%d.%d.%d", byte(ipAddr>>24), byte(ipAddr>>16), byte(ipAddr>>8), byte(ipAddr))
			arpObj.MacAddr = val.MacAddr.String()
			arpObj.Vlan = strconv.Itoa(val.VlanId)
			arpObj.Port = strconv.Itoa(int(val.IfIndex))
			arpList = append(arpList, &arpObj)
			numEntries += 1
		}
	}
	end = idx
	if end == len(nMgr.ipv4NeighborDBKeys) {
		more = false
	}
	return end, len(arpList), more, arpList
}

//FIXME: Add logic to garbage collect keys slice maintained for getbulk
func (nMgr *NeighborManager) GetBulkIPv6Neighbor(start, count int) (end, listLen int, more bool, ndpList []*asicdServices.NdpEntryHwState) {
	var idx, numEntries int = 0, 0
	if len(nMgr.ipv6NeighborDB) == 0 {
		return 0, 0, false, nil
	}
	for idx = start; idx < len(nMgr.ipv6NeighborDBKeys); idx++ {
		if val, ok := nMgr.ipv6NeighborDB[nMgr.ipv6NeighborDBKeys[idx]]; ok {
			var ndpObj asicdServices.NdpEntryHwState
			if numEntries == count {
				more = true
				break
			}
			ndpObj.IpAddr = nMgr.ipv6NeighborDBKeys[idx]
			ndpObj.MacAddr = val.MacAddr.String()
			ndpObj.Vlan = strconv.Itoa(val.VlanId)
			ndpObj.Port = strconv.Itoa(int(val.IfIndex))
			ndpList = append(ndpList, &ndpObj)
			numEntries += 1
		}
	}
	end = idx
	if end == len(nMgr.ipv6NeighborDBKeys) {
		more = false
	}
	return end, len(ndpList), more, ndpList
}

func (nMgr *NeighborManager) GetIPv6Neighbor(ipAddr string) (*asicdServices.NdpEntryHwState, error) {
	if val, ok := nMgr.ipv6NeighborDB[ipAddr]; ok {
		nbrObj := new(asicdServices.NdpEntryHwState)
		nbrObj.IpAddr = ipAddr
		nbrObj.MacAddr = val.MacAddr.String()
		nbrObj.Vlan = strconv.Itoa(val.VlanId)
		nbrObj.Port = strconv.Itoa(int(val.IfIndex))
		return nbrObj, nil
	} else {
		return nil, errors.New(fmt.Sprintln("Unable to locate NDP entry for neighbor -", ipAddr))
	}
}

func (nMgr *NeighborManager) HasIpAdj(macAddr string) bool {
	if _, ok := nMgr.macAddrToIPNbrXref[macAddr]; ok {
		return true
	}
	return false
}
