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
	"asicdInt"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"utils/commonDefs"
	"utils/logging"
)

const (
	INITIAL_LAG_MAP_SIZE = 1024
)

type lagInfo struct {
	lagName         string
	hashType        int
	portIfIndexList []int32
	activeIntfCount int
	ifIndex         int32
}

type LagManager struct {
	dbMutex       sync.RWMutex
	ifMgr         *IfManager
	portMgr       *PortManager
	vlanMgr       *VlanManager
	l3IntfMgr     *L3IntfManager
	lagDB         map[int32]lagInfo
	lagDbKeys     []int32
	plugins       []PluginIntf
	logger        *logging.Writer
	notifyChannel chan<- []byte
}

//Lag manager db
var LagMgr LagManager

func UpdateLagDB(lagId, hashType int, portList []int32) {
	var ifIndexList []int32
	for _, port := range portList {
		ifIndex, err := LagMgr.portMgr.GetIfIndexFromPortNum(port)
		if err != nil {
			ifIndexList = append(ifIndexList, ifIndex)
		}
	}
	activePortCount := LagMgr.portMgr.GetActiveIntfCountFromIfIndexList(ifIndexList)
	lagName := pluginCommon.LAG_PREFIX + strconv.Itoa(lagId)
	lagIfIndex := LagMgr.ifMgr.AllocateIfIndex(lagName, lagId, commonDefs.IfTypeLag)
	LagMgr.dbMutex.Lock()
	LagMgr.lagDB[lagIfIndex] = lagInfo{
		ifIndex:         lagIfIndex,
		lagName:         lagName,
		hashType:        hashType,
		portIfIndexList: ifIndexList,
		activeIntfCount: activePortCount,
	}
	LagMgr.dbMutex.Unlock()
}

func (lMgr *LagManager) Init(rsrcMgrs *ResourceManagers, bootMode int, pluginList []PluginIntf, logger *logging.Writer, notifyChannel chan<- []byte) {
	lMgr.ifMgr = rsrcMgrs.IfManager
	lMgr.portMgr = rsrcMgrs.PortManager
	lMgr.vlanMgr = rsrcMgrs.VlanManager
	lMgr.l3IntfMgr = rsrcMgrs.L3IntfManager
	lMgr.logger = logger
	lMgr.plugins = pluginList
	lMgr.notifyChannel = notifyChannel
	lMgr.lagDB = make(map[int32]lagInfo, INITIAL_LAG_MAP_SIZE)
	lMgr.lagDbKeys = make([]int32, 0, INITIAL_LAG_MAP_SIZE)

	if bootMode == pluginCommon.BOOT_MODE_WARMBOOT {
		//Restore lag db during warm boot
		for _, plugin := range lMgr.plugins {
			if isPluginAsicDriver(plugin) == true {
				rv := plugin.RestoreLagDB()
				if rv < 0 {
					lMgr.logger.Err("Failed to restore lag db during warm boot")
				}
				break
			}
		}
	}
}

func (lMgr *LagManager) Deinit() {
	var plugin PluginIntf
	/* Cleanup all Bonded intf's created by linux plugin */
	for _, plugin = range lMgr.plugins {
		if isPluginAsicDriver(plugin) == false {
			break
		}
	}
	lMgr.dbMutex.RLock()
	for _, lagIfIndex := range lMgr.lagDbKeys {
		lagId := pluginCommon.GetIdFromIfIndex(lagIfIndex)
		pluginObj := &pluginCommon.PluginLagInfo{
			IfName: lMgr.lagDB[lagIfIndex].lagName,
			HwId:   &lagId,
		}
		rv := plugin.DeleteLag(pluginObj)
		if rv < 0 {
			lMgr.logger.Err("Failed to cleanup lag interfaces")
		}
	}
	lMgr.dbMutex.RUnlock()
}

func (lMgr *LagManager) validateLagCfg(ifIndexList []int32) (bool, error) {
	var ok bool = true
	var err error
	for _, ifIndex := range ifIndexList {
		if lMgr.portMgr.GetPortConfigMode(ifIndex) == pluginCommon.PORT_MODE_L3 ||
			lMgr.portMgr.GetPortConfigMode(ifIndex) == pluginCommon.PORT_MODE_INTERNAL {
			ok = false
			err = errors.New("Please remove all L3 configuration prior to creating LAG")
		}
	}
	return ok, err
}

