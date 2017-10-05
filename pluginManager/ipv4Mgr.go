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
	"encoding/json"
	"errors"
	"models/events"
	"models/objects"
	"net"
	"syscall"
	"time"
	"utils/dbutils"
	"utils/eventUtils"
)

type IPv4IntfObj struct {
	IpAddr            string
	IntfRef           string
	IfIndex           int32
	AdminState        string
	OperState         string
	LastUpEventTime   string
	NumUpEvents       int32
	LastDownEventTime string
	NumDownEvents     int32
}

type IPv4RouteObj struct {
	IPRoute
	PrefixIp      []uint8
	Mask          []uint8
	NextHopIp     []uint8
	NextHopIpType int //next hop ip type (v4/v6)
	NextHopIfType int
	Weight        int
}

type IPv4NeighborInfo struct {
	MacAddr   net.HardwareAddr
	IfIndex   int32
	NextHopId uint64
	VlanId    int
}

type IPv4NextHopInfo struct {
	macAddr   net.HardwareAddr
	ifIndex   int32
	nextHopId uint64
	vlanId    int
	refCount  int
}

func (mgr *IPv4IntfObj) ConvertIPStringToUint32(ip net.IP, ipInfo *pluginCommon.PluginIPInfo) {
	parsedIP := uint32(ip[3]) | uint32(ip[2])<<8 | uint32(ip[1])<<16 | uint32(ip[0])<<24
	ipInfo.IpAddr = append(ipInfo.IpAddr, parsedIP)
}

func (mgr *IPv4IntfObj) VerifyIPConfig(ipInfo *pluginCommon.PluginIPInfo, ip net.IP, ipNet *net.IPNet) (err error) {
	ipInfo.MaskLen, _ = ipNet.Mask.Size()
	maskLen := ipInfo.MaskLen
	if maskLen > 32 || maskLen < 0 {
		return errors.New("Invalid netmask value")
	}
	mgr.ConvertIPStringToUint32(ip.To4(), ipInfo)

	parsedIP := ipInfo.IpAddr[0]
	for _, v4Intf := range L3IntfMgr.ipv4IntfDB {
		//Verify overlapping subnet does not already exist
		dbIP, dbIPNet, err := net.ParseCIDR(v4Intf.IpAddr)
		if err != nil {
			return errors.New("Failed to parse db ip addr during IPv4Intf create")
		}
		dbIP = dbIP.To4()
		dbParsedIP := uint32(dbIP[3]) | uint32(dbIP[2])<<8 | uint32(dbIP[1])<<16 | uint32(dbIP[0])<<24
		dbMaskLen, _ := dbIPNet.Mask.Size()
		//Apply larger netmask
		var maskedIp, dbMaskedIp uint32
		if maskLen < dbMaskLen {
			dbMaskedIp = dbParsedIP & uint32(0xFFFFFFFF<<uint32(32-maskLen))
			maskedIp = parsedIP & uint32(0xFFFFFFFF<<uint32(32-maskLen))
		} else {
			dbMaskedIp = dbParsedIP & uint32(0xFFFFFFFF<<uint32(32-dbMaskLen))
			maskedIp = parsedIP & uint32(0xFFFFFFFF<<uint32(32-dbMaskLen))
		}
		if maskedIp == dbMaskedIp {
			return errors.New("Failed to create ipv4 intf, matching ipv4 intf already exists")
		}
		//Verify no ip config exists on underlying l2 intf
		if ipInfo.IfIndex == v4Intf.IfIndex {
			return errors.New("Failed to create ipv4 intf, delete " +
				"existing ipv4 intf configuration and apply new config")
		}
	}
	return nil
}

func (mgr *IPv4IntfObj) GetIntfRef() string {
	return mgr.IntfRef
}

func (mgr *IPv4IntfObj) IPType() int {
	return syscall.AF_INET
}

func (mgr *IPv4IntfObj) SetIfIndex(ifIndex int32) {
	mgr.IfIndex = ifIndex
}

