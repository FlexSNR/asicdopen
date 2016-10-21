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
	"encoding/json"
	"errors"
	"fmt"
	"models/objects"
	"net"
	"strconv"
	"strings"
	_ "sync"
	"utils/commonDefs"
	"utils/dbutils"
	"utils/lockStack"
	"utils/logging"
)

type IPIntf interface {
	// Initialize DB and platform
	Init(*dbutils.DBUtil)
	// De-Initialize DB and platform
	DeInit(PluginIntf)
	// Verify the IP Address configuration
	VerifyIPConfig(*pluginCommon.PluginIPInfo, net.IP, *net.IPNet) error
	// Get Interface Reference
	GetIntfRef() string
	// Set IfIndex
	SetIfIndex(int32)
	// Get IP Type
	IPType() int
	// Update DB
	UpdateIpDb(string)
	// Send IP Intf Notification
	SendIpNotification(bool, string)
	// Convert IP Addr String to uint32 slice and also get netmask length from the string
	ConvertIPStringToUint32(net.IP, *pluginCommon.PluginIPInfo)
	// Process L2 State Change Notification
	ProcessL2IntfStateChange(int32, int)
	// Retrieve admin state curretly configured
	GetAdminState() string
	// Set admin state curretly configured
	SetAdminState(string)
	// Set operational state
	SetOperState(int)
	// Retrieve IP obj from DB
	GetObjFromInternalDb() (*pluginCommon.PluginIPInfo, error)
}

type SubIntfObj struct {
	IfIndex       int32
	ParentIfIndex int32
	IpAddr        string
	MacAddr       string
	IfName        string
	Enable        bool
}

type linkLocalIpInfo struct {
	IntfRef string
	Using   bool
}

type L3IntfManager struct {
	dbMutex       lockStack.MyLock //sync.RWMutex //lockStack.MyLock make sure debugger is set during Init
	intfMgr       *IfManager
	portMgr       *PortManager
	lagMgr        *LagManager
	vlanMgr       *VlanManager
	neighborMgr   *NeighborManager
	ipv4IntfDB    []*IPv4IntfObj
	ipv6IntfDB    []*IPv6IntfObj
	logicalIntfDB []*asicdServices.LogicalIntfState
	// @jgheewala FIXME do we want to use map for better optimization??
	SubIPv4IntfDB   []SubIntfObj
	plugins         []PluginIntf
	IfIdMap         map[int]int
	logger          *logging.Writer
	notifyChannel   chan<- []byte
	l3IntfRefCount  map[int]int // main a map of ip address's configured on a vlan
	linkLocalMap    map[string]objects.LinkScopeIpState
	linkLocalInUse  []string
	linklocalDbClnt *dbutils.DBUtil
	initComplete    bool
}

// L3intf db
var L3IntfMgr L3IntfManager

func (iMgr *L3IntfManager) InitIPIntf(dbHdl *dbutils.DBUtil) {
	iMgr.logger.Info("InitIpIntf")
	var ip4Obj IPIntf
	var ip6Obj IPIntf
	ip4Obj = &IPv4IntfObj{}
	ip6Obj = &IPv6IntfObj{}
	ip4Obj.Init(dbHdl)
	ip6Obj.Init(dbHdl)

	var dbObjSubIPv4Intf objects.SubIPv4Intf
	objListSubIPv4Intf, err := dbHdl.GetAllObjFromDb(dbObjSubIPv4Intf)
	if err == nil {
		for idx := 0; idx < len(objListSubIPv4Intf); idx++ {
			obj := asicdServices.NewSubIPv4Intf()
			dbObj := objListSubIPv4Intf[idx].(objects.SubIPv4Intf)
			objects.ConvertasicdSubIPv4IntfObjToThrift(&dbObj, obj)
			rv, _ := iMgr.CreateSubIPv4Intf(obj)
			if rv == false {
				iMgr.logger.Err("sub ipv4 interface create failed during init")
			}
		}
	} else {
		iMgr.logger.Err("DB Query failed during SubIPv4IntfConfig query: L3Intf init, Error:", err)
		iMgr.logger.Err("object list is", objListSubIPv4Intf)
	}
	iMgr.logger.Info("InitIpIntf done")
}

func (iMgr *L3IntfManager) InitLogicalIntf(dbHdl *dbutils.DBUtil) {
	var dbObjLogicalIntf objects.LogicalIntf
	objListLogicalIntf, err := dbHdl.GetAllObjFromDb(dbObjLogicalIntf)
	if err == nil {
		for idx := 0; idx < len(objListLogicalIntf); idx++ {
			obj := asicdServices.NewLogicalIntf()
			dbObj := objListLogicalIntf[idx].(objects.LogicalIntf)
			objects.ConvertasicdLogicalIntfObjToThrift(&dbObj, obj)
			rv, _ := iMgr.CreateLogicalIntfConfig(obj)
			if rv == false {
				iMgr.logger.Err("Logical Intf create failed during L3Intf init")
			}
		}
	} else {
		iMgr.logger.Err("DB Query failed during LogicalIntfConfig query: L3Intf init")
	}
}

func (iMgr *L3IntfManager) initLinkLocalDB(dbHdl *dbutils.DBUtil) {
	iMgr.linklocalDbClnt = dbHdl
}

func (iMgr *L3IntfManager) autoGenSystemIpv6linklocalAddr() {
	// @TODO: make sure the api takes into consideration WD-65
	maxPorts := iMgr.portMgr.GetMaxSysPorts()
	for i := 0; i < maxPorts; i++ {
		// if we decide to have different mac then change where we get mac address from
		macAddr := pluginCommon.SwitchMacAddr.String()
		mac, err := net.ParseMAC(macAddr)
		if err != nil {
			continue
		}
		firstOctet := mac[0]
		mac[0] = mac[0] &^ 2
		if mac[0] == firstOctet {
			mac[0] = mac[0] &^ 6
		}
		mac[5] = mac[5] + byte(i) // add portNum to last bit

		newLocalIp := mac.String()
		ll := strings.Split(newLocalIp, ":")
		newLocalIp = fmt.Sprintf("%s%s:%s%s:%s%s:%s%s", ll[0], ll[1], ll[2], "ff", "fe", ll[3], ll[4], ll[5])
		newLocalIp = "fe80::" + newLocalIp + "/64"
		var llObj objects.LinkScopeIpState
		llObj.LinkScopeIp = newLocalIp
		llObj.IntfRef = ""
		llObj.Used = false
		iMgr.linkLocalMap[newLocalIp] = llObj // not used
		iMgr.logger.Info("Generated IPV6 Link Local: index: ", i, "--->", newLocalIp)
	}
	iMgr.logger.Info("Auto generated IPV6 Link Local Pool: len is ", len(iMgr.linkLocalMap), "maxPorts:", maxPorts)
}

func (iMgr *L3IntfManager) Init(rsrcMgrs *ResourceManagers, dbHdl *dbutils.DBUtil, pluginList []PluginIntf,
	logger *logging.Writer, notifyChannel chan<- []byte) {
	iMgr.plugins = pluginList
	iMgr.logger = logger
	iMgr.notifyChannel = notifyChannel
	iMgr.intfMgr = rsrcMgrs.IfManager
	iMgr.portMgr = rsrcMgrs.PortManager
	iMgr.lagMgr = rsrcMgrs.LagManager
	iMgr.vlanMgr = rsrcMgrs.VlanManager
	iMgr.neighborMgr = rsrcMgrs.NeighborManager
	iMgr.dbMutex.Logger = logger
	iMgr.l3IntfRefCount = make(map[int]int, 100)
	iMgr.linkLocalMap = make(map[string]objects.LinkScopeIpState, iMgr.portMgr.GetMaxSysPorts())
	// set init complete and then only read db so that we do not miss any link state notifications
	iMgr.initComplete = true
	// always init link local db before system auto generated ipv6 address
	iMgr.initLinkLocalDB(dbHdl)
	iMgr.autoGenSystemIpv6linklocalAddr()
	if dbHdl != nil {
		iMgr.InitLogicalIntf(dbHdl)
		iMgr.InitIPIntf(dbHdl)
	}
	iMgr.logger.Info("L3 Manager Init is done")
}