func (lMgr *LagManager) CreateLag(ifName string, hashType int, ifIndexListStr string) (int32, error) {
	var lagId int
	var portList, ifIndexList []int32
	portIfIndexStrList, err := ParseUsrStrToList(ifIndexListStr)
	if err != nil {
		lMgr.logger.Err("Failed to parse port list during UpdateLag")
		return -1, err
	}
	for _, ifIndexStr := range portIfIndexStrList {
		ifIndex, err := strconv.Atoi(ifIndexStr)
		if err != nil {
			return -1, err
		}
		ifIndexList = append(ifIndexList, int32(ifIndex))
		port := lMgr.portMgr.GetPortNumFromIfIndex(int32(ifIndex))
		portList = append(portList, port)
	}
	ok, err := lMgr.validateLagCfg(ifIndexList)
	if !ok {
		return -1, err
	}
	lagId = -1
	// lagId is generated by the asic sdk
	// softswitch plugin will use this id as a base
	for _, plugin := range lMgr.plugins {
		var list []int32
		if isControllingPlugin(plugin) {
			list = portList
		} else {
			list = ifIndexList
		}
		pluginObj := &pluginCommon.PluginLagInfo{
			IfName:     ifName,
			HwId:       &lagId,
			HashType:   hashType,
			MemberList: list,
		}
		rv := plugin.CreateLag(pluginObj)
		if rv < 0 {
			lMgr.logger.Err("Failed to create lag group")
			return -1, errors.New("Failed to create lag group")
		}
	}
	lMgr.portMgr.SetPortConfigMode(pluginCommon.PORT_MODE_L2, ifIndexList)
	activePortCount := lMgr.portMgr.GetActiveIntfCountFromIfIndexList(ifIndexList)
	lagIfIndex := lMgr.ifMgr.AllocateIfIndex(ifName, lagId, commonDefs.IfTypeLag)
	lMgr.dbMutex.Lock()
	lMgr.lagDB[lagIfIndex] = lagInfo{
		lagName:         ifName,
		ifIndex:         lagIfIndex,
		hashType:        hashType,
		portIfIndexList: ifIndexList,
		activeIntfCount: activePortCount,
	}
	lMgr.dbMutex.Unlock()
	//Cache DB key to aid with getbulk
	lMgr.lagDbKeys = append(lMgr.lagDbKeys, lagIfIndex)
	//Publish notification - ARPd listens to lag create notification
	msg := pluginCommon.LagNotifyMsg{
		IfIndex:     lagIfIndex,
		IfIndexList: ifIndexList,
	}
	msgBuf, err := json.Marshal(msg)
	if err != nil {
		lMgr.logger.Err("Failed to marshal lag create message")
	}
	notification := pluginCommon.AsicdNotification{
		MsgType: uint8(pluginCommon.NOTIFY_LAG_CREATE),
		Msg:     msgBuf,
	}
	notificationBuf, err := json.Marshal(notification)
	if err != nil {
		lMgr.logger.Err("Failed to marshal lag create message")
	}
	lMgr.notifyChannel <- notificationBuf
	return lagIfIndex, nil
}

func (lMgr *LagManager) DeleteLag(ifIndex int32) int {
	lagId := pluginCommon.GetIdFromIfIndex(ifIndex)
	lMgr.dbMutex.RLock()
	if _, ok := lMgr.lagDB[ifIndex]; !ok {
		lMgr.logger.Err("Invalid lag id provided for delete operation")
		lMgr.dbMutex.RUnlock()
		return -1
	}
	lMgr.dbMutex.RUnlock()
	pluginObj := &pluginCommon.PluginLagInfo{
		IfName: lMgr.lagDB[ifIndex].lagName,
		HwId:   &lagId,
	}
	for _, plugin := range lMgr.plugins {
		rv := plugin.DeleteLag(pluginObj)
		if rv < 0 {
			lMgr.logger.Err("Failed to delete lag group")
			return -1
		}
	}
	lMgr.dbMutex.Lock()
	portIfIndexList := lMgr.lagDB[ifIndex].portIfIndexList
	delete(lMgr.lagDB, ifIndex)
	lMgr.dbMutex.Unlock()
	//Update cache of DB keys used for getbulk
	for idx := 0; idx < len(lMgr.lagDbKeys); idx++ {
		if lMgr.lagDbKeys[idx] == ifIndex {
			lMgr.lagDbKeys = append(lMgr.lagDbKeys[0:idx], lMgr.lagDbKeys[idx+1:]...)
			break
		}
	}
	lMgr.ifMgr.FreeIfIndex(ifIndex)
	lMgr.portMgr.SetPortConfigMode(pluginCommon.PORT_MODE_UNCONFIGURED, portIfIndexList)
	//Publish notification - ARPd listens to lag delete notification
	msg := pluginCommon.LagNotifyMsg{
		IfIndex:     ifIndex,
		IfIndexList: portIfIndexList,
	}
	msgBuf, err := json.Marshal(msg)
	if err != nil {
		lMgr.logger.Err("Failed to marshal lag create message")
	}
	notification := pluginCommon.AsicdNotification{
		MsgType: uint8(pluginCommon.NOTIFY_LAG_DELETE),
		Msg:     msgBuf,
	}
	notificationBuf, err := json.Marshal(notification)
	if err != nil {
		lMgr.logger.Err("Failed to marshal lag create message")
	}
	lMgr.notifyChannel <- notificationBuf
	return 0
}