func (mgr *IPv4IntfObj) SetAdminState(adminState string) {
	//Locate record
	for _, elem := range L3IntfMgr.ipv4IntfDB {
		if (elem.IpAddr == mgr.IpAddr) && (elem.IfIndex == mgr.IfIndex) {
			elem.AdminState = adminState
			break
		}
	}
}

func (mgr *IPv4IntfObj) SetOperState(operState int) {
	//Locate record
	for _, elem := range L3IntfMgr.ipv4IntfDB {
		if (elem.IpAddr == mgr.IpAddr) && (elem.IfIndex == mgr.IfIndex) {
			if operState == pluginCommon.INTF_STATE_UP {
				elem.OperState = INTF_STATE_UP
			} else {
				elem.OperState = INTF_STATE_DOWN
			}
			break
		}
	}
}

func (mgr *IPv4IntfObj) GetAdminState() string {
	var adminState string = INTF_STATE_DOWN
	//Locate record
	for _, elem := range L3IntfMgr.ipv4IntfDB {
		if (elem.IpAddr == mgr.IpAddr) && (elem.IfIndex == mgr.IfIndex) {
			adminState = elem.AdminState
			break
		}
	}
	return adminState
}

func (mgr *IPv4IntfObj) UpdateIpDb(operState string) {
	if operState == "" { // Delete the entry as update DB is called during delete
		//Locate record
		for idx, elem := range L3IntfMgr.ipv4IntfDB {
			if (elem.IpAddr == mgr.IpAddr) && (elem.IfIndex == mgr.IfIndex) {
				//Set element to nil for gc
				L3IntfMgr.ipv4IntfDB[idx] = nil
				L3IntfMgr.ipv4IntfDB = append(L3IntfMgr.ipv4IntfDB[:idx],
					L3IntfMgr.ipv4IntfDB[idx+1:]...)
				break
			}
		}
	} else {
		mgr.OperState = operState
		L3IntfMgr.ipv4IntfDB = append(L3IntfMgr.ipv4IntfDB, mgr)
	}
}

func (mgr *IPv4IntfObj) SendIpNotification(notifyType bool, operState string) {
	var msg interface{}
	var state int
	if operState == INTF_STATE_UP {
		state = 1
	} else {
		state = 0
	}
	// Send ip interface created notification
	msg = pluginCommon.IPv4IntfNotifyMsg{
		IpAddr:  mgr.IpAddr,
		IfIndex: mgr.IfIndex,
		IntfRef: mgr.IntfRef,
	}
	switch notifyType {
	case true:
		// send ip state notifcation
		L3IntfMgr.logger.Debug("Sent L3 Create IPv4 Interface msg:",
			mgr.IfIndex, mgr.IpAddr, "type:", pluginCommon.NOTIFY_IPV4INTF_CREATE)
		SendIPCreateDelNotification(msg, pluginCommon.NOTIFY_IPV4INTF_CREATE)
		SendL3IntfStateNotification(mgr.IpAddr, syscall.AF_INET, mgr.IfIndex, state)
	case false:
		//Send out state notification
		L3IntfMgr.logger.Debug("Sent L3 Delete IPv4 Interface msg:",
			mgr.IfIndex, mgr.IpAddr, "type:", pluginCommon.NOTIFY_IPV4INTF_DELETE)
		SendL3IntfStateNotification(mgr.IpAddr, syscall.AF_INET, mgr.IfIndex, 0)
		SendIPCreateDelNotification(msg, pluginCommon.NOTIFY_IPV4INTF_DELETE)
	}
}

func (mgr *IPv4RouteObj) IPType() int {
	return syscall.AF_INET
}

