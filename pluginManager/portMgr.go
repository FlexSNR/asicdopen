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
	"fmt"
	"models/events"
	"models/objects"
	"strings"
	"sync"
	"time"
	"utils/commonDefs"
	"utils/dbutils"
	"utils/eventUtils"
	"utils/logging"
)

type portConfig struct {
	asicdServices.Port
	mappedToHw        bool
	breakOutSupported bool
}

type PortManager struct {
	dbMutex              sync.RWMutex
	portUpdateMutex      sync.RWMutex
	ifMgr                *IfManager
	vlanMgr              *VlanManager
	lagMgr               *LagManager
	l3IntfMgr            *L3IntfManager
	maxPorts             int
	initComplete         bool
	portConfigDB         map[int32]*portConfig
	portStateDB          map[int32]*asicdServices.PortState
	bufferPortStateDB    map[int32]*asicdServices.BufferPortStatState
	portNumToIfIndexXref map[int32][]int32
	ifIndexToPortNumXref map[int32]int32
	ifIndexKeyCache      []int32
	logger               *logging.Writer
	plugins              []PluginIntf
	notifyChannel        chan<- []byte
}

//Port manager db
var PortMgr PortManager

/*
 * Called at init time to setup port state DB
 * - Allocates ifindex for port
 * - Sets up cross references between port num and ifindex
 * - Inserts into ifindex key cache for getbulk
 */
func InitPortStateDB(portNum int32, portName string) {
	var ifIndex int32
	PortMgr.dbMutex.Lock()
	//Allocate if index, setup cross references
	ifIndex = PortMgr.ifMgr.AllocateIfIndex(portName, 0, commonDefs.IfTypePort)
	PortMgr.ifIndexKeyCache = append(PortMgr.ifIndexKeyCache, ifIndex)
	PortMgr.ifIndexToPortNumXref[ifIndex] = portNum
	PortMgr.portNumToIfIndexXref[portNum] = append(PortMgr.portNumToIfIndexXref[portNum], ifIndex)
	PortMgr.portStateDB[ifIndex] = asicdServices.NewPortState()
	PortMgr.portStateDB[ifIndex].IfIndex = ifIndex
	PortMgr.portStateDB[ifIndex].Name = portName
	PortMgr.portStateDB[ifIndex].IntfRef = portName
	PortMgr.portStateDB[ifIndex].Pvid = int32(pluginCommon.DEFAULT_VLAN_ID)
	PortMgr.portStateDB[ifIndex].PresentInHW = "NO"
	PortMgr.portStateDB[ifIndex].OperState = "DOWN"
	PortMgr.portStateDB[ifIndex].ConfigMode = pluginCommon.PORT_MODE_UNCONFIGURED
	PortMgr.dbMutex.Unlock()
}

func InitBufferPortStateDB(portNum int32, portName string) {
	ifIndex, err := PortMgr.GetIfIndexFromPortNum(portNum)
	if err != nil {
		return
	}
	PortMgr.dbMutex.Lock()
	PortMgr.bufferPortStateDB[ifIndex] = asicdServices.NewBufferPortStatState()
	PortMgr.dbMutex.Unlock()
}

/* Called at init time only, to setup port config DB */
func InitPortConfigDB(pCfg *pluginCommon.PortConfig) {
	var ifIndex int32
	//Determine ifindex list for port
	if ifIndexList, ok := PortMgr.portNumToIfIndexXref[pCfg.PortNum]; ok {
		PortMgr.dbMutex.Lock()
		if pCfg.LogicalPortInfo {
			ifIndex = ifIndexList[1]
		} else {
			ifIndex = ifIndexList[0]
		}
		var portObj portConfig
		obj := PortMgr.GetThriftPortObjFromPluginObj(ifIndex, pCfg)
		portObj = *obj
		PortMgr.portConfigDB[ifIndex] = &portObj
		//Update port state that is dependent on config change
		if PortMgr.portConfigDB[ifIndex].mappedToHw {
			PortMgr.portStateDB[ifIndex].PresentInHW = "YES"
		} else {
			if !pCfg.LogicalPortInfo && PortMgr.portConfigDB[ifIndex].breakOutSupported {
				PortMgr.portStateDB[ifIndex].OperState = "Port broken out"
			}
			PortMgr.portStateDB[ifIndex].PresentInHW = "NO"
		}
		PortMgr.dbMutex.Unlock()
	}
}

func (pMgr *PortManager) GetAllIntfRef() (ifList []string) {
	pMgr.dbMutex.RLock()
	defer pMgr.dbMutex.RUnlock()
	for _, val := range pMgr.portConfigDB {
		ifList = append(ifList, val.IntfRef)
	}
	return ifList
}

func (pMgr *PortManager) GetNumOperStateUPPorts() int {
	var cnt int
	PortMgr.dbMutex.RLock()
	defer PortMgr.dbMutex.RUnlock()
	for ifIndex, val := range pMgr.portStateDB {
		if (val.OperState == INTF_STATE_UP) &&
			pMgr.portConfigDB[ifIndex].mappedToHw {
			cnt++
		}
	}
	return cnt
}

func (pMgr *PortManager) GetNumOperStateDOWNPorts() int {
	var cnt int
	PortMgr.dbMutex.RLock()
	defer PortMgr.dbMutex.RUnlock()
	for ifIndex, val := range pMgr.portStateDB {
		if (val.OperState == INTF_STATE_DOWN) &&
			pMgr.portConfigDB[ifIndex].mappedToHw {
			cnt++
		}
	}
	return cnt
}

/*
 *  UpdatePortStateDB is used during GetBulkPortState or GetPortState, so that we have latest stats counter
 *  from the hardware. We should not update anything else into the state db.
 */