func (iMgr *L3IntfManager) DeInitIPIntf(plugin PluginIntf) {
	var ip4Obj IPIntf
	var ip6Obj IPIntf
	ip4Obj = &IPv4IntfObj{}
	ip6Obj = &IPv6IntfObj{}
	
        ip4Obj.DeInit(plugin)
	ip6Obj.DeInit(plugin)

	for _, info := range iMgr.SubIPv4IntfDB {
		rv := DeleteSubIPv4IntfInt(info.IfName)
		if rv < 0 {
			iMgr.logger.Err("Failed to delete sub interface " +
				info.IfName)
		}
	}
}

func (iMgr *L3IntfManager) DeInitLogicalIntf(plugin PluginIntf) {
	for _, info := range iMgr.logicalIntfDB {
		if info != nil {
			rv := plugin.DeleteLogicalIntfConfig(info.Name, commonDefs.IfTypeLoopback)
			if rv < 0 {
				iMgr.logger.Err("Failed to cleanup logical interfaces")
			}
		}
	}
}

func (iMgr *L3IntfManager) Deinit() {
	/* Delete all loopback interfaces on linux plugin */
	for _, plugin := range iMgr.plugins {
		if isPluginAsicDriver(plugin) {
			continue
		}
		iMgr.dbMutex.RLock()
		iMgr.DeInitIPIntf(plugin)
		iMgr.DeInitLogicalIntf(plugin)
		iMgr.dbMutex.RUnlock()
	}
}

func (iMgr *L3IntfManager) GetNumV4Intfs() int {
	iMgr.dbMutex.RLock()
	defer iMgr.dbMutex.RUnlock()
	return len(iMgr.ipv4IntfDB)
}

func (iMgr *L3IntfManager) GetNumV6Intfs() int {
	iMgr.dbMutex.RLock()
	defer iMgr.dbMutex.RUnlock()
	return len(iMgr.ipv6IntfDB)
}

func SendL3IntfStateNotification(ipAddr string, ipType int, ifIndex int32, state int) {
	var notification pluginCommon.AsicdNotification
	var msgBuf []byte
	var err error
	switch ipType {
	case pluginCommon.IP_TYPE_IPV4:
		msg := pluginCommon.IPv4L3IntfStateNotifyMsg{
			IpAddr:  ipAddr,
			IfIndex: ifIndex,
			IfState: uint8(state),
		}
		notification.MsgType = uint8(pluginCommon.NOTIFY_IPV4_L3INTF_STATE_CHANGE)
		msgBuf, err = json.Marshal(msg)
	case pluginCommon.IP_TYPE_IPV6:
		msg := pluginCommon.IPv6L3IntfStateNotifyMsg{
			IpAddr:  ipAddr,
			IfIndex: ifIndex,
			IfState: uint8(state),
		}
		notification.MsgType = uint8(pluginCommon.NOTIFY_IPV6_L3INTF_STATE_CHANGE)
		msgBuf, err = json.Marshal(msg)
	default:
		L3IntfMgr.logger.Err("Skipping Sending Notification: Invalid ipType.")
		return
	}
	if err != nil {
		L3IntfMgr.logger.Err("Failed to marshal L3Intf state change message")
		return
	}
	notification.Msg = msgBuf
	notificationBuf, err := json.Marshal(notification)
	if err != nil {
		L3IntfMgr.logger.Err("Failed to marshal IPv4Intf state change message")
	}
	L3IntfMgr.notifyChannel <- notificationBuf
	L3IntfMgr.logger.Info("Sent L3 state change notification - ", ipAddr, ifIndex, state)
}

//Send out notification regarding ipv4 intf creation - Ribd listens to this
func SendIPCreateDelNotification(msg interface{}, notify_type int) {
	msgBuf, err := json.Marshal(msg)
	if err != nil {
		L3IntfMgr.logger.Err("Failed to marshal IPv4Intf create message")
	}
	notification := pluginCommon.AsicdNotification{
		uint8(notify_type),
		msgBuf,
	}
	notificationBuf, err := json.Marshal(notification)
	if err != nil {
		L3IntfMgr.logger.Err("Failed to marshal IPV4Intf create message")
	}

	L3IntfMgr.logger.Debug("Sent Notification type:", notify_type)
	L3IntfMgr.notifyChannel <- notificationBuf
}

func DetermineL2IntfOperState(ifIndex int32) int {
	l2Ref := pluginCommon.GetIdFromIfIndex(ifIndex)
	l2RefType := pluginCommon.GetTypeFromIfIndex(ifIndex)
	//Send out state notification
	var activeIntfCount, state int
	switch l2RefType {
	case commonDefs.IfTypePort:
		activeIntfCount = L3IntfMgr.portMgr.GetActiveIntfCountFromIfIndexList([]int32{ifIndex})
	case commonDefs.IfTypeLag:
		activeIntfCount = L3IntfMgr.lagMgr.LagGetActiveIntfCount(ifIndex)
	case commonDefs.IfTypeVlan:
		activeIntfCount = L3IntfMgr.vlanMgr.VlanGetActiveIntfCount(int(l2Ref))
	case commonDefs.IfTypeLoopback:
		activeIntfCount = 1
	}
	if activeIntfCount > 0 {
		state = pluginCommon.INTF_STATE_UP
	} else {
		state = pluginCommon.INTF_STATE_DOWN
	}
	return state
}

func (iMgr *L3IntfManager) L3IntfProcessL2IntfStateChange(ifIndex int32, state int) {
	//Dont handle state change notifications until init is complete
	if iMgr.initComplete {
		var ip4Obj IPIntf
		var ip6Obj IPIntf
		ip4Obj = &IPv4IntfObj{}
		ip6Obj = &IPv6IntfObj{}
		ip4Obj.ProcessL2IntfStateChange(ifIndex, state)
		ip6Obj.ProcessL2IntfStateChange(ifIndex, state)
	}
}

/*
 *  API will store VlanId and IfName in ipInfo (PluginIPInfo)
 *      It will also return l2RefType so that it can be used during call to plugins
 */
func GetVlanIdAndIfNameFromL2Ref(ipInfo *pluginCommon.PluginIPInfo, create bool) (int, error) {
	ifIndex := ipInfo.IfIndex
	l2Ref := pluginCommon.GetIdFromIfIndex(ifIndex)
	l2RefType := pluginCommon.GetTypeFromIfIndex(ifIndex)
	L3IntfMgr.logger.Info("create/delete:", create, "for ipintf for ifIndex", strconv.Itoa(int(ifIndex)),
		"l2RefType type= ", strconv.Itoa(l2RefType), " l2ref=", strconv.Itoa(l2Ref), "ipInfo:", *ipInfo)
	switch l2RefType {
	case commonDefs.IfTypePort, commonDefs.IfTypeLag:
		ipInfo.VlanId = L3IntfMgr.vlanMgr.GetSysVlanContainingIf(ifIndex)
		switch create {
		case true:
			if l2RefType == commonDefs.IfTypePort {
				if L3IntfMgr.portMgr.GetPortConfigMode(ifIndex) == pluginCommon.PORT_MODE_L2 ||
					L3IntfMgr.portMgr.GetPortConfigMode(ifIndex) == pluginCommon.PORT_MODE_INTERNAL {
					return l2RefType, errors.New("Please remove all L2 configuration on " + ipInfo.IfName + " before creating an IP interface")
				}
			}
			// during ip create we first check for vlanID to see if it is valid or not..
			// if it is not valid then we will create vlan id....
			// if that fails then only we will return error
			if ipInfo.VlanId < 0 {
				ipInfo.VlanId, _, _ = L3IntfMgr.vlanMgr.CreateVlan(&asicdServices.Vlan{
					AdminState:    INTF_STATE_UP,
					VlanId:        pluginCommon.SYS_RSVD_VLAN,
					IntfList:      []string{""},
					UntagIntfList: []string{strconv.Itoa(int(ifIndex))},
				})
				if ipInfo.VlanId < 0 {
					return l2RefType, errors.New("System reserved vlan creation " +
						"failed during IPIntf create" + strconv.Itoa(int(ipInfo.VlanId)))
				}
			}
		}
	case commonDefs.IfTypeVlan:
		//Implicit vlan creation not required here
		ipInfo.VlanId = l2Ref
	case commonDefs.IfTypeLoopback:
		for intfIdx := 0; intfIdx < len(L3IntfMgr.logicalIntfDB); intfIdx++ {
			tempElem := *(L3IntfMgr.logicalIntfDB[intfIdx])
			if tempElem.IfIndex == ifIndex {
				ipInfo.IfName = tempElem.Name
				break
			}
		}
		break
	default:
		return l2RefType, errors.New("Unknown ifIndex passed during IPIntf create")
	}
	return l2RefType, nil
}