func (mgr *IPv4IntfObj) Init(dbHdl *dbutils.DBUtil) {
	var dbObjIPv4Intf objects.IPv4Intf
	objListIPv4Intf, err := dbHdl.GetAllObjFromDb(dbObjIPv4Intf)
	if err == nil {
		for idx := 0; idx < len(objListIPv4Intf); idx++ {
			obj := asicdServices.NewIPv4Intf()
			dbObj := objListIPv4Intf[idx].(objects.IPv4Intf)
			objects.ConvertasicdIPv4IntfObjToThrift(&dbObj, obj)
			rv, _ := L3IntfMgr.CreateIPIntf(obj.IpAddr, obj.IntfRef, obj.AdminState)
			if rv == false {
				L3IntfMgr.logger.Err("Logical Intf create failed during L3Intf init")
			}
		}
	} else {
		L3IntfMgr.logger.Err("DB Query failed during LogicalIntfConfig query: L3Intf init")
	}
}

func (mgr *IPv4IntfObj) GetObjFromInternalDb() (*pluginCommon.PluginIPInfo, error) {
	var err error
	var found bool
	var elem *IPv4IntfObj

	//Locate record
	for _, elem = range L3IntfMgr.ipv4IntfDB {
		if (elem.IpAddr == mgr.IpAddr) && (elem.IfIndex == mgr.IfIndex) {
			found = true
			break
		}
	}
	ipInfo := &pluginCommon.PluginIPInfo{}
	if found {
		ipInfo.IfIndex = elem.IfIndex
		ipInfo.IfName = elem.IntfRef
		ipInfo.IpType = syscall.AF_INET
		ipInfo.Address = elem.IpAddr
		dbIP, dbIPNet, _ := net.ParseCIDR(elem.IpAddr)
		mgr.ConvertIPStringToUint32(dbIP.To4(), ipInfo)
		ipInfo.MaskLen, _ = dbIPNet.Mask.Size()
	} else {
		err = errors.New("Unable to locate IP intf obj in internal DB")
	}
	return ipInfo, err
}

func (mgr *IPv4IntfObj) DeInit(plugin PluginIntf) {
	for _, info := range L3IntfMgr.ipv4IntfDB {
		if info != nil { // do we need to move this to interface or not
			ip, ipNet, err := net.ParseCIDR(info.IpAddr)
			if err != nil {
				L3IntfMgr.logger.Err("Failed to cleanup ipv4 interfaces")
				continue
			}
			ip = ip.To4()

			ipInfo := &pluginCommon.PluginIPInfo{}
			mgr.ConvertIPStringToUint32(ip.To4(), ipInfo)
			ipInfo.MaskLen, _ = ipNet.Mask.Size()
			ipInfo.IfIndex = info.IfIndex
			ipInfo.VlanId = pluginCommon.SYS_RSVD_VLAN
			ipInfo.IfName = info.IntfRef
			ipInfo.Address = info.IpAddr
			rv := plugin.DeleteIPIntf(ipInfo)
			if rv < 0 {
				L3IntfMgr.logger.Err("Failed to cleanup ipv4 interfaces")
				continue
			}
		}
	}
}