func (lMgr *LagManager) UpdateLag(ifIndex int32, hashType int, ifIndexListStr string) (int, error) {
	var portList, oldPortList, ifIndexList []int32
	lagId := pluginCommon.GetIdFromIfIndex(ifIndex)
	portIfIndexStrList, err := ParseUsrStrToList(ifIndexListStr)
	if err != nil {
		lMgr.logger.Err("Failed to parse port list during UpdateLag")
		return -1, err
	}
	for _, ifIndexStr := range portIfIndexStrList {
		ifIndex, err := strconv.Atoi(ifIndexStr)
		if err != nil {
			return -1, err
		}
		ifIndexList = append(ifIndexList, int32(ifIndex))
		port := lMgr.portMgr.GetPortNumFromIfIndex(int32(ifIndex))
		portList = append(portList, port)
	}
	ok, err := lMgr.validateLagCfg(ifIndexList)
	if !ok {
		return -1, err
	}
	lMgr.dbMutex.RLock()
	if _, ok := lMgr.lagDB[ifIndex]; !ok {
		lMgr.logger.Err("Invalid lag group specified for update operation")
		lMgr.dbMutex.RUnlock()
		return -1, errors.New("Invalid lag group specified for update operation")
	}
	oldPortIfIndexList := lMgr.lagDB[ifIndex].portIfIndexList
	lMgr.dbMutex.RUnlock()
	if !(len(portList) > 0) {
		lMgr.logger.Err("Invalid port arguments during lag update")
	}
	for _, val := range oldPortIfIndexList {
		port := lMgr.portMgr.GetPortNumFromIfIndex(val)
		oldPortList = append(oldPortList, port)
	}
	for _, plugin := range lMgr.plugins {
		//Asic plugin needs port list, while linux plugin needs ifIndex list
		var oldList, newList []int32
		if isControllingPlugin(plugin) {
			oldList = oldPortList
			newList = portList
		} else {
			oldList = oldPortIfIndexList
			newList = ifIndexList
		}
		pluginObj := &pluginCommon.PluginUpdateLagInfo{
			IfName:        lMgr.lagDB[ifIndex].lagName,
			HwId:          &lagId,
			HashType:      hashType,
			OldMemberList: oldList,
			MemberList:    newList,
		}
		rv := plugin.UpdateLag(pluginObj)
		if rv < 0 {
			lMgr.logger.Err("Failed to update lag group")
			return -1, errors.New("Failed to update lag group")
		}
	}
	activePortCount := lMgr.portMgr.GetActiveIntfCountFromIfIndexList(ifIndexList)
	lMgr.dbMutex.Lock()
	lMgr.lagDB[ifIndex] = lagInfo{
		lagName:         lMgr.lagDB[ifIndex].lagName,
		ifIndex:         ifIndex,
		hashType:        hashType,
		portIfIndexList: ifIndexList,
		activeIntfCount: activePortCount,
	}
	lMgr.dbMutex.Unlock()
	lMgr.portMgr.SetPortConfigMode(pluginCommon.PORT_MODE_L3, oldPortIfIndexList)
	lMgr.portMgr.SetPortConfigMode(pluginCommon.PORT_MODE_L2, ifIndexList)
	//Publish notification - ARPd listens to lag create notification
	msg := pluginCommon.LagNotifyMsg{
		IfIndex:     ifIndex,
		IfIndexList: ifIndexList,
	}
	msgBuf, err := json.Marshal(msg)
	if err != nil {
		lMgr.logger.Err("Failed to marshal lag create message")
	}
	notification := pluginCommon.AsicdNotification{
		MsgType: uint8(pluginCommon.NOTIFY_LAG_UPDATE),
		Msg:     msgBuf,
	}
	notificationBuf, err := json.Marshal(notification)
	if err != nil {
		lMgr.logger.Err("Failed to marshal lag create message")
	}
	lMgr.notifyChannel <- notificationBuf
	return 0, nil
}