func CreateIPIntfInternal(ipInfo *pluginCommon.PluginIPInfo) (bool, error) {
	l2RefType, err := GetVlanIdAndIfNameFromL2Ref(ipInfo, true /*create Vlan*/)
	if err != nil {
		return false, err
	}
	ipInfo.RefCount = L3IntfMgr.l3IntfRefCount[ipInfo.VlanId]
	ipInfo.InstallRoute = 1

	//Have plugins create interface only if admin state is enabled
	if ipInfo.AdminState == pluginCommon.INTF_STATE_UP {
		for _, plugin := range L3IntfMgr.plugins {
			if l2RefType == commonDefs.IfTypeLoopback && isPluginAsicDriver(plugin) {
				L3IntfMgr.logger.Info("dont call asicd to configure ip address on a loopback intf")
				continue
			}
			rv := plugin.CreateIPIntf(ipInfo)
			if rv < 0 {
				return false, errors.New(":Plugin Failed to create IPIntf")
			}
		}
	}
	if l2RefType == commonDefs.IfTypePort {
		L3IntfMgr.portMgr.SetPortConfigMode(pluginCommon.PORT_MODE_L3, []int32{ipInfo.IfIndex})
	}
	return true, nil
}

func (iMgr *L3IntfManager) handleIPCreate(ipObj IPIntf, adminState string, ip net.IP, ipNet *net.IPNet) (*pluginCommon.PluginIPInfo, error) {
	ipInfo := &pluginCommon.PluginIPInfo{}

	//Convert intf ref to ifindex
	ifIndexList, err := iMgr.intfMgr.ConvertIntfStrListToIfIndexList([]string{ipObj.GetIntfRef()})
	if err != nil {
		return ipInfo, errors.New(fmt.Sprintln("Invalid interface reference provided in ip intf config object:", ipObj.GetIntfRef()))
	}
	ipInfo.IfIndex = ifIndexList[0]
	ipInfo.IfName, _ = L3IntfMgr.intfMgr.GetIfNameForIfIndex(ipInfo.IfIndex)
	ipInfo.IpType = ipObj.IPType()
	if adminState == INTF_STATE_UP {
		ipInfo.AdminState = pluginCommon.INTF_STATE_UP
	} else {
		ipInfo.AdminState = pluginCommon.INTF_STATE_DOWN
	}

	// Verify config
	iMgr.dbMutex.RLock()
	err = ipObj.VerifyIPConfig(ipInfo, ip, ipNet)
	iMgr.dbMutex.RUnlock()
	if err != nil {
		return ipInfo, err
	}
	ipInfo.Address = ip.String() + "/" + strconv.Itoa(ipInfo.MaskLen)

	// Create interface and setup neighbor
	_, err = CreateIPIntfInternal(ipInfo)
	if err != nil {
		return ipInfo, err
	}

	// Update RefCount for Vlan Interface
	ipInfo.RefCount = ipInfo.RefCount + 1
	iMgr.l3IntfRefCount[ipInfo.VlanId] = ipInfo.RefCount

	return ipInfo, nil
}

func (iMgr *L3IntfManager) CreateIPIntf(ipAddr, intfRef, adminState string) (bool, error) {
	// Pre-check
	if !strings.Contains(ipAddr, "/") {
		return false, errors.New("Create interface failed, because " +
			"netmask is not specified for ip " + ipAddr)
	}
	ip, ipNet, err := net.ParseCIDR(ipAddr)
	if err != nil {
		return false, errors.New("Failed to parse ip addr during IPIntf create")
	}
	var ipObj IPIntf
	if ip.To4() != nil {
		// IPv4 Object
		ipObj = &IPv4IntfObj{
			IpAddr:     ipAddr,
			IntfRef:    intfRef,
			AdminState: adminState,
		}
	} else {
		// IPv6 Object
		ipObj = &IPv6IntfObj{
			IpAddr:     ipAddr,
			IntfRef:    intfRef,
			AdminState: adminState,
		}
	}

	ipInfo, err := iMgr.handleIPCreate(ipObj, adminState, ip, ipNet)
	if err != nil {
		return false, err
	}

	ipObj.SetIfIndex(ipInfo.IfIndex)
	var operState string
	if DetermineL2IntfOperState(ipInfo.IfIndex) == pluginCommon.INTF_STATE_UP {
		operState = INTF_STATE_UP
	} else {
		operState = INTF_STATE_DOWN
	}

	// Update DB with IP information
	iMgr.dbMutex.Lock()
	ipObj.UpdateIpDb(operState)
	iMgr.dbMutex.Unlock()

	// Send IP Create Notification & L3 State Notification
	ipObj.SendIpNotification(true /*create*/, operState)

	return true, nil
}

func (iMgr *L3IntfManager) UpdateIPIntf(oldIpAddr, oldIntfRef, ipAddr, intfRef, adminState string) (ok bool, err error) {
	if ipAddr != oldIpAddr {
		//Treat this as a delete and add
		ok, err = iMgr.DeleteIPIntf(oldIpAddr, intfRef)
		if err != nil {
			return ok, err
		}
		ok, err = iMgr.CreateIPIntf(ipAddr, intfRef, adminState)
		if err != nil {
			return ok, err
		}
	} else {
		ip, _, err := net.ParseCIDR(ipAddr)
		if err != nil {
			return false, errors.New("Failed to parse ip addr during IPIntf update")
		}
		ifIndex, _ := iMgr.intfMgr.GetIfIndexForIfName(intfRef)
		var ipObj IPIntf
		if ip.To4() != nil {
			// IPv4 Object
			ipObj = &IPv4IntfObj{
				IpAddr:     ipAddr,
				IntfRef:    intfRef,
				AdminState: adminState,
				IfIndex:    ifIndex,
			}
		} else {
			// IPv6 Object
			ipObj = &IPv6IntfObj{
				IpAddr:     ipAddr,
				IntfRef:    intfRef,
				AdminState: adminState,
				IfIndex:    ifIndex,
			}
		}
		// Update IP intf
		iMgr.dbMutex.Lock()
		obj, err := ipObj.GetObjFromInternalDb()
		iMgr.dbMutex.Unlock()
		if err == nil {
			obj.IfName = intfRef
			if adminState == INTF_STATE_UP {
				obj.AdminState = pluginCommon.INTF_STATE_UP
			} else {
				obj.AdminState = pluginCommon.INTF_STATE_DOWN
			}
			for _, plugin := range iMgr.plugins {
				rv := plugin.UpdateIPIntf(obj)
				if rv < 0 {
					return false, errors.New("Plugin Failed to update IPIntf")
				}
			}
			var operState int
			iMgr.dbMutex.Lock()
			if (adminState == INTF_STATE_UP) && (DetermineL2IntfOperState(obj.IfIndex) == pluginCommon.INTF_STATE_UP) {
				operState = pluginCommon.INTF_STATE_UP
			} else {
				operState = pluginCommon.INTF_STATE_DOWN
			}
			//Update admin state value in DB
			ipObj.SetAdminState(adminState)
			ipObj.SetOperState(operState)
			// Send IP Create Notification & L3 State Notification
			SendL3IntfStateNotification(ipAddr, obj.IpType, obj.IfIndex, operState)
			iMgr.dbMutex.Unlock()
		} else {
			iMgr.logger.Err("Unable to locate record in internal DB during IP Intf update - ", err)
		}
	}
	return ok, err
}