func (mgr *IPv4IntfObj) ProcessL2IntfStateChange(ifIndex int32, state int) {
	var newOperState string
	var newState int
	L3IntfMgr.dbMutex.Lock()
	for _, intfInfo := range L3IntfMgr.ipv4IntfDB {
		if intfInfo.IfIndex == ifIndex {
			evtKey := events.IPv4IntfKey{
				IntfRef: intfInfo.IntfRef,
			}
			if (intfInfo.AdminState == INTF_STATE_UP) && (state == pluginCommon.INTF_STATE_UP) {
				newOperState = INTF_STATE_UP
				newState = pluginCommon.INTF_STATE_UP
			} else {
				newOperState = INTF_STATE_DOWN
				newState = pluginCommon.INTF_STATE_DOWN
			}
			if newOperState != intfInfo.OperState {
				//Update state in sw cache
				switch newState {
				case pluginCommon.INTF_STATE_UP:
					intfInfo.OperState = INTF_STATE_UP
					intfInfo.NumUpEvents += 1
					intfInfo.LastUpEventTime = time.Now().String()
					txEvent := eventUtils.TxEvent{
						EventId:        events.IPv4IntfOperStateUp,
						Key:            evtKey,
						AdditionalInfo: "",
						AdditionalData: nil,
					}
					err := eventUtils.PublishEvents(&txEvent)
					if err != nil {
						L3IntfMgr.logger.Err("Error publishing ipv4 intf oper state up events")
					}
				case pluginCommon.INTF_STATE_DOWN:
					intfInfo.OperState = INTF_STATE_DOWN
					intfInfo.NumDownEvents += 1
					intfInfo.LastDownEventTime = time.Now().String()
					txEvent := eventUtils.TxEvent{
						EventId:        events.IPv4IntfOperStateDown,
						Key:            evtKey,
						AdditionalInfo: "",
						AdditionalData: nil,
					}
					err := eventUtils.PublishEvents(&txEvent)
					if err != nil {
						L3IntfMgr.logger.Err("Error publishing ipv4 intf oper state down events")
					}
				}
				SendL3IntfStateNotification(intfInfo.IpAddr, pluginCommon.IP_TYPE_IPV4, ifIndex, newState)
			}
		}
	}
	L3IntfMgr.dbMutex.Unlock()
}

func (mgr *IPv4NeighborInfo) ConvertIPStringToUint32(info **pluginCommon.PluginIPNeighborInfo, ipAddr string) {
	nbrInfo := *info
	ip := net.ParseIP(ipAddr)
	ip = ip.To4()
	parsedIP := uint32(ip[3]) | uint32(ip[2])<<8 | uint32(ip[1])<<16 | uint32(ip[0])<<24
	nbrInfo.IpAddr = append(nbrInfo.IpAddr, parsedIP)
}

func (mgr *IPv4NeighborInfo) RestoreNeighborDB(nbrInfo *pluginCommon.PluginIPNeighborInfo) {
	NeighborMgr.ipv4NeighborDB[nbrInfo.IpAddr[0]] = IPv4NeighborInfo{
		MacAddr:   nbrInfo.MacAddr,
		IfIndex:   nbrInfo.IfIndex,
		NextHopId: nbrInfo.NextHopId,
	}
}

func (mgr *IPv4NeighborInfo) NeighborExists(nbrInfo *pluginCommon.PluginIPNeighborInfo) bool {
	if _, ok := NeighborMgr.ipv4NeighborDB[nbrInfo.IpAddr[0]]; ok {
		return true
	}
	return false
}