func UpdatePortStateDB(pState *pluginCommon.PortState) {
	ifIndex, err := PortMgr.GetIfIndexFromPortNum(pState.PortNum)
	if err == nil {
		PortMgr.dbMutex.Lock()
		PortMgr.portStateDB[ifIndex].IfInOctets = pState.IfInOctets
		PortMgr.portStateDB[ifIndex].IfInUcastPkts = pState.IfInUcastPkts
		PortMgr.portStateDB[ifIndex].IfInDiscards = pState.IfInDiscards
		PortMgr.portStateDB[ifIndex].IfInErrors = pState.IfInErrors
		PortMgr.portStateDB[ifIndex].IfInUnknownProtos = pState.IfInUnknownProtos
		PortMgr.portStateDB[ifIndex].IfOutOctets = pState.IfOutOctets
		PortMgr.portStateDB[ifIndex].IfOutUcastPkts = pState.IfOutUcastPkts
		PortMgr.portStateDB[ifIndex].IfOutDiscards = pState.IfOutDiscards
		PortMgr.portStateDB[ifIndex].IfOutErrors = pState.IfOutErrors
		PortMgr.portStateDB[ifIndex].IfEtherUnderSizePktCnt = pState.IfEtherUnderSizePktCnt
		PortMgr.portStateDB[ifIndex].IfEtherOverSizePktCnt = pState.IfEtherOverSizePktCnt
		PortMgr.portStateDB[ifIndex].IfEtherFragments = pState.IfEtherFragments
		PortMgr.portStateDB[ifIndex].IfEtherCRCAlignError = pState.IfEtherCRCAlignError
		PortMgr.portStateDB[ifIndex].IfEtherJabber = pState.IfEtherJabber
		PortMgr.portStateDB[ifIndex].IfEtherPkts = pState.IfEtherPkts
		PortMgr.portStateDB[ifIndex].IfEtherMCPkts = pState.IfEtherMCPkts
		PortMgr.portStateDB[ifIndex].IfEtherBcastPkts = pState.IfEtherBcastPkts
		PortMgr.portStateDB[ifIndex].IfEtherPkts64OrLessOctets = pState.IfEtherPkts64OrLessOctets
		PortMgr.portStateDB[ifIndex].IfEtherPkts65To127Octets = pState.IfEtherPkts65To127Octets
		PortMgr.portStateDB[ifIndex].IfEtherPkts128To255Octets = pState.IfEtherPkts128To255Octets
		PortMgr.portStateDB[ifIndex].IfEtherPkts256To511Octets = pState.IfEtherPkts256To511Octets
		PortMgr.portStateDB[ifIndex].IfEtherPkts512To1023Octets = pState.IfEtherPkts512To1023Octets
		PortMgr.portStateDB[ifIndex].IfEtherPkts1024To1518Octets = pState.IfEtherPkts1024To1518Octets
		PortMgr.portStateDB[ifIndex].PRBSRxErrCnt = pState.PRBSRxErrCnt
		PortMgr.dbMutex.Unlock()
	}
}

func UpdateBufferPortStateDB(bState *pluginCommon.BufferPortState) {
	//ifIndexList, err := PortMgr.ifMgr.ConvertIntfStrListToIfIndexList([]string{bState.IntfRef})
	ifIndex, err := PortMgr.GetIfIndexFromPortNum(bState.IfIndex)
	if err == nil {
		//ifIndex := ifIndexList[0]
		if bState == nil {
			return
		}
		PortMgr.dbMutex.Lock()
		_, ok := PortMgr.bufferPortStateDB[ifIndex]
		if !ok {
			PortMgr.bufferPortStateDB[ifIndex] = asicdServices.NewBufferPortStatState()
		}
		bufferPortStateDB := PortMgr.bufferPortStateDB[ifIndex]
		bufferPortStateDB.IntfRef = bState.IntfRef
		bufferPortStateDB.IfIndex = bState.IfIndex
		bufferPortStateDB.EgressPort = int64(bState.EgressPort)
		bufferPortStateDB.PortBufferStat = int64(bState.PortBufferPortStat)
		PortMgr.bufferPortStateDB[ifIndex] = bufferPortStateDB
		PortMgr.dbMutex.Unlock()
	}
}