func (iMgr *L3IntfManager) DeleteIPIntf(ipAddr, intfRef string) (bool, error) {
	ipInfo := &pluginCommon.PluginIPInfo{}
	//Convert intf ref to ifindex
	ifIndexList, err := iMgr.intfMgr.ConvertIntfStrListToIfIndexList([]string{intfRef})
	if err != nil {
		return false, errors.New("Invalid interface reference provided in ip intf config object")
	}
	ipInfo.IfIndex = ifIndexList[0]
	l2RefType, err := GetVlanIdAndIfNameFromL2Ref(ipInfo, false /*delete vlan*/)

	ip, ipNet, err := net.ParseCIDR(ipAddr)
	if err != nil {
		return false, errors.New("Failed to parse ip addr during IPIntf delete")
	}

	var ipObj IPIntf
	if ip.To4() != nil {
		// IPv4 Object
		ipObj = &IPv4IntfObj{
			IpAddr:  ipAddr,
			IntfRef: intfRef,
		}
		ip = ip.To4()
	} else {
		// IPv6 Object
		ipObj = &IPv6IntfObj{
			IpAddr:  ipAddr,
			IntfRef: intfRef,
		}
		ip = ip.To16()
	}
	ipInfo.IpType = ipObj.IPType()
	ipInfo.MaskLen, _ = ipNet.Mask.Size()
	// Get parsedIp Information
	ipObj.ConvertIPStringToUint32(ip, ipInfo)
	ipInfo.Address = ip.String() + "/" + strconv.Itoa(ipInfo.MaskLen)
	ipInfo.RefCount = iMgr.l3IntfRefCount[ipInfo.VlanId]
	ipInfo.IfName = intfRef
	ipObj.SetIfIndex(ipInfo.IfIndex)

	//Only perform delete if Admin state is up
	if ipObj.GetAdminState() == INTF_STATE_UP {
		// Call all plugins and delete IP Interface
		for _, plugin := range iMgr.plugins {
			if l2RefType == commonDefs.IfTypeLoopback && isPluginAsicDriver(plugin) {
				L3IntfMgr.logger.Info("dont call asicd to delete ipv4 on a loopback intf")
				continue
			}
			rv := plugin.DeleteIPIntf(ipInfo)
			if rv < 0 {
				return false, errors.New("Plugin failed to delete IP interface")
			}
		}
	}

	//Update Sw State
	//for ipv6 we will always verify ipAddress and ifIndex
	iMgr.dbMutex.Lock()
	ipObj.UpdateIpDb("")
	iMgr.dbMutex.Unlock()

	// Update RefCount
	ipInfo.RefCount = ipInfo.RefCount - 1
	iMgr.l3IntfRefCount[ipInfo.VlanId] = ipInfo.RefCount

	//If delete successful & there are no more ip address for the same vlane remove internal vlan if any
	//and update sw state also and send out notifications
	if ipInfo.RefCount == 0 {
		switch l2RefType {
		case commonDefs.IfTypePort, commonDefs.IfTypeLag:
			vid, _, _ := iMgr.vlanMgr.DeleteVlan(&asicdServices.Vlan{
				VlanId:        pluginCommon.SYS_RSVD_VLAN,
				IntfList:      []string{""},
				UntagIntfList: []string{strconv.Itoa(int(ipInfo.IfIndex))},
			})
			if vid < 0 {
				return false, errors.New(fmt.Sprintln("System reserved vlan deletion failed during",
					"IPIntf delete for ifIndex:", ipInfo.IfIndex, "ip address:", ipInfo.Address))
			}
			if l2RefType == commonDefs.IfTypePort {
				iMgr.portMgr.SetPortConfigMode(pluginCommon.PORT_MODE_UNCONFIGURED, []int32{ipInfo.IfIndex})
			}
		}
	}
	//Send out notification regarding ip intf deletion - Ribd listens to this
	ipObj.SendIpNotification(false /*delete*/, INTF_STATE_DOWN)
	return true, nil
}

func (iMgr *L3IntfManager) deleteLSEntry(ipAddr string) {
	for idx, _ := range iMgr.linkLocalInUse {
		if iMgr.linkLocalInUse[idx] == ipAddr {
			iMgr.linkLocalInUse = append(iMgr.linkLocalInUse[:idx], iMgr.linkLocalInUse[idx+1:]...)
			break
		}
	}
}

func (iMgr *L3IntfManager) insertLSEntry(ipAddr string) {
	iMgr.linkLocalInUse = append(iMgr.linkLocalInUse, ipAddr)
}

func (iMgr *L3IntfManager) UpdateLinkLocalDB(operation bool, obj *objects.LinkScopeIpState) {
	switch operation {
	case true: // store in db
		err := iMgr.linklocalDbClnt.StoreObjectInDb(*obj)
		if err != nil {
			iMgr.logger.Err("Failed to add link scope state ip", obj.LinkScopeIp, " object to DB with error", err)
			return
		}
		iMgr.logger.Info("IPV6: LS stored ipaddr:", obj.LinkScopeIp, "for intfRef:", obj.IntfRef, "in db")
		iMgr.insertLSEntry(obj.LinkScopeIp)
	case false: // remove in db
		err := iMgr.linklocalDbClnt.DeleteObjectFromDb(*obj)
		if err != nil {
			iMgr.logger.Err("Failed to delete link scope state ip", obj.LinkScopeIp, " object from DB with error", err)
			return
		}
		iMgr.logger.Info("IPV6: LS removed ipaddr:", obj.LinkScopeIp, "for intfRef:", obj.IntfRef, "from db")
		iMgr.deleteLSEntry(obj.LinkScopeIp)
	}
}

func (iMgr *L3IntfManager) updatelinkLocalEntry(linkScopeIp, intfRef string, used bool) {
	llObj, exists := iMgr.linkLocalMap[linkScopeIp]
	if !exists {
		iMgr.logger.Err("AutoGenerated IPV6 LS IP not found in Link Local Cache Info", linkScopeIp,
			"becuase of that We are not gonna update the cache... fix your bug")
		return
	}
	llObj.IntfRef = intfRef
	llObj.Used = used
	ifIndexList, err := iMgr.intfMgr.ConvertIntfStrListToIfIndexList([]string{intfRef})
	if err == nil {
		llObj.IfIndex = ifIndexList[0]
	} else {
		iMgr.logger.Err("Sorry clients you will be messed up because i cannot find ifIndex for intfref", intfRef)
	}
	iMgr.linkLocalMap[linkScopeIp] = llObj
	iMgr.UpdateLinkLocalDB(used, &llObj)
}

func (iMgr *L3IntfManager) CreateIPv6LinkLocalIP(intfRef string) (bool, error) {
	newLocalIp := ""
	for ipAddr, llObj := range iMgr.linkLocalMap {
		if !llObj.Used {
			newLocalIp = ipAddr
			break
		}
	}
	if newLocalIp == "" {
		return false, errors.New("No more link-local IPv6 Address can be assigned on the system")
	}

	created, err := iMgr.CreateIPIntf(newLocalIp, intfRef, INTF_STATE_UP)
	if err != nil {
		return created, err
	}

	iMgr.dbMutex.Lock()
	iMgr.updatelinkLocalEntry(newLocalIp, intfRef, true)
	iMgr.dbMutex.Unlock()
	return created, err
}

func (iMgr *L3IntfManager) UpdateIPv6LinkLocalIP(intfRef, adminState string) (bool, error) {
	localIp := ""
	for ipAddr, llObj := range iMgr.linkLocalMap {
		if llObj.IntfRef == intfRef {
			localIp = ipAddr
			break
		}
	}
	if localIp == "" {
		return false, errors.New("Unable to find link-local IPv6 address during update")
	}

	updated, err := iMgr.UpdateIPIntf(localIp, intfRef, localIp, intfRef, adminState)
	if err != nil {
		return updated, err
	}
	return updated, err
}