func (lMgr *LagManager) LagProcessPortStateChange(ifIndex int32, state int) {
	lMgr.dbMutex.Lock()
	for lagIfIndex, info := range lMgr.lagDB {
		for _, p := range info.portIfIndexList {
			if p == ifIndex {
				if state == pluginCommon.INTF_STATE_DOWN {
					lMgr.lagDB[lagIfIndex] = lagInfo{
						lagName:         info.lagName,
						hashType:        info.hashType,
						portIfIndexList: info.portIfIndexList,
						activeIntfCount: info.activeIntfCount - 1,
					}
					if lMgr.lagDB[lagIfIndex].activeIntfCount == 0 {
						lMgr.vlanMgr.VlanProcessIntfStateChange(lagIfIndex, state)
						lMgr.l3IntfMgr.L3IntfProcessL2IntfStateChange(lagIfIndex, state)
					}
				} else {
					lMgr.lagDB[lagIfIndex] = lagInfo{
						lagName:         info.lagName,
						hashType:        info.hashType,
						portIfIndexList: info.portIfIndexList,
						activeIntfCount: info.activeIntfCount + 1,
					}
					if lMgr.lagDB[lagIfIndex].activeIntfCount == 1 {
						lMgr.vlanMgr.VlanProcessIntfStateChange(lagIfIndex, state)
						lMgr.l3IntfMgr.L3IntfProcessL2IntfStateChange(lagIfIndex, state)
					}
				}
				break
			}
		}
	}
	lMgr.dbMutex.Unlock()
}

func (lMgr *LagManager) GetBulkLag(start, count int) (end, listLen int, more bool, lagList []*asicdInt.Lag) {
	var numEntries, idx int = 0, 0
	if start < 0 {
		lMgr.logger.Err("Invalid start index argument in get bulk lag")
		return -1, 0, false, nil
	}
	if count < 0 {
		lMgr.logger.Err("Invalid count in get bulk lag")
		return -1, 0, false, nil
	}
	lMgr.dbMutex.RLock()
	for idx = start; idx < len(lMgr.lagDbKeys); idx++ {
		lagIfIndex := lMgr.lagDbKeys[idx]
		if numEntries == count {
			more = true
			break
		}
		lagList = append(lagList, &asicdInt.Lag{
			LagIfIndex:  lagIfIndex,
			HashType:    int32(lMgr.lagDB[lagIfIndex].hashType),
			IfIndexList: lMgr.lagDB[lagIfIndex].portIfIndexList,
		})
		numEntries += 1
	}
	end = idx
	if idx == len(lMgr.lagDbKeys) {
		more = false
	}
	listLen = len(lagList)
	lMgr.dbMutex.RUnlock()
	return end, listLen, more, lagList
}

func (lMgr *LagManager) LagGetActiveIntfCount(ifIndex int32) int {
	lMgr.dbMutex.RLock()
	defer lMgr.dbMutex.RUnlock()
	return (lMgr.lagDB[ifIndex].activeIntfCount)
}

func (lMgr *LagManager) GetActiveLagCountFromLagList(lagIfIndexList []int32) int {
	var activeLagCount int = 0
	lMgr.dbMutex.RLock()
	for _, lagIfIndex := range lagIfIndexList {
		if lMgr.lagDB[lagIfIndex].activeIntfCount > 0 {
			activeLagCount += 1
		}
	}
	lMgr.dbMutex.RUnlock()
	return activeLagCount
}

func (lMgr *LagManager) GetLagMembers(lagIfIndex int32) []int32 {
	lMgr.dbMutex.RLock()
	defer lMgr.dbMutex.RUnlock()
	return (lMgr.lagDB[lagIfIndex].portIfIndexList)
}