func (mgr *IPv4NeighborInfo) UpdateNbrDb(nbrInfo *pluginCommon.PluginIPNeighborInfo) {
	// Update SW cache
	if nbrInfo.OperationType == NBR_DELETE_OPERATION ||
		nbrInfo.OperationType == RT_NBR_DELETE_OPERATION ||
		nbrInfo.OperationType == NH_GROUP_DELETE_OPERATION {
		delete(NeighborMgr.ipv4NeighborDB, nbrInfo.IpAddr[0])
		//Delete cross reference used for mac move lookups
		learnedIps := NeighborMgr.macAddrToIPNbrXref[nbrInfo.MacAddr.String()]
		for idx, ip := range learnedIps {
			switch ip.(type) {
			case uint32:
				if ip.(uint32) == nbrInfo.IpAddr[0] {
					learnedIps = append(learnedIps[:idx], learnedIps[idx+1:]...)
					break
				}
			}
		}
		NeighborMgr.macAddrToIPNbrXref[nbrInfo.MacAddr.String()] = learnedIps
		if len(learnedIps) == 0 {
			//During delete we need to go ahead and remove any left over mac-ip cross reference software entry
			NeighborMgr.l2Mgr.RemoveMacIpAdjEntry(nbrInfo.MacAddr.String())
			delete(NeighborMgr.macAddrToIPNbrXref, nbrInfo.MacAddr.String())
		}
		//Delete entry from key cache
		for idx, val := range NeighborMgr.ipv4NeighborDBKeys {
			if val == nbrInfo.IpAddr[0] {
				NeighborMgr.ipv4NeighborDBKeys =
					append(NeighborMgr.ipv4NeighborDBKeys[:idx], NeighborMgr.ipv4NeighborDBKeys[idx+1:]...)
				break
			}
		}
	} else {
		NeighborMgr.ipv4NeighborDB[nbrInfo.IpAddr[0]] = IPv4NeighborInfo{
			MacAddr:   nbrInfo.MacAddr,
			IfIndex:   nbrInfo.IfIndex,
			NextHopId: nbrInfo.NextHopId,
			VlanId:    nbrInfo.VlanId,
		}

		//Insert into key cache only if it is create opertaion
		if nbrInfo.OperationType == NBR_CREATE_OPERATION ||
			nbrInfo.OperationType == RT_NBR_CREATE_OPERATION ||
			nbrInfo.OperationType == NH_GROUP_CREATE_OPERATION ||
			nbrInfo.OperationType == NH_GROUP_UPDATE_OPERATION {
			NeighborMgr.ipv4NeighborDBKeys = append(NeighborMgr.ipv4NeighborDBKeys, nbrInfo.IpAddr[0])
			//Update cross reference map used for mac move lookups by appending new ip' only when
			//arp or ndp is doing nbr entry create
			NeighborMgr.macAddrToIPNbrXref[nbrInfo.MacAddr.String()] =
				append(NeighborMgr.macAddrToIPNbrXref[nbrInfo.MacAddr.String()],
					nbrInfo.IpAddr[0])
		}
	}
}

func (mgr *IPv4NeighborInfo) IPType() int {
	return syscall.AF_INET
}

func (mgr *IPv4NeighborInfo) GetNextHopId(info **pluginCommon.PluginIPNeighborInfo) {
	nbrInfo := *info
	nbrInfo.NextHopId = NeighborMgr.ipv4NeighborDB[nbrInfo.IpAddr[0]].NextHopId
}

func (mgr *IPv4NeighborInfo) GetMacAddr(info **pluginCommon.PluginIPNeighborInfo) {
	nbrInfo := *info
	nbrInfo.MacAddr = NeighborMgr.ipv4NeighborDB[nbrInfo.IpAddr[0]].MacAddr
}

func (mgr *IPv4NeighborInfo) CreateMacMoveNotificationMsg(nbrInfo *pluginCommon.PluginIPNeighborInfo) (notificationBuf []byte, err error) {
	msg := pluginCommon.IPv4NbrMacMoveNotifyMsg{
		IpAddr:  nbrInfo.Address,
		IfIndex: nbrInfo.L2IfIndex,
		VlanId:  int32(nbrInfo.VlanId),
	}
	msgBuf, err := json.Marshal(msg)
	if err != nil {
		NextHopMgr.logger.Err("Failed to marshal IPv4 Nbr mac move message")
		return notificationBuf, errors.New("Failed to marshal IPv4 Nbr mac move message")
	}
	notification := pluginCommon.AsicdNotification{
		uint8(pluginCommon.NOTIFY_IPV4NBR_MAC_MOVE),
		msgBuf,
	}
	NextHopMgr.logger.Debug("Sending IPV4 Mac Move Notification, msg:", msg)
	notificationBuf, err = json.Marshal(notification)
	return notificationBuf, nil
}

func (mgr *IPv4NextHopInfo) ConvertIPStringToUint32(info **pluginCommon.PluginIPNeighborInfo) {
	nbrInfo := *info
	ipAddr := nbrInfo.Address
	ip := net.ParseIP(ipAddr)
	ip = ip.To4()
	if ip == nil {
		NextHopMgr.logger.Err("IPv4NextHopInfo:ConvertIPStringToUint32, ip is nil for neighborinfo: ", *info, " ipAddr:", ipAddr)
		panic("IPv4NextHopInfo:ConvertIPStringToUint32, ip is nil for neighborinfo: ")
		//return
	}
	parsedIP := uint32(ip[3]) | uint32(ip[2])<<8 | uint32(ip[1])<<16 | uint32(ip[0])<<24
	nbrInfo.IpAddr = append(nbrInfo.IpAddr, parsedIP)
}