func (pMgr *PortManager) Init(rsrcMgrs *ResourceManagers, dbHdl *dbutils.DBUtil, pluginList []PluginIntf, logger *logging.Writer, notifyChannel chan<- []byte) {
	var portmap map[int32]string = make(map[int32]string)
	var buffermap map[int32]string = make(map[int32]string)
	pMgr.ifMgr = rsrcMgrs.IfManager
	pMgr.vlanMgr = rsrcMgrs.VlanManager
	pMgr.lagMgr = rsrcMgrs.LagManager
	pMgr.l3IntfMgr = rsrcMgrs.L3IntfManager
	pMgr.logger = logger
	pMgr.plugins = pluginList
	pMgr.notifyChannel = notifyChannel
	pMgr.portConfigDB = make(map[int32]*portConfig)
	pMgr.portStateDB = make(map[int32]*asicdServices.PortState)
	pMgr.bufferPortStateDB = make(map[int32]*asicdServices.BufferPortStatState)
	pMgr.portNumToIfIndexXref = make(map[int32][]int32)
	pMgr.ifIndexToPortNumXref = make(map[int32]int32)
	for _, plugin := range pMgr.plugins {
		if isControllingPlugin(plugin) == true {
			pMgr.maxPorts = plugin.GetMaxSysPorts()
			if pMgr.maxPorts < 0 {
				pMgr.logger.Err("Failed to retrieve max port count during port manager init")
			}
			rv := plugin.InitPortStateDB(portmap)
			if rv < 0 {
				pMgr.logger.Err("Failed to retrieve port state info from plugin")
			}
			rv = plugin.InitPortConfigDB()
			if rv < 0 {
				pMgr.logger.Err("Failed to retrieve port config info from plugin")
			}
			rv = plugin.InitBufferPortStateDB(buffermap)
			if rv < 0 {
				pMgr.logger.Err("Failed to retrieve buffer state info from plugin")
			}
		} else {
			pMgr.dbMutex.RLock()
			for _, ifIndex := range pMgr.ifIndexKeyCache {
				portmap[ifIndex] = pMgr.portStateDB[ifIndex].Name
			}
			pMgr.dbMutex.RUnlock()
			plugin.InitPortStateDB(portmap)
			plugin.InitBufferPortStateDB(buffermap)
		}
	}
	PortMgr.logger.Info("Init: maxPorts in system are:", pMgr.maxPorts)
	PortMgr.initComplete = true
	if dbHdl != nil {
		var dbObj objects.Port
		var portObjList []*asicdServices.Port
		var breakOutportObjList []*asicdServices.Port
		objList, err := dbHdl.GetAllObjFromDb(dbObj)
		if err != nil {
			logger.Err("DB Query failed during Port Init")
			return
		}
		for _, val := range objList {
			obj := asicdServices.NewPort()
			dbObject := val.(objects.Port)
			objects.ConvertasicdPortObjToThrift(&dbObject, obj)
			if obj.BreakOutMode == pluginCommon.PORT_BREAKOUT_MODE_UNSUPPORTED_STRING {
				//If breakout not supported add to tail of slice
				breakOutportObjList = append(breakOutportObjList, obj)
			} else {
				//If breakout mode is set add to head of slice
				portObjList = append([]*asicdServices.Port{obj}, portObjList...)
			}
		}

		portObjList = append(portObjList, breakOutportObjList...)
		for _, val := range portObjList {
			rv, err := pMgr.UpdatePortConfig(nil, val, nil)
			if rv == false {
				logger.Err(fmt.Sprintln("Failed to update port config", err))
				return
			}
		}
	}
}

func (pMgr *PortManager) Deinit() {
	/* Currently no-op */
}

func (pMgr *PortManager) GetMaxSysPorts() int {
	return pMgr.maxPorts
}

func (pMgr *PortManager) GetIfIndexFromPortNum(port int32) (int32, error) {
	var ifIndex int32
	var ifIndexList []int32
	var ok bool
	if ifIndexList, ok = pMgr.portNumToIfIndexXref[port]; !ok {
		pMgr.logger.Warning("Unable to locate ifindex for port - ", port)
		return 0, errors.New("Unable to locate ifindex for port")
	} else {
		pMgr.dbMutex.RLock()
		for _, ifIndex = range ifIndexList {
			if pMgr.portConfigDB[ifIndex].mappedToHw {
				break
			}
		}
		pMgr.dbMutex.RUnlock()
	}
	return ifIndex, nil
}

func (pMgr *PortManager) GetPortNumFromIfIndex(ifIndex int32) int32 {
	var port int32
	var ok bool
	if port, ok = pMgr.ifIndexToPortNumXref[ifIndex]; !ok {
		pMgr.logger.Warning(fmt.Sprintln("Unable to locate port num for ifIndex - ", ifIndex))
	}
	return port
}

func (pMgr *PortManager) ErrorDisablePort(ifIndex int32, adminState string, ErrDisableReason string) (bool, error) {
	PortMgr.dbMutex.Lock()
	pMgr.portStateDB[ifIndex].ErrDisableReason = ErrDisableReason
	pMgr.portConfigDB[ifIndex].AdminState = adminState
	PortMgr.dbMutex.Unlock()
	portNum := pMgr.GetPortNumFromIfIndex(ifIndex)
	for _, plugin := range pMgr.plugins {
		rv := plugin.PortEnableSet(portNum, adminState)
		if rv < 0 {
			return false, errors.New("Failed to ErrDisable port")
		}
	}
	return true, nil
}

func (pMgr *PortManager) NotifyMtuChange(ifIndexList []int32, newMtu int32) {
	for _, ifIndex := range ifIndexList {
		msg := pluginCommon.PortConfigMtuChangeNotifyMsg{
			IfIndex: ifIndex,
			Mtu:     newMtu,
		}
		msgBuf, err := json.Marshal(msg)
		if err != nil {
			PortMgr.logger.Err("Error in marshalling Json, in PortConfigMtuChangeNotifyMsg")
		}
		notification := pluginCommon.AsicdNotification{
			MsgType: uint8(pluginCommon.NOTIFY_PORT_CONFIG_MTU_CHANGE),
			Msg:     msgBuf,
		}
		notificationBuf, err := json.Marshal(notification)
		if err != nil {
			PortMgr.logger.Err("Failed to marshal PortConfigMtuChangeNotifyMsg")
		}
		PortMgr.notifyChannel <- notificationBuf
	}
}

func (pMgr *PortManager) SetPortConfigMode(mode string, ifIndexList []int32) {
	var oldCfgMode string
	pMgr.dbMutex.Lock()
	defer pMgr.dbMutex.Unlock()
	for _, ifIndex := range ifIndexList {
		if commonDefs.IfTypePort == pluginCommon.GetTypeFromIfIndex(ifIndex) {
			pInfo := pMgr.portStateDB[ifIndex]
			oldCfgMode = pInfo.ConfigMode
			pInfo.ConfigMode = mode
			//Send notification of config mode change
			msg := pluginCommon.PortConfigModeChgNotifyMsg{
				IfIndex: ifIndex,
				OldMode: oldCfgMode,
				NewMode: mode,
			}
			msgBuf, err := json.Marshal(msg)
			if err != nil {
				PortMgr.logger.Err("Error in marshalling Json, when sending port config mode change notification")
			}
			notification := pluginCommon.AsicdNotification{
				MsgType: uint8(pluginCommon.NOTIFY_PORT_CONFIG_MODE_CHANGE),
				Msg:     msgBuf,
			}
			notificationBuf, err := json.Marshal(notification)
			if err != nil {
				PortMgr.logger.Err("Failed to marshal port config change message")
			}
			PortMgr.notifyChannel <- notificationBuf
		}
	}
}