func (iMgr *L3IntfManager) DeleteIPv6LinkLocalIP(intfRef string) (bool, error) {
	delLocalIp := ""
	for ipAddr, llObj := range iMgr.linkLocalMap {
		if llObj.IntfRef == intfRef {
			delLocalIp = ipAddr
			break
		}
	}
	if delLocalIp == "" {
		return false, errors.New(fmt.Sprintln("Nothing to delete for link-local IPv6 Address can be assigned on the system",
			"for interface", intfRef))
	}

	deleted, err := iMgr.DeleteIPIntf(delLocalIp, intfRef)
	if err != nil {
		return deleted, err
	}

	iMgr.dbMutex.Lock()
	iMgr.updatelinkLocalEntry(delLocalIp, intfRef, false)
	iMgr.dbMutex.Unlock()
	return deleted, err
}

func (iMgr *L3IntfManager) GetIPv4IntfState(intfRef string) (*asicdServices.IPv4IntfState, error) {
	if iMgr.ipv4IntfDB == nil {
		L3IntfMgr.logger.Err(fmt.Sprintln("ipv4IntfDB nil"))
		return nil, errors.New("Nil DB")
	}
	//Convert intf ref to ifindex
	ifIndexList, err := iMgr.intfMgr.ConvertIntfStrListToIfIndexList([]string{intfRef})
	if err != nil {
		return nil, errors.New("Invalid interface reference provided during get ipv4 intf state")
	}
	ifIndex := ifIndexList[0]
	iMgr.dbMutex.RLock()
	for i := 0; i < len(iMgr.ipv4IntfDB); i++ {
		ipv4Intf := iMgr.ipv4IntfDB[i]
		if ipv4Intf.IfIndex == ifIndex {
			iMgr.dbMutex.RUnlock()
			return &asicdServices.IPv4IntfState{
				IntfRef:           ipv4Intf.IntfRef,
				IpAddr:            ipv4Intf.IpAddr,
				IfIndex:           ipv4Intf.IfIndex,
				OperState:         ipv4Intf.OperState,
				NumUpEvents:       ipv4Intf.NumUpEvents,
				LastUpEventTime:   ipv4Intf.LastUpEventTime,
				NumDownEvents:     ipv4Intf.NumDownEvents,
				LastDownEventTime: ipv4Intf.LastDownEventTime,
				L2IntfType:        commonDefs.GetIfTypeName(pluginCommon.GetTypeFromIfIndex(ipv4Intf.IfIndex)),
				L2IntfId:          int32(pluginCommon.GetIdFromIfIndex(ipv4Intf.IfIndex)),
			}, nil
		}
	}
	iMgr.dbMutex.RUnlock()
	return nil, errors.New(fmt.Sprintln("Unable to locate ipv4 interface corresponding to intf - ", intfRef))
}

func (iMgr *L3IntfManager) GetBulkIPv4IntfState(start, count int) (end, listLen int, more bool, ipv4IntfList []*asicdServices.IPv4IntfState) {
	if start < 0 {
		iMgr.logger.Err("Invalid start index argument in get bulk ipv4intf")
		return -1, 0, false, nil
	}
	if count < 0 {
		iMgr.logger.Err("Invalid count in get bulk ipv4intf")
		return -1, 0, false, nil
	}
	if (start + count) < len(iMgr.ipv4IntfDB) {
		more = true
		end = start + count
	} else {
		more = false
		end = len(iMgr.ipv4IntfDB)
	}
	iMgr.dbMutex.RLock()
	for idx := start; idx < len(iMgr.ipv4IntfDB); idx++ {
		ipv4IntfList = append(ipv4IntfList, &asicdServices.IPv4IntfState{
			IntfRef:           iMgr.ipv4IntfDB[idx].IntfRef,
			IpAddr:            iMgr.ipv4IntfDB[idx].IpAddr,
			IfIndex:           iMgr.ipv4IntfDB[idx].IfIndex,
			OperState:         iMgr.ipv4IntfDB[idx].OperState,
			NumUpEvents:       iMgr.ipv4IntfDB[idx].NumUpEvents,
			LastUpEventTime:   iMgr.ipv4IntfDB[idx].LastUpEventTime,
			NumDownEvents:     iMgr.ipv4IntfDB[idx].NumDownEvents,
			LastDownEventTime: iMgr.ipv4IntfDB[idx].LastDownEventTime,
			L2IntfType:        commonDefs.GetIfTypeName(pluginCommon.GetTypeFromIfIndex(iMgr.ipv4IntfDB[idx].IfIndex)),
			L2IntfId:          int32(pluginCommon.GetIdFromIfIndex(iMgr.ipv4IntfDB[idx].IfIndex)),
		})
	}
	iMgr.dbMutex.RUnlock()
	return end, len(ipv4IntfList), more, ipv4IntfList
}

func (iMgr *L3IntfManager) GetIPv6IntfState(intfRef string) (*asicdServices.IPv6IntfState, error) {
	if iMgr.ipv6IntfDB == nil {
		L3IntfMgr.logger.Err(fmt.Sprintln("ipv6IntfDB nil"))
		return nil, errors.New("Nil DB")
	}
	//Convert intf ref to ifindex
	ifIndexList, err := iMgr.intfMgr.ConvertIntfStrListToIfIndexList([]string{intfRef})
	if err != nil {
		return nil, errors.New("Invalid interface reference provided during get ipv4 intf state")
	}
	ifIndex := ifIndexList[0]
	iMgr.dbMutex.RLock()
	for i := 0; i < len(iMgr.ipv6IntfDB); i++ {
		ipv6Intf := iMgr.ipv6IntfDB[i]
		if ipv6Intf.IfIndex == ifIndex {
			iMgr.dbMutex.RUnlock()
			return &asicdServices.IPv6IntfState{
				IntfRef:           ipv6Intf.IntfRef,
				IpAddr:            ipv6Intf.IpAddr,
				IfIndex:           ipv6Intf.IfIndex,
				OperState:         ipv6Intf.OperState,
				NumUpEvents:       ipv6Intf.NumUpEvents,
				LastUpEventTime:   ipv6Intf.LastUpEventTime,
				NumDownEvents:     ipv6Intf.NumDownEvents,
				LastDownEventTime: ipv6Intf.LastDownEventTime,
				L2IntfType:        commonDefs.GetIfTypeName(pluginCommon.GetTypeFromIfIndex(ipv6Intf.IfIndex)),
				L2IntfId:          int32(pluginCommon.GetIdFromIfIndex(ipv6Intf.IfIndex)),
			}, nil
		}
	}
	iMgr.dbMutex.RUnlock()
	return nil, errors.New(fmt.Sprintln("Unable to locate ipv6 interface corresponding to intf - ", intfRef))
}

func (iMgr *L3IntfManager) GetBulkIPv6IntfState(start, count int) (end, listLen int, more bool, ipv6IntfList []*asicdServices.IPv6IntfState) {
	if start < 0 {
		iMgr.logger.Err("Invalid start index argument in get bulk ipv6intf")
		return -1, 0, false, nil
	}
	if count < 0 {
		iMgr.logger.Err("Invalid count in get bulk ipv4intf")
		return -1, 0, false, nil
	}
	if (start + count) < len(iMgr.ipv6IntfDB) {
		more = true
		end = start + count
	} else {
		more = false
		end = len(iMgr.ipv6IntfDB)
	}
	iMgr.dbMutex.RLock()
	for idx := start; idx < len(iMgr.ipv6IntfDB); idx++ {
		ipv6IntfList = append(ipv6IntfList, &asicdServices.IPv6IntfState{
			IntfRef:           iMgr.ipv6IntfDB[idx].IntfRef,
			IpAddr:            iMgr.ipv6IntfDB[idx].IpAddr,
			IfIndex:           iMgr.ipv6IntfDB[idx].IfIndex,
			OperState:         iMgr.ipv6IntfDB[idx].OperState,
			NumUpEvents:       iMgr.ipv6IntfDB[idx].NumUpEvents,
			LastUpEventTime:   iMgr.ipv6IntfDB[idx].LastUpEventTime,
			NumDownEvents:     iMgr.ipv6IntfDB[idx].NumDownEvents,
			LastDownEventTime: iMgr.ipv6IntfDB[idx].LastDownEventTime,
			L2IntfType:        commonDefs.GetIfTypeName(pluginCommon.GetTypeFromIfIndex(iMgr.ipv6IntfDB[idx].IfIndex)),
			L2IntfId:          int32(pluginCommon.GetIdFromIfIndex(iMgr.ipv6IntfDB[idx].IfIndex)),
		})
	}
	iMgr.dbMutex.RUnlock()
	return end, len(ipv6IntfList), more, ipv6IntfList
}