func (mgr *IPv4NextHopInfo) CollectInfoFromDb(info **pluginCommon.PluginIPNeighborInfo) {
	nbrInfo := *info
	nhInfo, exists := NextHopMgr.ipv4NextHopDB[nbrInfo.IpAddr[0]]
	if !exists {
		return
	}
	nbrInfo.VlanId = nhInfo.vlanId
	nbrInfo.IfIndex = nhInfo.ifIndex
	nbrInfo.MacAddr = nhInfo.macAddr
}

func (mgr *IPv4NextHopInfo) NextHopExists(entry **pluginCommon.PluginIPNeighborInfo) bool {
	nbrInfo := *entry
	// @TODO: fix this for other operation types
	if info, ok := NextHopMgr.ipv4NextHopDB[nbrInfo.IpAddr[0]]; ok {
		if nbrInfo.OperationType == RT_NBR_CREATE_OPERATION || nbrInfo.OperationType == NH_GROUP_CREATE_OPERATION ||
			nbrInfo.OperationType == NH_GROUP_UPDATE_OPERATION || nbrInfo.OperationType == NBR_CREATE_OPERATION {
			NextHopMgr.logger.Debug("IPv4 next hop already exists. Incrementing ref count for", nbrInfo.Address)
			info.refCount += 1
			nbrInfo.NextHopId = info.nextHopId
		}
		return true
	}
	return false
}

func (mgr *IPv4NextHopInfo) UpdateNhDb(nbrInfo *pluginCommon.PluginIPNeighborInfo) {
	if nbrInfo.OperationType == NBR_DELETE_OPERATION || nbrInfo.OperationType == NH_GROUP_DELETE_OPERATION ||
		nbrInfo.OperationType == RT_NBR_DELETE_OPERATION {
		//Update SW cache
		delete(NextHopMgr.ipv4NextHopDB, nbrInfo.IpAddr[0])
	} else {
		entry, exists := NextHopMgr.ipv4NextHopDB[nbrInfo.IpAddr[0]]
		if !exists {
			NextHopMgr.ipv4NextHopDB[nbrInfo.IpAddr[0]] = &IPv4NextHopInfo{
				macAddr:   nbrInfo.MacAddr,
				ifIndex:   nbrInfo.IfIndex,
				nextHopId: nbrInfo.NextHopId,
				vlanId:    nbrInfo.VlanId,
				refCount:  1,
			}
		} else {
			entry.macAddr = nbrInfo.MacAddr
			entry.ifIndex = nbrInfo.IfIndex
			entry.nextHopId = nbrInfo.NextHopId
			entry.vlanId = nbrInfo.VlanId
		}
	}
}

func (mgr *IPv4NextHopInfo) NextHopId(info **pluginCommon.PluginIPNeighborInfo) {
	nbrInfo := *info
	nbrInfo.NextHopId = NextHopMgr.ipv4NextHopDB[nbrInfo.IpAddr[0]].nextHopId
}

func (mgr *IPv4NextHopInfo) CheckRefCount(nbrInfo *pluginCommon.PluginIPNeighborInfo) bool {
	info := NextHopMgr.ipv4NextHopDB[nbrInfo.IpAddr[0]]
	info.refCount -= 1
	NextHopMgr.ipv4NextHopDB[nbrInfo.IpAddr[0]] = info
	if info.refCount == 0 {
		return true
	}
	return false
}

func (mgr *IPv4NextHopInfo) GetRefCount(nbrInfo *pluginCommon.PluginIPNeighborInfo) int {
	info, exists := NextHopMgr.ipv4NextHopDB[nbrInfo.IpAddr[0]]
	if exists {
		return info.refCount
	}
	return 0
}