func (pMgr *PortManager) GetPortConfigMode(ifIndex int32) string {
	var mode string
	pMgr.dbMutex.RLock()
	defer pMgr.dbMutex.RUnlock()
	if commonDefs.IfTypePort == pluginCommon.GetTypeFromIfIndex(ifIndex) {
		mode = pMgr.portStateDB[ifIndex].ConfigMode
	} else {
		//If ifType != Port, then mode has to be l2 (vlan/lag member)
		mode = pluginCommon.PORT_MODE_L2
	}
	return mode
}

func (pMgr *PortManager) SetPvidForIfIndexList(pvid int, ifIndexList []int32) {
	pMgr.dbMutex.Lock()
	defer pMgr.dbMutex.Unlock()
	for _, ifIndex := range ifIndexList {
		ifType := pluginCommon.GetTypeFromIfIndex(ifIndex)
		switch ifType {
		case commonDefs.IfTypePort:
			pState := pMgr.portStateDB[ifIndex]
			pState.Pvid = int32(pvid)
		case commonDefs.IfTypeLag:
			//Get LAG members
			list := pMgr.lagMgr.GetLagMembers(ifIndex)
			for _, pIfIndex := range list {
				pState := pMgr.portStateDB[pIfIndex]
				pState.Pvid = int32(pvid)
			}
		}
	}
}

func (pMgr *PortManager) GetPvidForIfIndex(ifIndex int32) int {
	var pvid int = pluginCommon.DEFAULT_VLAN_ID
	pMgr.dbMutex.RLock()
	defer pMgr.dbMutex.RUnlock()
	ifType := pluginCommon.GetTypeFromIfIndex(ifIndex)
	switch ifType {
	case commonDefs.IfTypePort:
		pvid = int(pMgr.portStateDB[ifIndex].Pvid)
	case commonDefs.IfTypeLag:
		//Get LAG members
		list := pMgr.lagMgr.GetLagMembers(ifIndex)
		for _, pIfIndex := range list {
			pvid = int(pMgr.portStateDB[pIfIndex].Pvid)
			break
		}
	}
	return pvid
}

func (pMgr *PortManager) GetThriftPortObjFromPluginObj(ifIndex int32, pCfg *pluginCommon.PortConfig) *portConfig {
	ifName, err := pMgr.ifMgr.GetIfNameForIfIndex(ifIndex)
	if err != nil {
		pMgr.logger.Err("Unable to locate ifname when converting port config to thrift obj")
		return nil
	}
	obj := new(portConfig)
	obj.IntfRef = ifName
	obj.IfIndex = ifIndex
	obj.Description = pCfg.Description
	obj.PhyIntfType = pCfg.PhyIntfType
	obj.AdminState = pCfg.AdminState
	obj.MacAddr = pCfg.MacAddr
	obj.Speed = pCfg.Speed
	obj.Duplex = pCfg.Duplex
	obj.Autoneg = pCfg.Autoneg
	obj.MediaType = pCfg.MediaType
	obj.Mtu = pCfg.Mtu
	obj.BreakOutMode = pMgr.ConvertBreakOutModeEnumToString(pCfg.BreakOutMode)
	obj.breakOutSupported = pCfg.BreakOutSupported
	obj.mappedToHw = pCfg.MappedToHw
	obj.LoopbackMode = pluginCommon.LBModeEnumToStr[pCfg.LoopbackMode]
	obj.EnableFEC = pCfg.EnableFEC
	obj.PRBSTxEnable = pCfg.PRBSTxEnable
	obj.PRBSRxEnable = pCfg.PRBSRxEnable
	obj.PRBSPolynomial = pluginCommon.PrbsEnumToStr[pCfg.PRBSPolynomial]
	return obj
}

func (pMgr *PortManager) GetPluginPortObjFromThriftObj(portObj *asicdServices.Port) (*pluginCommon.PortConfig, int32, error) {
	ifIndexList, err := pMgr.ifMgr.ConvertIntfStrListToIfIndexList([]string{portObj.IntfRef})
	if err != nil {
		pMgr.logger.Err("Invalid intfref argument when converting thrift obj to port config: ERROR:", err, "port config:", portObj.IntfRef)
		return nil, int32(0), err
	}
	ifIndex := ifIndexList[0]
	portNum := pMgr.GetPortNumFromIfIndex(ifIndex)
	obj := new(pluginCommon.PortConfig)
	obj.PortNum = portNum
	obj.PortName = portObj.IntfRef
	obj.Description = portObj.Description
	obj.PhyIntfType = portObj.PhyIntfType
	obj.AdminState = portObj.AdminState
	obj.MacAddr = portObj.MacAddr
	obj.Speed = portObj.Speed
	obj.Duplex = portObj.Duplex
	obj.Autoneg = portObj.Autoneg
	obj.MediaType = portObj.MediaType
	obj.Mtu = portObj.Mtu
	obj.BreakOutMode = pMgr.ConvertBreakOutModeUsrStringToEnum(portObj.BreakOutMode)
	obj.LoopbackMode = pluginCommon.LBModeStrToEnum[portObj.LoopbackMode]
	obj.EnableFEC = portObj.EnableFEC
	obj.PRBSTxEnable = portObj.PRBSTxEnable
	obj.PRBSRxEnable = portObj.PRBSRxEnable
	obj.PRBSPolynomial = pluginCommon.PrbsStrToEnum[portObj.PRBSPolynomial]
	return obj, ifIndex, nil
}

