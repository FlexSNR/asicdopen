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
	"bytes"
	"encoding/binary"
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

type IPv6IntfObj struct {
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

type IPv6RouteObj struct {
	IPRoute
	PrefixIp      []uint8
	Mask          []uint8
	NextHopIp     []uint8
	NextHopIpType int //next hop ip type (v4/v6)
	NextHopIfType int
	Weight        int
}

type IPv6NeighborInfo struct {
	MacAddr   net.HardwareAddr
	IfIndex   int32
	NextHopId uint64
	VlanId    int
}

type IPv6NextHopInfo struct {
	macAddr   net.HardwareAddr
	ifIndex   int32
	nextHopId uint64
	vlanId    int
	refCount  int
}

func (mgr *IPv6IntfObj) ConvertIPStringToUint32(ip net.IP, ipInfo *pluginCommon.PluginIPInfo) {
	var parsedIp [4]uint32
	parsedIp[0] = uint32(ip[3]) | uint32(ip[2])<<8 | uint32(ip[1])<<16 | uint32(ip[0])<<24
	parsedIp[1] = uint32(ip[7]) | uint32(ip[6])<<8 | uint32(ip[5])<<16 | uint32(ip[4])<<24
	parsedIp[2] = uint32(ip[11]) | uint32(ip[10])<<8 | uint32(ip[9])<<16 | uint32(ip[8])<<24
	parsedIp[3] = uint32(ip[15]) | uint32(ip[14])<<8 | uint32(ip[13])<<16 | uint32(ip[12])<<24
	for idx, _ := range parsedIp {
		ipInfo.IpAddr = append(ipInfo.IpAddr, parsedIp[idx])
	}
	if ip.IsLinkLocalUnicast() {
		ipInfo.IPv6Type = LINK_LOCAL_IP
	}
}

func (mgr *IPv6IntfObj) VerifyIPConfig(ipInfo *pluginCommon.PluginIPInfo, ip net.IP, ipNet *net.IPNet) (err error) {
	ipInfo.MaskLen, _ = ipNet.Mask.Size()
	maskLen := ipInfo.MaskLen
	if maskLen > 128 || maskLen < 0 {
		return errors.New("Invalid netmask value")
	}

	mgr.ConvertIPStringToUint32(ip.To16(), ipInfo)
	for _, v6Intf := range L3IntfMgr.ipv6IntfDB {
		//Verify overlapping subnet does not already exist
		dbIP, dbIPNet, err := net.ParseCIDR(v6Intf.IpAddr)
		if err != nil {
			return errors.New("Failed to parse db ip addr during IPv6Intf create")
		}
		dbMaskLen, _ := dbIPNet.Mask.Size()
		largerMask := dbIPNet.Mask
		if maskLen > dbMaskLen {
			largerMask = ipNet.Mask
		}
		if dbIP.IsLinkLocalUnicast() && ip.IsLinkLocalUnicast() {
			// when it is link local ip do not compare largerMask.. rather compare last 80 bits
			// 48 bits for Mac + 32 bit for ifIndex = 80 bits
			// ipv6 is 128 which is 16 bytes, hence for 80 bits you need last 10 bytes...
			if bytes.Equal(ip[6:], dbIP[6:]) {
				return errors.New("Failed to create ipv6 link local intf for " + ip.String() +
					" as matching ipv6 intf:" + v6Intf.IpAddr + "already exists")
			}
		} else {
			if bytes.Equal(ip.Mask(largerMask), dbIP.Mask(largerMask)) {
				return errors.New("Failed to create ipv6 intf for " + v6Intf.IpAddr +
					" as matching ipv6 intf already exists")
			}
		}
		//Verify no ip config exists on underlying l2 intf & the ip address is not link local ip
		if ipInfo.IfIndex == v6Intf.IfIndex && !ip.IsLinkLocalUnicast() {
			return errors.New("Failed to create ipv6 intf, delete " +
				"existing ipv6 intf configuration and apply new config")
		}
	}
	return nil
}

func (mgr *IPv6IntfObj) GetIntfRef() string {
	return mgr.IntfRef
}

func (mgr *IPv6IntfObj) IPType() int {
	return syscall.AF_INET6
}
func (mgr *IPv6IntfObj) SetIfIndex(ifIndex int32) {
	mgr.IfIndex = ifIndex
}

func (mgr *IPv6IntfObj) SetAdminState(adminState string) {
	//Locate record
	for _, elem := range L3IntfMgr.ipv6IntfDB {
		if (elem.IpAddr == mgr.IpAddr) && (elem.IfIndex == mgr.IfIndex) {
			elem.AdminState = adminState
			break
		}
	}
}

func (mgr *IPv6IntfObj) GetAdminState() string {
	var adminState string = INTF_STATE_DOWN
	//Locate record
	for _, elem := range L3IntfMgr.ipv6IntfDB {
		if (elem.IpAddr == mgr.IpAddr) && (elem.IfIndex == mgr.IfIndex) {
			adminState = elem.AdminState
			break
		}
	}
	return adminState
}

func (mgr *IPv6IntfObj) SetOperState(operState int) {
	//Locate record
	for _, elem := range L3IntfMgr.ipv6IntfDB {
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

func (mgr *IPv6IntfObj) UpdateIpDb(operState string) {
	if operState == "" {
		// Delete the entry as update DB is called during delete
		//Locate record
		for idx, elem := range L3IntfMgr.ipv6IntfDB {
			if (elem.IpAddr == mgr.IpAddr) && (elem.IfIndex == mgr.IfIndex) {
				//Set element to nil for gc
				L3IntfMgr.ipv6IntfDB[idx] = nil
				L3IntfMgr.ipv6IntfDB = append(L3IntfMgr.ipv6IntfDB[:idx],
					L3IntfMgr.ipv6IntfDB[idx+1:]...)
				break
			}
		}
	} else {
		mgr.OperState = operState
		L3IntfMgr.ipv6IntfDB = append(L3IntfMgr.ipv6IntfDB, mgr)
	}
}

func (mgr *IPv6IntfObj) SendIpNotification(notifyType bool, operState string) {
	var msg interface{}
	var state int
	if operState == INTF_STATE_UP {
		state = 1
	} else {
		state = 0
	}
	// Send ip interface created notification
	msg = pluginCommon.IPv6IntfNotifyMsg{
		IpAddr:  mgr.IpAddr,
		IfIndex: mgr.IfIndex,
		IntfRef: mgr.IntfRef,
	}
	switch notifyType {
	case true:
		// send ip state notifcation
		L3IntfMgr.logger.Debug("Sent L3 Create IPv6 Interface msg:",
			mgr.IfIndex, mgr.IpAddr, "type:", pluginCommon.NOTIFY_IPV6INTF_CREATE)
		SendIPCreateDelNotification(msg, pluginCommon.NOTIFY_IPV6INTF_CREATE)
		SendL3IntfStateNotification(mgr.IpAddr, syscall.AF_INET6, mgr.IfIndex, state)
	case false:
		//Send out state notification
		L3IntfMgr.logger.Debug("Sent L3 Delete IPv6 Interface msg:",
			mgr.IfIndex, mgr.IpAddr, "type:", pluginCommon.NOTIFY_IPV6INTF_DELETE)
		SendL3IntfStateNotification(mgr.IpAddr, syscall.AF_INET6, mgr.IfIndex, 0)
		SendIPCreateDelNotification(msg, pluginCommon.NOTIFY_IPV6INTF_DELETE)
	}
}
func (mgr *IPv6RouteObj) IPType() int {
	return syscall.AF_INET6
}
func (mgr *IPv6IntfObj) Init(dbHdl *dbutils.DBUtil) {
	var dbObjIPv6Intf objects.IPv6Intf
	objListIPv6Intf, err := dbHdl.GetAllObjFromDb(dbObjIPv6Intf)
	L3IntfMgr.logger.Info("after GetAllObjFromDb(dbObjIPv6Intf):err:", err, " len(objListIPv6Intf) = ", len(objListIPv6Intf))
	if err == nil {
		for idx := 0; idx < len(objListIPv6Intf); idx++ {
			obj := asicdServices.NewIPv6Intf()
			dbObj := objListIPv6Intf[idx].(objects.IPv6Intf)
			objects.ConvertasicdIPv6IntfObjToThrift(&dbObj, obj)
			L3IntfMgr.logger.Debug("Creating IP GS on restart", obj.IpAddr, obj.IntfRef)
			if obj.IpAddr != "" {
				rv, _ := L3IntfMgr.CreateIPIntf(obj.IpAddr, obj.IntfRef, obj.AdminState)
				if rv == false {
					L3IntfMgr.logger.Err("IPv6 Global Intf create failed during L3Intf init")
				}
			}
		}
	} else {
		L3IntfMgr.logger.Err("DB Query failed during IPv6IntfObj query: L3Intf init")
	}

	var dbObjLSIPv6Intf objects.LinkScopeIpState
	dbObjList, err := dbHdl.GetAllObjFromDb(dbObjLSIPv6Intf)
	L3IntfMgr.logger.Info("IPV6: LS after GetAllObjFromDb(dbObjLSIPv6Intf) err:", err, "len(dbObjList) = ", len(dbObjList))
	if err != nil {
		L3IntfMgr.logger.Err("DB Query failed for Link Scope IP Address during IPV6 Init error:", err)
		return
	}
	for _, entry := range dbObjList {
		obj := asicdServices.NewLinkScopeIpState()
		dbObj := entry.(objects.LinkScopeIpState)
		objects.ConvertasicdLinkScopeIpStateObjToThrift(&dbObj, obj)
		L3IntfMgr.logger.Debug("Creating IP LS on restart", obj.LinkScopeIp, obj.IntfRef)
		rv, _ := L3IntfMgr.CreateIPIntf(obj.LinkScopeIp, obj.IntfRef, INTF_STATE_UP)
		if rv == true {
			//@TODO: jgheewala CREATE AN EVENT FOR FAILURE
		} else {
			L3IntfMgr.dbMutex.Lock()
			L3IntfMgr.updatelinkLocalEntry(obj.LinkScopeIp, obj.IntfRef, true)
			L3IntfMgr.dbMutex.Unlock()
		}
	}
}

func (mgr *IPv6IntfObj) GetObjFromInternalDb() (*pluginCommon.PluginIPInfo, error) {
	var err error
	var found bool
	var elem *IPv6IntfObj

	//Locate record
	for _, elem = range L3IntfMgr.ipv6IntfDB {
		if (elem.IpAddr == mgr.IpAddr) && (elem.IfIndex == mgr.IfIndex) {
			found = true
			break
		}
	}
	ipInfo := &pluginCommon.PluginIPInfo{}
	if found {
		ipInfo.IfIndex = elem.IfIndex
		ipInfo.IfName = elem.IntfRef
		ipInfo.IpType = syscall.AF_INET6
		ipInfo.Address = elem.IpAddr
		dbIP, dbIPNet, _ := net.ParseCIDR(elem.IpAddr)
		mgr.ConvertIPStringToUint32(dbIP.To16(), ipInfo)
		ipInfo.MaskLen, _ = dbIPNet.Mask.Size()
	} else {
		err = errors.New("Unable to locate IP intf obj in internal DB")
	}
	return ipInfo, err
}

func (mgr *IPv6IntfObj) DeInit(plugin PluginIntf) {
	for _, info := range L3IntfMgr.ipv6IntfDB {
		if info != nil { // do we need to move this to interface or not
			ip, ipNet, err := net.ParseCIDR(info.IpAddr)
			if err != nil {
				L3IntfMgr.logger.Err("Failed to cleanup ipv6 interfaces")
				continue
			}
			ip = ip.To16()

			ipInfo := &pluginCommon.PluginIPInfo{}
			mgr.ConvertIPStringToUint32(ip.To16(), ipInfo)
			ipInfo.MaskLen, _ = ipNet.Mask.Size()
			ipInfo.IfIndex = info.IfIndex
			ipInfo.VlanId = pluginCommon.SYS_RSVD_VLAN
			ipInfo.IfName = info.IntfRef
			ipInfo.Address = info.IpAddr
			rv := plugin.DeleteIPIntf(ipInfo)
			if rv < 0 {
				L3IntfMgr.logger.Err("Failed to cleanup ipv6 interfaces")
				continue
			}
			//@TODO: do we need to delete entry from linkscopeIp
		}
	}
}

func (mgr *IPv6IntfObj) ProcessL2IntfStateChange(ifIndex int32, state int) {
	var newOperState string
	var newState int
	L3IntfMgr.dbMutex.Lock()
	for _, intfInfo := range L3IntfMgr.ipv6IntfDB {
		if intfInfo.IfIndex == ifIndex {
			evtKey := events.IPv6IntfKey{
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
						EventId:        events.IPv6IntfOperStateUp,
						Key:            evtKey,
						AdditionalInfo: "",
						AdditionalData: nil,
					}
					err := eventUtils.PublishEvents(&txEvent)
					if err != nil {
						L3IntfMgr.logger.Err("Error publishing ipv6 intf oper state up events")
					}
				case pluginCommon.INTF_STATE_DOWN:
					intfInfo.OperState = INTF_STATE_DOWN
					intfInfo.NumDownEvents += 1
					intfInfo.LastDownEventTime = time.Now().String()
					txEvent := eventUtils.TxEvent{
						EventId:        events.IPv6IntfOperStateDown,
						Key:            evtKey,
						AdditionalInfo: "",
						AdditionalData: nil,
					}
					err := eventUtils.PublishEvents(&txEvent)
					if err != nil {
						L3IntfMgr.logger.Err("Error publishing ipv6 intf oper state down events")
					}
				}
				SendL3IntfStateNotification(intfInfo.IpAddr, pluginCommon.IP_TYPE_IPV6, ifIndex, state)
			}
		}
	}
	L3IntfMgr.dbMutex.Unlock()
}

func (mgr *IPv6NeighborInfo) ConvertIPStringToUint32(info **pluginCommon.PluginIPNeighborInfo, ipAddr string) {
	nbrInfo := *info
	var parsedIp [4]uint32
	ip := net.ParseIP(ipAddr)
	ip = ip.To16()
	parsedIp[0] = uint32(ip[3]) | uint32(ip[2])<<8 | uint32(ip[1])<<16 | uint32(ip[0])<<24
	parsedIp[1] = uint32(ip[7]) | uint32(ip[6])<<8 | uint32(ip[5])<<16 | uint32(ip[4])<<24
	parsedIp[2] = uint32(ip[11]) | uint32(ip[10])<<8 | uint32(ip[9])<<16 | uint32(ip[8])<<24
	parsedIp[3] = uint32(ip[15]) | uint32(ip[14])<<8 | uint32(ip[13])<<16 | uint32(ip[12])<<24
	for idx, _ := range parsedIp {
		nbrInfo.IpAddr = append(nbrInfo.IpAddr, parsedIp[idx])
	}
}

func (mgr *IPv6NeighborInfo) RestoreNeighborDB(nbrInfo *pluginCommon.PluginIPNeighborInfo) {
	var temp []byte
	temp = make([]byte, 16)
	binary.BigEndian.PutUint32(temp[:4], nbrInfo.IpAddr[0])
	binary.BigEndian.PutUint32(temp[4:8], nbrInfo.IpAddr[1])
	binary.BigEndian.PutUint32(temp[8:12], nbrInfo.IpAddr[2])
	binary.BigEndian.PutUint32(temp[12:], nbrInfo.IpAddr[3])
	ip := net.IP(temp)
	NeighborMgr.ipv6NeighborDB[ip.String()] = IPv6NeighborInfo{
		MacAddr:   nbrInfo.MacAddr,
		IfIndex:   nbrInfo.IfIndex,
		NextHopId: nbrInfo.NextHopId,
	}
}

func (mgr *IPv6NeighborInfo) NeighborExists(nbrInfo *pluginCommon.PluginIPNeighborInfo) bool {
	if _, ok := NeighborMgr.ipv6NeighborDB[nbrInfo.Address]; ok {
		return true
	}
	return false
}

func (mgr *IPv6NeighborInfo) UpdateNbrDb(nbrInfo *pluginCommon.PluginIPNeighborInfo) {
	// Update SW cache
	if nbrInfo.OperationType == NBR_DELETE_OPERATION {
		delete(NeighborMgr.ipv6NeighborDB, nbrInfo.Address)
		//Delete cross reference used for mac move lookups
		learnedIps := NeighborMgr.macAddrToIPNbrXref[nbrInfo.MacAddr.String()]
		for idx, ip := range learnedIps {
			switch ip.(type) {
			case string:
				if ip.(string) == nbrInfo.Address {
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
		for idx, val := range NeighborMgr.ipv6NeighborDBKeys {
			if val == nbrInfo.Address {
				NeighborMgr.ipv6NeighborDBKeys =
					append(NeighborMgr.ipv6NeighborDBKeys[:idx], NeighborMgr.ipv6NeighborDBKeys[idx+1:]...)
				break
			}
		}
	} else {
		NeighborMgr.ipv6NeighborDB[nbrInfo.Address] = IPv6NeighborInfo{
			MacAddr:   nbrInfo.MacAddr,
			IfIndex:   nbrInfo.IfIndex,
			NextHopId: nbrInfo.NextHopId,
			VlanId:    nbrInfo.VlanId,
		}
		//Insert into key cache only if it is create opertaion
		if nbrInfo.OperationType == NBR_CREATE_OPERATION {
			NeighborMgr.ipv6NeighborDBKeys = append(NeighborMgr.ipv6NeighborDBKeys, nbrInfo.Address)
			//Update cross reference map used for mac move lookups by appending new ip' only when
			//arp or ndp is doing nbr entry create
			NeighborMgr.macAddrToIPNbrXref[nbrInfo.MacAddr.String()] =
				append(NeighborMgr.macAddrToIPNbrXref[nbrInfo.MacAddr.String()],
					nbrInfo.Address)
		}
	}
}

func (mgr *IPv6NeighborInfo) IPType() int {
	return syscall.AF_INET6
}

func (mgr *IPv6NeighborInfo) GetNextHopId(info **pluginCommon.PluginIPNeighborInfo) {
	nbrInfo := *info
	nbrInfo.NextHopId = NeighborMgr.ipv6NeighborDB[nbrInfo.Address].NextHopId
}

func (mgr *IPv6NeighborInfo) GetMacAddr(info **pluginCommon.PluginIPNeighborInfo) {
	nbrInfo := *info
	nbrInfo.MacAddr = NeighborMgr.ipv6NeighborDB[nbrInfo.Address].MacAddr
}

func (mgr *IPv6NeighborInfo) CreateMacMoveNotificationMsg(nbrInfo *pluginCommon.PluginIPNeighborInfo) (notificationBuf []byte, err error) {
	msg := pluginCommon.IPv6NbrMacMoveNotifyMsg{
		IpAddr:  nbrInfo.Address,
		IfIndex: nbrInfo.L2IfIndex,
		VlanId:  int32(nbrInfo.VlanId),
	}
	msgBuf, err := json.Marshal(msg)
	if err != nil {
		NextHopMgr.logger.Err("Failed to marshal IPv6 Nbr mac move message")
		return notificationBuf, errors.New("Failed to marshal IPv6 Nbr mac move message")
	}
	notification := pluginCommon.AsicdNotification{
		uint8(pluginCommon.NOTIFY_IPV6NBR_MAC_MOVE),
		msgBuf,
	}
	NextHopMgr.logger.Debug("Sending IPV6 Mac Move Notification, msg:", msg)
	notificationBuf, err = json.Marshal(notification)
	return notificationBuf, nil

}

func (mgr *IPv6NextHopInfo) ConvertIPStringToUint32(info **pluginCommon.PluginIPNeighborInfo) {
	nbrInfo := *info
	ipAddr := nbrInfo.Address
	var parsedIp [4]uint32
	ip := net.ParseIP(ipAddr)
	ip = ip.To16()
	if ip == nil {
		NextHopMgr.logger.Err("IPv6NextHopInfo:ConvertIPStringToUint32, ip is nil for neighborinfo: ", *info, " ipAddr:", ipAddr)
		panic("IPv6NextHopInfo:ConvertIPStringToUint32, ip is nil for neighborinfo: ")
		//return
	}
	parsedIp[0] = uint32(ip[3]) | uint32(ip[2])<<8 | uint32(ip[1])<<16 | uint32(ip[0])<<24
	parsedIp[1] = uint32(ip[7]) | uint32(ip[6])<<8 | uint32(ip[5])<<16 | uint32(ip[4])<<24
	parsedIp[2] = uint32(ip[11]) | uint32(ip[10])<<8 | uint32(ip[9])<<16 | uint32(ip[8])<<24
	parsedIp[3] = uint32(ip[15]) | uint32(ip[14])<<8 | uint32(ip[13])<<16 | uint32(ip[12])<<24
	for idx, _ := range parsedIp {
		nbrInfo.IpAddr = append(nbrInfo.IpAddr, parsedIp[idx])
	}
}

func (mgr *IPv6NextHopInfo) CollectInfoFromDb(info **pluginCommon.PluginIPNeighborInfo) {
	nbrInfo := *info
	nhInfo, exists := NextHopMgr.ipv6NextHopDB[nbrInfo.Address]
	if !exists {
		return
	}
	nbrInfo.VlanId = nhInfo.vlanId
	nbrInfo.IfIndex = nhInfo.ifIndex
	nbrInfo.MacAddr = nhInfo.macAddr
}

func (mgr *IPv6NextHopInfo) NextHopExists(entry **pluginCommon.PluginIPNeighborInfo) bool {
	nbrInfo := *entry
	// @TODO: fix this for other operation types
	if info, ok := NextHopMgr.ipv6NextHopDB[nbrInfo.Address]; ok {
		if nbrInfo.OperationType == RT_NBR_CREATE_OPERATION || nbrInfo.OperationType == NH_GROUP_CREATE_OPERATION ||
			nbrInfo.OperationType == NH_GROUP_UPDATE_OPERATION || nbrInfo.OperationType == NBR_CREATE_OPERATION {
			NextHopMgr.logger.Debug("IPv6 next hop already exists. Incrementing ref count for:", nbrInfo.Address)
			info.refCount += 1
			nbrInfo.NextHopId = info.nextHopId
		}
		return true
	}
	return false
}

func (mgr *IPv6NextHopInfo) UpdateNhDb(nbrInfo *pluginCommon.PluginIPNeighborInfo) {
	if nbrInfo.OperationType == NBR_DELETE_OPERATION || nbrInfo.OperationType == NH_GROUP_DELETE_OPERATION ||
		nbrInfo.OperationType == RT_NBR_DELETE_OPERATION {
		//Update SW cache
		delete(NextHopMgr.ipv6NextHopDB, nbrInfo.Address)
	} else {
		entry, exists := NextHopMgr.ipv6NextHopDB[nbrInfo.Address]
		if !exists {
			NextHopMgr.ipv6NextHopDB[nbrInfo.Address] = &IPv6NextHopInfo{
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

func (mgr *IPv6NextHopInfo) NextHopId(info **pluginCommon.PluginIPNeighborInfo) {
	nbrInfo := *info
	nbrInfo.NextHopId = NextHopMgr.ipv6NextHopDB[nbrInfo.Address].nextHopId
}

func (mgr *IPv6NextHopInfo) CheckRefCount(nbrInfo *pluginCommon.PluginIPNeighborInfo) bool {
	info := NextHopMgr.ipv6NextHopDB[nbrInfo.Address]
	info.refCount -= 1
	NextHopMgr.ipv6NextHopDB[nbrInfo.Address] = info
	if info.refCount == 0 {
		return true
	}
	return false
}

func (mgr *IPv6NextHopInfo) GetRefCount(nbrInfo *pluginCommon.PluginIPNeighborInfo) int {
	info, exists := NextHopMgr.ipv6NextHopDB[nbrInfo.Address]
	if exists {
		return info.refCount
	}
	return 0
}