func (iMgr *L3IntfManager) CreateLogicalIntfConfig(confObj *asicdServices.LogicalIntf) (bool, error) {
	var ifIndex int32
	iMgr.logger.Info(fmt.Sprintln("CreateLogicalIntfConfig for name: ", confObj.Name, " Type: ", confObj.Type))
	ifType := -1
	tolower := strings.ToLower(confObj.Type)
	iMgr.logger.Info("tolower:", tolower)
	switch tolower {
	case "loopback":
		ifType = commonDefs.IfTypeLoopback
		ifIndex = iMgr.intfMgr.AllocateIfIndex(confObj.Name, 0, commonDefs.IfTypeLoopback)
		break
	default:
		iMgr.logger.Info("Unknown logical intf type: ", confObj.Type)
		return false, errors.New(fmt.Sprintln("Unknown logical intf type: ", confObj.Type))
		break
	}

	for _, plugin := range iMgr.plugins {
		rv := plugin.CreateLogicalIntfConfig(confObj.Name, ifType)
		if rv < 0 {
			return false, errors.New("Plugin failed to create logical interface")
		}
	}
	logicalIntfState := &asicdServices.LogicalIntfState{
		Name:    confObj.Name,
		IfIndex: ifIndex,
		SrcMac:  pluginCommon.SwitchMacAddr.String(),
	}
	//Update SW cache
	iMgr.dbMutex.Lock()
	iMgr.logicalIntfDB = append(iMgr.logicalIntfDB, logicalIntfState)
	iMgr.dbMutex.Unlock()

	//Publish notification - RIBd listens to logical intf create notification
	msg := pluginCommon.LogicalIntfNotifyMsg{
		IfIndex:         ifIndex,
		LogicalIntfName: confObj.Name,
	}
	msgBuf, err := json.Marshal(msg)
	if err != nil {
		iMgr.logger.Err("Failed to marshal logicalIntf create message")
	}
	notification := pluginCommon.AsicdNotification{
		MsgType: uint8(pluginCommon.NOTIFY_LOGICAL_INTF_CREATE),
		Msg:     msgBuf,
	}
	notificationBuf, err := json.Marshal(notification)
	if err != nil {
		iMgr.logger.Err("Failed to marshal vlan create message")
	}
	iMgr.notifyChannel <- notificationBuf
	return true, nil
}

func (iMgr *L3IntfManager) DeleteLogicalIntfConfig(confObj *asicdServices.LogicalIntf) (bool, error) {
	var ifIndex int32
	iMgr.logger.Info(fmt.Sprintln("DeleteLogicalIntfConfig for name: ", confObj.Name, " Type: ", confObj.Type))
	ifType := -1
	switch confObj.Type {
	case "Loopback":
		ifType = commonDefs.IfTypeLoopback
		break
	default:
		iMgr.logger.Info("Unknown logical intf type: ", confObj.Type)
		break
	}
	for _, plugin := range iMgr.plugins {
		rv := plugin.DeleteLogicalIntfConfig(confObj.Name, ifType)
		if rv < 0 {
			return false, errors.New("Plugin failed to delete logical interface")
		}
	}
	for i := 0; i < len(iMgr.logicalIntfDB); i++ {
		if iMgr.logicalIntfDB[i].Name == confObj.Name {
			ifIndex = iMgr.logicalIntfDB[i].IfIndex
			iMgr.intfMgr.FreeIfIndex(ifIndex)
			iMgr.logicalIntfDB = append(iMgr.logicalIntfDB[:i], iMgr.logicalIntfDB[i+1:]...)
			break
		}
	}
	//Publish notification - RIBd listens to logical intf delete notification
	msg := pluginCommon.LogicalIntfNotifyMsg{
		IfIndex:         ifIndex,
		LogicalIntfName: confObj.Name,
	}
	msgBuf, err := json.Marshal(msg)
	if err != nil {
		iMgr.logger.Err("Failed to marshal logicalIntf delete message")
	}
	notification := pluginCommon.AsicdNotification{
		MsgType: uint8(pluginCommon.NOTIFY_LOGICAL_INTF_DELETE),
		Msg:     msgBuf,
	}
	notificationBuf, err := json.Marshal(notification)
	if err != nil {
		iMgr.logger.Err("Failed to marshal logicalIntf delete message")
	}
	iMgr.notifyChannel <- notificationBuf
	return true, nil
}

func (iMgr *L3IntfManager) GetLogicalIntfState(name string) (*asicdServices.LogicalIntfState, error) {
	iMgr.dbMutex.RLock()
	for i := 0; i < len(iMgr.logicalIntfDB); i++ {
		intf := iMgr.logicalIntfDB[i]
		if intf.Name == name {
			iMgr.dbMutex.RUnlock()
			return intf, nil
		}
	}
	iMgr.dbMutex.RUnlock()
	return nil, errors.New(fmt.Sprintln("Unable to locate logical interface - ", name))
}

func (iMgr *L3IntfManager) GetBulkLogicalIntfState(start, count int) (end, listLen int, more bool, intfList []*asicdServices.LogicalIntfState) {
	if start < 0 {
		iMgr.logger.Err("Invalid start index argument in get bulk logicalIntfState")
		return -1, 0, false, nil
	}
	if count < 0 {
		iMgr.logger.Err("Invalid count in get bulk logicalIntfState")
		return -1, 0, false, nil
	}
	if (start + count) < len(iMgr.logicalIntfDB) {
		more = true
		end = start + count
		listLen = count
	} else {
		more = false
		end = len(iMgr.logicalIntfDB)
		listLen = len(iMgr.logicalIntfDB) - start
	}
	iMgr.dbMutex.RLock()
	intfList = append(intfList, iMgr.logicalIntfDB[start:end]...)
	iMgr.dbMutex.RUnlock()
	return end, listLen, more, intfList
}