func (pMgr *PortManager) DiffWithDBObjAndSetFlags(obj *asicdServices.Port) int32 {
	var flags int32 = 0
	pMgr.dbMutex.RLock()
	defer pMgr.dbMutex.RUnlock()
	dbObj := pMgr.portConfigDB[obj.IfIndex]
	if obj.AdminState != dbObj.AdminState {
		flags |= pluginCommon.PORT_ATTR_ADMIN_STATE
	}
	if obj.Speed != dbObj.Speed {
		flags |= pluginCommon.PORT_ATTR_SPEED
	}
	if obj.Duplex != dbObj.Duplex {
		flags |= pluginCommon.PORT_ATTR_DUPLEX
	}
	if obj.Autoneg != dbObj.Autoneg {
		flags |= pluginCommon.PORT_ATTR_AUTONEG
	}
	if obj.Mtu != dbObj.Mtu {
		flags |= pluginCommon.PORT_ATTR_MTU
	}
	if dbObj.breakOutSupported {
		if obj.BreakOutMode != dbObj.BreakOutMode {
			flags |= pluginCommon.PORT_ATTR_BREAKOUT_MODE
		}
	}
	if obj.EnableFEC != dbObj.EnableFEC {
		flags |= pluginCommon.PORT_ATTR_ENABLE_FEC
	}
	if obj.PRBSTxEnable != dbObj.PRBSTxEnable {
		flags |= pluginCommon.PORT_ATTR_TX_PRBS_EN
	}
	if obj.PRBSRxEnable != dbObj.PRBSRxEnable {
		flags |= pluginCommon.PORT_ATTR_RX_PRBS_EN
	}
	if obj.PRBSPolynomial != dbObj.PRBSPolynomial {
		flags |= pluginCommon.PORT_ATTR_PRBS_POLY
	}
	return flags
}

func (pMgr *PortManager) ConvertUpdateAttrSetToFlags(attrset []bool) int32 {
	var flags int32
	for idx, val := range attrset {
		if val {
			switch idx {
			case 3:
				flags |= pluginCommon.PORT_ATTR_PHY_INTF_TYPE
			case 4:
				flags |= pluginCommon.PORT_ATTR_ADMIN_STATE
			case 5:
				flags |= pluginCommon.PORT_ATTR_MAC_ADDR
			case 6:
				flags |= pluginCommon.PORT_ATTR_SPEED
			case 7:
				flags |= pluginCommon.PORT_ATTR_DUPLEX
			case 8:
				flags |= pluginCommon.PORT_ATTR_AUTONEG
			case 9:
				flags |= pluginCommon.PORT_ATTR_MEDIA_TYPE
			case 10:
				flags |= pluginCommon.PORT_ATTR_MTU
			case 11:
				flags |= pluginCommon.PORT_ATTR_BREAKOUT_MODE
			case 12:
				flags |= pluginCommon.PORT_ATTR_LOOPBACK_MODE
			case 13:
				flags |= pluginCommon.PORT_ATTR_ENABLE_FEC
			case 14:
				flags |= pluginCommon.PORT_ATTR_TX_PRBS_EN
			case 15:
				flags |= pluginCommon.PORT_ATTR_RX_PRBS_EN
			case 16:
				flags |= pluginCommon.PORT_ATTR_PRBS_POLY
			}
		}
	}
	return flags
}

func (pMgr *PortManager) UpdatePortConfig(oldPortObj, newPortObj *asicdServices.Port, attrset []bool) (bool, error) {
	var updateFlags int32 = 0
	portObj, portIfIndex, err := pMgr.GetPluginPortObjFromThriftObj(newPortObj)
	if err != nil {
		return false, err
	}
	if attrset == nil {
		updateFlags = pMgr.DiffWithDBObjAndSetFlags(newPortObj)
	} else {
		updateFlags = pMgr.ConvertUpdateAttrSetToFlags(attrset)
	}
	if (updateFlags & pluginCommon.PORT_ATTR_MAC_ADDR) == pluginCommon.PORT_ATTR_MAC_ADDR {
		return false, errors.New("Updating mac address on port object is currently unsupported")
	}
	if (updateFlags & pluginCommon.PORT_ATTR_PHY_INTF_TYPE) == pluginCommon.PORT_ATTR_PHY_INTF_TYPE {
		return false, errors.New("Updating phy interface type on port object is currently unsupported")
	}
	if (updateFlags & pluginCommon.PORT_ATTR_MEDIA_TYPE) == pluginCommon.PORT_ATTR_MEDIA_TYPE {
		return false, errors.New("Updating media type on port object is currently unsupported")
	}
	if (updateFlags & pluginCommon.PORT_ATTR_BREAKOUT_MODE) == pluginCommon.PORT_ATTR_BREAKOUT_MODE {
		//Check if port supports breakout
		pMgr.dbMutex.RLock()
		if !pMgr.portConfigDB[portIfIndex].breakOutSupported {
			pMgr.dbMutex.RUnlock()
			return false, errors.New(fmt.Sprintln("Updating break out mode not supportd on this port - ", portIfIndex))
		}
		pMgr.dbMutex.RUnlock()
	}
	breakOutPortsList := make([]int32, 10) // This is hardcoded to 10 assuming that we support 4 breakout ports only
	var ifIndexPluginList []int32
	//Following lock needed to ensure link state change notifications are not sent until update in all plugins is complete
	pMgr.portUpdateMutex.Lock()
	for _, plugin := range pMgr.plugins {
		if isControllingPlugin(plugin) == true { // removed the check for breakout mode
			rv := plugin.UpdatePortConfig(updateFlags, portObj, breakOutPortsList)
			if rv < 0 {
				pMgr.portUpdateMutex.Unlock()
				return false, errors.New("Failed to update port configuration")
			}
			for _, portNum := range breakOutPortsList {
				if portNum == -1 {
					break
				}
				if portNum == 0 {
					continue
				}
				ifIndex, err := pMgr.GetIfIndexFromPortNum(portNum)
				if err == nil {
					ifIndexPluginList = append(ifIndexPluginList, ifIndex)
				}
			}
		} else { // linux is not the controlling plugin
			if !isPluginAsicDriver(plugin) {
				// this is linux plugin... so get the ifIndex from portNum
				rv := plugin.UpdatePortConfig(updateFlags, portObj, ifIndexPluginList)
				if rv < 0 {
					pMgr.portUpdateMutex.Unlock()
					return false, errors.New("Failed to update port configuration in linux")
				}
			}
		}
	}
	pMgr.portUpdateMutex.Unlock()
	ifIndexList, err := pMgr.ifMgr.ConvertIntfStrListToIfIndexList([]string{newPortObj.IntfRef})
	if err != nil {
		pMgr.logger.Err("Invalid intfref argument in get port config")
		return false, errors.New("Invalid intf ref in port config get request")
	}
	ifIndex := ifIndexList[0]
	pMgr.dbMutex.Lock()
	info := pMgr.portConfigDB[ifIndex]
	info.Port = *newPortObj
	pMgr.portConfigDB[ifIndex] = info
	pMgr.dbMutex.Unlock()
	//notify Mtu change
	if (updateFlags & pluginCommon.PORT_ATTR_MTU) == pluginCommon.PORT_ATTR_MTU {
		pMgr.NotifyMtuChange(ifIndexList, newPortObj.Mtu)
	}
	return true, nil
}

func (pMgr *PortManager) GetPortConfig(intfRef string) (*asicdServices.Port, error) {
	ifIndexList, err := pMgr.ifMgr.ConvertIntfStrListToIfIndexList([]string{intfRef})
	if err != nil {
		pMgr.logger.Err("Invalid intfref argument in get port config")
		return nil, errors.New("Invalid intf ref in port config get request")
	}
	ifIndex := ifIndexList[0]
	pMgr.dbMutex.RLock()
	pCfg := pMgr.portConfigDB[ifIndex]
	pMgr.dbMutex.RUnlock()
	return &pCfg.Port, nil
}

func (pMgr *PortManager) GetBulkPortConfig(start, count int) (end, listLen int, more bool, pCfgList []*asicdServices.Port) {
	var numEntries, idx int = 0, 0
	if (start > len(pMgr.ifIndexKeyCache)) || (start < 0) {
		pMgr.logger.Err("Invalid start port argument in get bulk port config")
		return -1, 0, false, nil
	}
	if count < 0 {
		pMgr.logger.Err("Invalid count in get bulk port config")
		return -1, 0, false, nil
	}
	pMgr.dbMutex.RLock()
	for idx = start; idx < len(pMgr.ifIndexKeyCache); idx++ {
		if numEntries == count {
			more = true
			break
		}
		ifIndex := pMgr.ifIndexKeyCache[idx]
		pCfgList = append(pCfgList, &pMgr.portConfigDB[ifIndex].Port)
		numEntries += 1
	}
	pMgr.dbMutex.RUnlock()
	end = idx
	if idx == len(pMgr.ifIndexKeyCache) {
		more = false
	}
	return end, len(pCfgList), more, pCfgList
}

func (pMgr *PortManager) GetPortState(intfRef string) (*asicdServices.PortState, error) {
	ifIndexList, err := pMgr.ifMgr.ConvertIntfStrListToIfIndexList([]string{intfRef})
	if err != nil {
		pMgr.logger.Err("Invalid intfref argument in get port state ")
		return nil, errors.New("Invalid intf ref in port state get request")
	}
	ifIndex := ifIndexList[0]
	port := pMgr.ifIndexToPortNumXref[ifIndex]
	for _, plugin := range pMgr.plugins {
		if isControllingPlugin(plugin) == true {
			rv := plugin.UpdatePortStateDB(port, int32(int(port)+1))
			if rv < 0 {
				return nil, errors.New("Failed to retrieve port state info from plugin")
			}
		}
	}
	pMgr.dbMutex.RLock()
	pState := pMgr.portStateDB[ifIndex]
	pMgr.dbMutex.RUnlock()
	return pState, nil
}

func (pMgr *PortManager) GetBulkPortState(start, count int) (end, listLen int, more bool, pStateList []*asicdServices.PortState) {
	var numEntries, idx int = 0, 0
	if (start > len(pMgr.ifIndexKeyCache)) || (start < 0) {
		pMgr.logger.Err("Invalid start port argument during get bulk port state start", start,
			"Max Ports on system", pMgr.maxPorts)
		return -1, 0, false, nil
	}
	if count < 0 {
		pMgr.logger.Err("Invalid count during get bulk port state")
		return -1, 0, false, nil
	}
	if pMgr.ifIndexKeyCache == nil {
		pMgr.logger.Err("pMgr.ifIndexKeyCache not initialized")
		return 0, 0, false, nil
	}
	for idx = start; idx < len(pMgr.ifIndexKeyCache); idx++ {
		if numEntries == count {
			more = true
			break
		}
		ifIndex := pMgr.ifIndexKeyCache[idx]
		port := pMgr.ifIndexToPortNumXref[ifIndex]
		for _, plugin := range pMgr.plugins {
			if isControllingPlugin(plugin) == true {
				rv := plugin.UpdatePortStateDB(port, port+int32(1))
				if rv < 0 {
					pMgr.logger.Err("Failed to retrieve port state info from plugin")
					return -1, 0, false, nil
				}
			}
		}
		numEntries += 1
	}
	pMgr.dbMutex.RLock()
	for idx = start; idx < start+numEntries; idx++ {
		ifIndex := pMgr.ifIndexKeyCache[idx]
		pStateList = append(pStateList, pMgr.portStateDB[ifIndex])
	}
	pMgr.dbMutex.RUnlock()
	end = idx
	if idx == len(pMgr.ifIndexKeyCache) {
		more = false
	}
	return end, len(pStateList), more, pStateList
}