func (iMgr *L3IntfManager) GetRouterIfIndexContainingNeighbor(neighborIP []uint8) (routerIfIndex, ifIndex int32) {
	//iMgr.logger.Info("GetRouterIfIndexContainingNeighbor for neighbor IP:", neighborIP)
	routerIfIndex = pluginCommon.INVALID_IFINDEX
	ifIndex = pluginCommon.INVALID_IFINDEX
	iMgr.dbMutex.RLock()
	defer iMgr.dbMutex.RUnlock()
	for _, v4Intf := range iMgr.ipv4IntfDB {
		var maskedIp, dbMaskedIp net.IP
		dbIP, dbIPNet, err := net.ParseCIDR(v4Intf.IpAddr)
		if err != nil {
			iMgr.logger.Err("Failed to parse db ip addr during IPv4Intf create")
			return routerIfIndex, ifIndex
		}
		dbIP = dbIP.To4()
		maskedIp = (net.IP(neighborIP)).Mask(net.IPMask(dbIPNet.Mask))
		dbMaskedIp = (net.IP(dbIP)).Mask(net.IPMask(dbIPNet.Mask))
		//iMgr.logger.Debug("dbMaskedIP, neighborMaskedIP = ", dbMaskedIp, maskedIp)
		if bytes.Equal(maskedIp, dbMaskedIp) {
			ifType := pluginCommon.GetTypeFromIfIndex(v4Intf.IfIndex)
			switch ifType {
			case commonDefs.IfTypePort, commonDefs.IfTypeLag:
				vlanId := iMgr.vlanMgr.GetSysVlanContainingIf(v4Intf.IfIndex)
				ifIndex = v4Intf.IfIndex
				routerIfIndex = pluginCommon.GetIfIndexFromIdType(vlanId, commonDefs.IfTypeVlan)
			default:
				ifIndex = v4Intf.IfIndex
				routerIfIndex = v4Intf.IfIndex
			}
			break
		}
	}
	return routerIfIndex, ifIndex
}
func (iMgr *L3IntfManager) GetRouterIfIndexContainingv6Neighbor(neighborIP []uint8) (routerIfIndex, ifIndex int32) {
	routerIfIndex = pluginCommon.INVALID_IFINDEX
	ifIndex = pluginCommon.INVALID_IFINDEX
	iMgr.dbMutex.RLock()
	defer iMgr.dbMutex.RUnlock()
	for _, v6Intf := range iMgr.ipv6IntfDB {
		var maskedIp, dbMaskedIp net.IP
		dbIP, dbIPNet, err := net.ParseCIDR(v6Intf.IpAddr)
		if err != nil {
			iMgr.logger.Err("Failed to parse db ip addr during IPv6Intf create")
			return routerIfIndex, ifIndex
		}
		dbIP = dbIP.To16()
		maskedIp = (net.IP(neighborIP)).Mask(net.IPMask(dbIPNet.Mask))
		dbMaskedIp = (net.IP(dbIP)).Mask(net.IPMask(dbIPNet.Mask))
		iMgr.logger.Debug("dbMaskedIP, neighborMaskedIP = ", dbMaskedIp, maskedIp)
		if bytes.Equal(maskedIp, dbMaskedIp) {
			ifType := pluginCommon.GetTypeFromIfIndex(v6Intf.IfIndex)
			switch ifType {
			case commonDefs.IfTypePort, commonDefs.IfTypeLag:
				vlanId := iMgr.vlanMgr.GetSysVlanContainingIf(v6Intf.IfIndex)
				ifIndex = v6Intf.IfIndex
				routerIfIndex = pluginCommon.GetIfIndexFromIdType(vlanId, commonDefs.IfTypeVlan)
			default:
				ifIndex = v6Intf.IfIndex
				routerIfIndex = v6Intf.IfIndex
			}
			break
		}
	}
	return routerIfIndex, ifIndex
}

func VerifySubIPv4Config(parsedIP uint32, maskLen int) error {
	for _, subv4Intf := range L3IntfMgr.SubIPv4IntfDB {
		//Verify overlapping subnet does not already exist
		dbIP, dbIPNet, err := net.ParseCIDR(subv4Intf.IpAddr)
		if err != nil {
			return errors.New("Failed to parse db ip addr during SubIPv4Intf create")
		}
		dbIP = dbIP.To4()
		dbParsedIP := uint32(dbIP[3]) | uint32(dbIP[2])<<8 |
			uint32(dbIP[1])<<16 | uint32(dbIP[0])<<24
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
			return errors.New("Failed to create sub ipv4 intf, " +
				"matching sub ipv4 intf already exists")
		}
	}
	return nil
}

func CreateSubIPv4IntfInt(ifIndex int32, mac string, parsedIP uint32, maskLen int,
	stateUp bool, subIntfIfIndex int32) (string, error) {
	refId := pluginCommon.GetIdFromIfIndex(ifIndex)
	refType := pluginCommon.GetTypeFromIfIndex(ifIndex)
	L3IntfMgr.logger.Info("create sub ipv4intf for ifIndex", ifIndex, "type=", refType, "l2ref =", refId)
	var ifName string
	// IPV4 sub interface is created on physical port and hence the
	// id comes from ifIndex
	vid := refId
	// Create all plugins for programming sub ipv4 interface
	l2Mac, _ := net.ParseMAC(mac)
	obj := pluginCommon.SubIntfPluginObj{
		IfIndex:        ifIndex,
		SubIntfIfIndex: subIntfIfIndex,
		IpAddr:         parsedIP,
		MaskLen:        maskLen,
		VlanId:         vid,
		StateUp:        stateUp,
		MacAddr:        l2Mac,
	}
	for _, plugin := range L3IntfMgr.plugins {
		// IP addr, net mask, ifindex, vlan id and id (used for adding link)
		rv := plugin.CreateSubIPv4Intf(obj, &ifName)
		if rv < 0 {
			return "", errors.New("Creating Sub IPv4 interface failed")
		}
	}
	return ifName, nil
}

func (iMgr *L3IntfManager) CreateSubIPv4Intf(confObj *asicdServices.SubIPv4Intf) (bool, error) {
	var subIntfIfIndex int32
	var tmpIfName string
	// Pre-check for netmask
	if !strings.Contains(confObj.IpAddr, "/") {
		return false, errors.New("Create sub interface failed, because " +
			"netmask is not specified for ip " + confObj.IpAddr)
	}
	ip, ipNet, err := net.ParseCIDR(confObj.IpAddr)
	if err != nil {
		return false, errors.New("Failed to parse ip addr during IPv4Intf create")
	}
	ip = ip.To4()
	parsedIP := uint32(ip[3]) | uint32(ip[2])<<8 | uint32(ip[1])<<16 | uint32(ip[0])<<24
	maskLen, _ := ipNet.Mask.Size()
	if maskLen > 32 || maskLen < 0 {
		return false, errors.New("Invalid netmask value")
	}
	// Get new id for sub interface
	switch confObj.Type {
	case "Secondary":
		tmpIfName = "Sec:" + confObj.IpAddr
		subIntfIfIndex = iMgr.intfMgr.AllocateIfIndex(tmpIfName, 0, commonDefs.IfTypeLoopback)
	case "Virtual":
		tmpIfName = "Virt:" + confObj.IpAddr
		subIntfIfIndex = iMgr.intfMgr.AllocateIfIndex(tmpIfName, 0, commonDefs.IfTypeVirtual)
	default:
		L3IntfMgr.logger.Info("Unknown sub ipv4 intf type: ", confObj.Type)
		return false, err
	}
	// Verify sub ipv4 interface config
	iMgr.dbMutex.Lock()
	err = VerifySubIPv4Config(parsedIP, maskLen)
	iMgr.dbMutex.Unlock()
	if err != nil {
		return false, err
	}
	//Convert parent intf ref to ifindex
	ifIndexList, err := iMgr.intfMgr.ConvertIntfStrListToIfIndexList([]string{confObj.IntfRef})
	if err != nil {
		return false, errors.New("Invalid interface reference provided in ipv4 sub intf config object")
	}
	parentIfIndex := ifIndexList[0]
	// We will create sub ipv4 interface only when the config object is enabled
	// Create Sub IPv4 interface internal
	ifName, err := CreateSubIPv4IntfInt(parentIfIndex, confObj.MacAddr, parsedIP, maskLen,
		confObj.Enable, subIntfIfIndex)
	if err != nil {
		return false, err
	}
	//Update intfMgr with ifName
	iMgr.intfMgr.UpdateIfNameForIfIndex(subIntfIfIndex, tmpIfName, ifName)
	subIntfObj := SubIntfObj{
		IfIndex:       subIntfIfIndex,
		ParentIfIndex: parentIfIndex,
		IpAddr:        confObj.IpAddr,
		MacAddr:       confObj.MacAddr,
		IfName:        ifName,
		Enable:        confObj.Enable,
	}
	//Update Sw State
	iMgr.dbMutex.Lock()
	iMgr.SubIPv4IntfDB = append(iMgr.SubIPv4IntfDB, subIntfObj)
	iMgr.dbMutex.Unlock()

	// Send (sub) ipv4 interface created notification
	var msg interface{}
	msg = pluginCommon.IPv4IntfNotifyMsg{
		IpAddr:  confObj.IpAddr,
		IfIndex: subIntfIfIndex,
	}
	SendIPCreateDelNotification(msg, pluginCommon.NOTIFY_IPV4INTF_CREATE)
	//Send out (sub) ipv4 state notification
	var state = 0
	if confObj.Enable {
		state = pluginCommon.INTF_STATE_UP
	} else {
		state = pluginCommon.INTF_STATE_DOWN
	}
	SendL3IntfStateNotification(confObj.IpAddr, pluginCommon.IP_TYPE_IPV4, subIntfIfIndex, state)
	return true, nil
}