func (pMgr *PortManager) GetBulkBufferPortStatState(start, count int) (end, listLen int, more bool, bStateList []*asicdServices.BufferPortStatState) {
	var numEntries, idx int = 0, 0
	if (start > len(pMgr.ifIndexKeyCache)) || (start < 0) {
		pMgr.logger.Err("Invalid start port argument during get bulk buffer state")
		return -1, 0, false, nil
	}
	if count < 0 {
		pMgr.logger.Err("Invalid count during get bulk buffer state")
		return -1, 0, false, nil
	}
	if pMgr.ifIndexKeyCache == nil {
		pMgr.logger.Err("BufferStats: pMgr.ifIndexKeyCache not initialized")
		return 0, 0, false, nil
	}

	index := pMgr.ifIndexKeyCache[start]
	port := pMgr.ifIndexToPortNumXref[index]
	endPort := int(start) + count
	if endPort > len(pMgr.ifIndexKeyCache) {
		endPort = len(pMgr.ifIndexKeyCache) + 1
	}
	for _, plugin := range pMgr.plugins {
		if isControllingPlugin(plugin) == true {
			rv := plugin.UpdateBufferPortStateDB(port, int32(endPort))
			if rv < 0 {
				pMgr.logger.Err("Failed to retrieve buffer state info from plugin")
				return -1, 0, false, nil
			}
		}
	}
	pMgr.dbMutex.RLock()
	for idx = start; idx < len(pMgr.ifIndexKeyCache); idx++ {
		if numEntries == count {
			more = true
			break
		}
		ifIndex := pMgr.ifIndexKeyCache[idx]
		if pMgr.bufferPortStateDB[ifIndex] == nil {
			continue
		}
		bStateList = append(bStateList, pMgr.bufferPortStateDB[ifIndex])
		numEntries += 1
	}
	pMgr.dbMutex.RUnlock()
	end = idx
	if idx == len(pMgr.ifIndexKeyCache) {
		more = false
	}
	return end, len(bStateList), more, bStateList
}

func (pMgr *PortManager) GetBufferPortStatState(intfRef string) (*asicdServices.BufferPortStatState, error) {
	ifIndexList, err := pMgr.ifMgr.ConvertIntfStrListToIfIndexList([]string{intfRef})
	if err != nil {
		pMgr.logger.Err("Invalid intfref argument in get buffer state ")
		return nil, errors.New("Invalid intf ref in buffer state get request")
	}
	ifIndex := ifIndexList[0]
	port := pMgr.ifIndexToPortNumXref[ifIndex]
	for _, plugin := range pMgr.plugins {
		if isControllingPlugin(plugin) == true {
			rv := plugin.UpdateBufferPortStateDB(port, int32(int(port)+1))
			if rv < 0 {
				return nil, errors.New("Failed to retrieve buffer state info from plugin")
			}
		}
	}
	pMgr.dbMutex.RLock()
	// ADD  buffer state values
	bState := pMgr.bufferPortStateDB[ifIndex]
	pMgr.dbMutex.RUnlock()
	return bState, nil
}

func (pMgr *PortManager) GetActiveIntfCountFromIfIndexList(ifIndexList []int32) int {
	var activeIntfCount int = 0
	pMgr.dbMutex.RLock()
	for _, ifIndex := range ifIndexList {
		if len(pMgr.portStateDB) <= int(ifIndex) {
			pMgr.logger.Err("portStateDB does not have entry for ifIndex")
			continue
		}
		if pMgr.portStateDB[ifIndex].OperState == "UP" {
			activeIntfCount += 1
		}
	}
	pMgr.dbMutex.RUnlock()
	return activeIntfCount
}

func NotifyLinkStateChange(ifIndex int32, operState uint8) {
	msg := pluginCommon.L2IntfStateNotifyMsg{
		IfIndex: ifIndex,
		IfState: operState,
	}
	msgBuf, err := json.Marshal(msg)
	if err != nil {
		PortMgr.logger.Err("Error in marshalling Json, in opennsl NotifyLinkStateChange")
	}
	notification := pluginCommon.AsicdNotification{
		MsgType: uint8(pluginCommon.NOTIFY_L2INTF_STATE_CHANGE),
		Msg:     msgBuf,
	}
	notificationBuf, err := json.Marshal(notification)
	if err != nil {
		PortMgr.logger.Err("Failed to marshal vlan create message")
	}
	PortMgr.notifyChannel <- notificationBuf
}