func DeleteSubIPv4IntfInt(ifName string) int {
	for _, plugin := range L3IntfMgr.plugins {
		// IP addr, net mask, ifindex, vlan id and id (used for adding link)
		rv := plugin.DeleteSubIPv4Intf(ifName)
		if rv < 0 {
			return -1
		}
	}
	return 1
}

func FindSubIntfIPv4Index(ipAddr string) int {
	for idx, info := range L3IntfMgr.SubIPv4IntfDB {
		if strings.Compare(ipAddr, info.IpAddr) == 0 {
			return idx
		}
	}
	return -1
}

func (iMgr *L3IntfManager) DeleteSubIPv4Intf(subipv4IntfObj *asicdServices.SubIPv4Intf) (bool, error) {
	errString := ""
	iMgr.dbMutex.RLock()
	elemId := FindSubIntfIPv4Index(subipv4IntfObj.IpAddr)
	iMgr.dbMutex.RUnlock()
	if elemId < 0 {
		return false, errors.New("Cannot find entry in DB")
	}
	rv := DeleteSubIPv4IntfInt(iMgr.SubIPv4IntfDB[elemId].IfName)
	if rv < 0 {
		errString = "Failed to delete sub interface " +
			iMgr.SubIPv4IntfDB[elemId].IfName + " and ip address is " +
			subipv4IntfObj.IpAddr
		return false, errors.New(errString)
	}
	//Send out state notification
	SendL3IntfStateNotification(subipv4IntfObj.IpAddr, pluginCommon.IP_TYPE_IPV4, iMgr.SubIPv4IntfDB[elemId].IfIndex,
		pluginCommon.INTF_STATE_DOWN)
	var msg interface{}
	msg = pluginCommon.IPv4IntfNotifyMsg{
		IpAddr:  subipv4IntfObj.IpAddr,
		IfIndex: iMgr.SubIPv4IntfDB[elemId].IfIndex,
	}
	//Send out notification regarding ipv4 intf deletion - Ribd listens to this
	SendIPCreateDelNotification(msg, pluginCommon.NOTIFY_IPV4INTF_DELETE)
	//Free subIntfIfIndex
	iMgr.intfMgr.FreeIfIndex(iMgr.SubIPv4IntfDB[elemId].IfIndex)
	//Update Sw State
	iMgr.dbMutex.Lock()
	iMgr.SubIPv4IntfDB = append(iMgr.SubIPv4IntfDB[:elemId], iMgr.SubIPv4IntfDB[elemId+1:]...)
	iMgr.dbMutex.Unlock()
	return true, nil
}

func UpdateSubIPv4IntfInt(ifName string, macAddr net.HardwareAddr, ipAddr string,
	stateUp bool) int {
	ip, _, err := net.ParseCIDR(ipAddr)
	if err != nil {
		L3IntfMgr.logger.Err("Failed to parse ip addr",
			"during update subinterface for interface", ifName, "Err:", err)
		return -1
	}
	ip = ip.To4()
	parsedIP := uint32(ip[3]) | uint32(ip[2])<<8 |
		uint32(ip[1])<<16 | uint32(ip[0])<<24
	L3IntfMgr.logger.Info("Calling plugin updates for " + ifName)
	for _, plugin := range L3IntfMgr.plugins {
		rv := plugin.UpdateSubIPv4Intf(ifName, macAddr, parsedIP, stateUp)
		if rv < 0 {
			L3IntfMgr.logger.Err("Error from plugin returing early")
			return -1
		}
	}
	return 1
}

func (iMgr *L3IntfManager) UpdateSubIPv4Intf(oldObj, newObj *asicdServices.SubIPv4Intf,
	attrset []bool) (bool, error) {
	var rv int
	newMacAddr := ""
	stateChanged := false

	// Find the index for the element in DB
	iMgr.dbMutex.RLock()
	elemId := FindSubIntfIPv4Index(oldObj.IpAddr)
	iMgr.dbMutex.RUnlock()
	if elemId < 0 {
		return false, errors.New("Cannot find entry in DB & hence " +
			"Update cannot be performed")
	}
	// Check which attributes are changed
	for elem, _ := range attrset {
		if !attrset[elem] {
			continue
		}
		switch elem {
		case 0, 1, 2:
			return false, errors.New("Update for IpAddr/IfIndex/Type is not supported")
		case 3:
			//We will always use newconfig mac...regardless of attribute
			//being set or not
		case 4:
			stateChanged = true
		}
	}
	// If mac is changed then mac will not be nil otherwise mac will be nil
	mac, _ := net.ParseMAC(newObj.MacAddr)
	iMgr.logger.Info("New mac add is", mac, "for input", newMacAddr)
	if stateChanged {
		// if state changed then use new value
		rv = UpdateSubIPv4IntfInt(iMgr.SubIPv4IntfDB[elemId].IfName, mac,
			iMgr.SubIPv4IntfDB[elemId].IpAddr, newObj.Enable)
	} else { // else use old value
		rv = UpdateSubIPv4IntfInt(iMgr.SubIPv4IntfDB[elemId].IfName, mac,
			iMgr.SubIPv4IntfDB[elemId].IpAddr, oldObj.Enable)
	}
	if rv < 0 {
		return false, errors.New("Updating sub interface " +
			iMgr.SubIPv4IntfDB[elemId].IfName + "failed")
	}
	//Update Sw State
	iMgr.dbMutex.Lock()
	if newMacAddr != "" {
		iMgr.SubIPv4IntfDB[elemId].MacAddr = newObj.MacAddr
	}
	if stateChanged {
		iMgr.SubIPv4IntfDB[elemId].Enable = newObj.Enable
	}
	iMgr.dbMutex.Unlock()
	return true, nil
}

func (iMgr *L3IntfManager) GetBulkIPv6LinkScope(start, count int) (end, listLen int, more bool, lsList []*asicdServices.LinkScopeIpState) {
	var idx, numEntries int = 0, 0
	iMgr.dbMutex.RLock()
	for idx = start; idx < len(iMgr.linkLocalInUse); idx++ {
		entry, ok := iMgr.linkLocalMap[iMgr.linkLocalInUse[idx]]
		if !ok {
			continue
		}
		if numEntries == count {
			more = true
			break
		}
		if entry.Used {
			var lsObj asicdServices.LinkScopeIpState
			lsObj.IntfRef = entry.IntfRef
			lsObj.LinkScopeIp = entry.LinkScopeIp
			lsObj.Used = entry.Used
			lsList = append(lsList, &lsObj)
			numEntries += 1
		}
	}
	iMgr.dbMutex.RUnlock()
	end = idx
	if end == len(iMgr.linkLocalInUse) {
		more = false
	}
	return end, len(lsList), more, lsList

}

func (iMgr *L3IntfManager) GetIPv6LinkScope(lsIpAddr string) (*asicdServices.LinkScopeIpState, error) {
	iMgr.dbMutex.RLock()
	if val, ok := iMgr.linkLocalMap[lsIpAddr]; ok {
		lsObj := new(asicdServices.LinkScopeIpState)
		lsObj.LinkScopeIp = lsIpAddr
		lsObj.IntfRef = val.IntfRef
		lsObj.Used = val.Used
		iMgr.dbMutex.RUnlock()
		return lsObj, nil
	} else {
		iMgr.dbMutex.RUnlock()
		return nil, errors.New(fmt.Sprintln("Unable to locate Link Scope entry for Ip:-", lsIpAddr))
	}
}