func linkStateChngHandler(ifIndex, portNum, speed int32, duplexType, operState string) {
	var oldOperState string
	var state int
	if operState == "UP" {
		state = pluginCommon.INTF_STATE_UP
	} else {
		state = pluginCommon.INTF_STATE_DOWN
	}
	PortMgr.portUpdateMutex.Lock()
	//Notify protocol daemons of link state change
	NotifyLinkStateChange(ifIndex, uint8(state))
	PortMgr.portUpdateMutex.Unlock()
	PortMgr.logger.Info("Sent notification for port link state change - ", ifIndex, operState, speed)
	PortMgr.dbMutex.Lock()
	PortMgr.portConfigDB[ifIndex].Duplex = duplexType
	PortMgr.portConfigDB[ifIndex].Speed = speed
	oldOperState = PortMgr.portStateDB[ifIndex].OperState
	PortMgr.portStateDB[ifIndex].OperState = operState
	switch operState {
	case pluginCommon.UpDownState[pluginCommon.INTF_STATE_UP]:
		if oldOperState == pluginCommon.UpDownState[pluginCommon.INTF_STATE_DOWN] {
			PortMgr.portStateDB[ifIndex].NumUpEvents += 1
			PortMgr.portStateDB[ifIndex].LastUpEventTime = time.Now().String()
		}
		evtKey := events.PortKey{
			IntfRef: PortMgr.portConfigDB[ifIndex].IntfRef,
		}
		txEvent := eventUtils.TxEvent{
			EventId:        events.PortOperStateUp,
			Key:            evtKey,
			AdditionalInfo: "",
			AdditionalData: nil,
		}
		err := eventUtils.PublishEvents(&txEvent)
		if err != nil {
			PortMgr.logger.Err("Error publish new events for port up")
			break
		}
	case pluginCommon.UpDownState[pluginCommon.INTF_STATE_DOWN]:
		if oldOperState == pluginCommon.UpDownState[pluginCommon.INTF_STATE_UP] {
			PortMgr.portStateDB[ifIndex].NumDownEvents += 1
			PortMgr.portStateDB[ifIndex].LastDownEventTime = time.Now().String()
		}
		evtKey := events.PortKey{
			IntfRef: PortMgr.portConfigDB[ifIndex].IntfRef,
		}
		txEvent := eventUtils.TxEvent{
			EventId:        events.PortOperStateDown,
			Key:            evtKey,
			AdditionalInfo: "",
			AdditionalData: nil,
		}
		err := eventUtils.PublishEvents(&txEvent)
		if err != nil {
			PortMgr.logger.Err("Error publish new events for port up")
			break
		}
	}
	PortMgr.dbMutex.Unlock()
	PortMgr.vlanMgr.VlanProcessIntfStateChange(ifIndex, state)
	PortMgr.lagMgr.LagProcessPortStateChange(ifIndex, state)
	PortMgr.l3IntfMgr.L3IntfProcessL2IntfStateChange(ifIndex, state)
}

func ProcessLinkStateChange(portNum, speed int32, duplexType, operState string) {
	/*
	 * Accept link state change updates only after init is complete. Prior to init
	 * completion, portConfigDB contains nil pointers
	 */
	if !PortMgr.initComplete {
		return
	}
	ifIndex, err := PortMgr.GetIfIndexFromPortNum(portNum)
	if err == nil {
		go linkStateChngHandler(ifIndex, portNum, speed, duplexType, operState)
	}
}

func (pMgr *PortManager) PortProtocolEnable(port, protocol int32, ena bool) error {
	if (int(port) > pMgr.maxPorts) || (port < 0) {
		pMgr.logger.Err("Invalid port argument in port protocol enable")
		return errors.New("Invalid port number in get request")
	}
	for _, plugin := range pMgr.plugins {
		if isControllingPlugin(plugin) == true {
			rv := plugin.UpdatePortStateDB(port, port+1)
			if rv < 0 {
				return errors.New("Failed to retrieve port state info from plugin")
			}
		}
	}
	return nil
}

func (pMgr *PortManager) ConvertBreakOutModeUsrStringToEnum(mode string) int32 {
	var newMode int32
	boMode := strings.TrimSpace(mode)
	boMode = strings.ToLower(boMode)
	switch boMode {
	case "1x40":
		newMode = pluginCommon.PORT_BREAKOUT_MODE_1x40
	case "4x10":
		newMode = pluginCommon.PORT_BREAKOUT_MODE_4x10
	case "1x100":
		newMode = pluginCommon.PORT_BREAKOUT_MODE_1x100
	case pluginCommon.PORT_BREAKOUT_MODE_UNSUPPORTED_STRING:
		//No-op, ignore during playback from DB
		newMode = 0
	default:
		pMgr.logger.Err("Invalid user port breakout mode string provided")
		newMode = 0
	}
	return newMode
}

func (pMgr *PortManager) ConvertBreakOutModeEnumToString(mode int32) string {
	var newMode string
	switch mode {
	case pluginCommon.PORT_BREAKOUT_MODE_UNSUPPORTED:
		newMode = pluginCommon.PORT_BREAKOUT_MODE_UNSUPPORTED_STRING
	case pluginCommon.PORT_BREAKOUT_MODE_1x40:
		newMode = "1x40"
	case pluginCommon.PORT_BREAKOUT_MODE_4x10:
		newMode = "4x10"
	case pluginCommon.PORT_BREAKOUT_MODE_1x100:
		newMode = "1x100"
	default:
		pMgr.logger.Err("Invalid user port breakout enum value")
		newMode = pluginCommon.PORT_BREAKOUT_MODE_UNSUPPORTED_STRING
	}
	return newMode
}

func (pMgr *PortManager) clearStatePerPort(port int32) {
	for _, plugin := range pMgr.plugins {
		if isControllingPlugin(plugin) == true {
			_ = plugin.ClearPortStat(port)
		}
	}
}

func (pMgr *PortManager) ClearStats(intfRef string) (bool, error) {
	switch intfRef {
	case "All":
		// iterate over entire port list and clear all the ports counter
		for _, portNum := range pMgr.ifIndexToPortNumXref {
			pMgr.clearStatePerPort(portNum)
		}
	default:
		// meaning a specific port needs to be delete
		ifIndexList, err := pMgr.ifMgr.ConvertIntfStrListToIfIndexList([]string{intfRef})
		if err != nil {
			pMgr.logger.Err("Invalid intfref argument in get port state ")
			return false, errors.New("Invalid intf ref in port state get request")
		}
		ifIndex := ifIndexList[0]
		port := pMgr.ifIndexToPortNumXref[ifIndex]
		pMgr.clearStatePerPort(port)
	}

	return true, nil
}
