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

/*
 * VR: Temporarily commenting out MPLS code in asicd
 *
import (
	"asicd/pluginManager/pluginCommon"
	"asicdServices"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"utils/dbutils"
	"utils/logging"
)

type MplsIntfObj struct {
	IntfRef           string
	IfIndex           int32
	OperState         string
	LastUpEventTime   string
	NumUpEvents       int32
	LastDownEventTime string
	NumDownEvents     int32
}

type MplsIntfManager struct {
	dbMutex       sync.RWMutex
	intfMgr       *IfManager
	mplsIntfDB    []*MplsIntfObj
	plugins       []PluginIntf
	IfIdMap       map[int]int
	logger        *logging.Writer
	notifyChannel chan<- []byte
}

// Mpls intf db
var MplsIntfMgr MplsIntfManager

func (iMgr *MplsIntfManager) Init(rsrcMgrs *ResourceManagers, dbHdl *dbutils.DBUtil, pluginList []PluginIntf, logger *logging.Writer, notifyChannel chan<- []byte) {
	iMgr.plugins = pluginList
	iMgr.logger = logger
	iMgr.notifyChannel = notifyChannel
	iMgr.intfMgr = rsrcMgrs.IfManager
}

func (iMgr *MplsIntfManager) Deinit() {
	for _, plugin := range iMgr.plugins {
		if isPluginAsicDriver(plugin) {
			continue
		}
		iMgr.dbMutex.RLock()
		for _, info := range iMgr.mplsIntfDB {
			if info != nil {
				//		ifName, _ := MplsIntfMgr.intfMgr.GetIfNameForIfIndex(info.IfIndex)
				//		rv := plugin.DeleteMplsIntf(ifName)
				//		if rv < 0 {
				//			iMgr.logger.Err("Failed to cleanup mpls interfaces")
				//			continue
				//		}
			}
		}
		iMgr.dbMutex.RUnlock()
	}
}

func SendMplsIntfStateNotificationMsg(ifIndex int32, state int) {
	msg := pluginCommon.MplsIntfStateNotifyMsg{
		IfIndex: ifIndex,
		IfState: uint8(state),
	}
	msgBuf, err := json.Marshal(msg)
	if err != nil {
		MplsIntfMgr.logger.Err("Failed to marshal MplsIntf state change message")
	}
	notification := pluginCommon.AsicdNotification{
		MsgType: uint8(pluginCommon.NOTIFY_MPLSINTF_STATE_CHANGE),
		Msg:     msgBuf,
	}
	notificationBuf, err := json.Marshal(notification)
	if err != nil {
		L3IntfMgr.logger.Err("Failed to marshal MplsIntf state change message")
	}
	MplsIntfMgr.notifyChannel <- notificationBuf
	MplsIntfMgr.logger.Info(fmt.Sprintln("Sent Mpls state change notification - ", ifIndex, state))
}

//Send out notification regarding Mpls intf creation - LMd listens to this
func SendMplsCreateDelNotification(ifIndex int32, notify_type int) {
	msg := pluginCommon.MplsIntfNotifyMsg{
		IfIndex: ifIndex,
	}
	msgBuf, err := json.Marshal(msg)
	if err != nil {
		MplsIntfMgr.logger.Err("Failed to marshal MplsIntf create message")
	}
	notification := pluginCommon.AsicdNotification{
		uint8(notify_type),
		msgBuf,
	}
	notificationBuf, err := json.Marshal(notification)
	if err != nil {
		L3IntfMgr.logger.Err("Failed to marshal MplsIntf create message")
	}
	MplsIntfMgr.notifyChannel <- notificationBuf
}

func DetermineL3IntfOperState(ifIndex int32) int {
	state := pluginCommon.INTF_STATE_UP
	return state
}

//Send out Mpls interface state up/down notification
func SendMplsIntfStateNotification(ifIndex int32) {
	state := DetermineL3IntfOperState(ifIndex)
	SendMplsIntfStateNotificationMsg(ifIndex, state)
}

//Check for erroneous configuration in mpls intf db first
func VerifyMplsConfig(IfIndex int32) error {
	return nil
}

func (iMgr *MplsIntfManager) CreateMplsIntf(mplsIntfObj *asicdServices.MplsIntf) (bool, error) {
	var ifIndex int32
	//Convert intf ref to ifindex
	ifIndexList, err := iMgr.intfMgr.ConvertIntfStrListToIfIndexList([]string{mplsIntfObj.IntfRef})
	if err != nil {
		return false, errors.New("Invalid interface reference provided in mpls intf config object")
	}
	ifIndex = ifIndexList[0]
	// Verify mpls config
	iMgr.dbMutex.Lock()
	err = VerifyMplsConfig(ifIndex)
	iMgr.dbMutex.Unlock()
	if err != nil {
		return false, err
	}
	// Create mpls interface and setup neighbor
	//_, err = CreateIPv4IntfInternal(ifIndex, parsedIP, maskLen)
	if err != nil {
		return false, err
	}
	//Update Sw State
	mplsIntf := MplsIntfObj{
		IntfRef: mplsIntfObj.IntfRef,
		IfIndex: ifIndex,
	}
	if DetermineL3IntfOperState(ifIndex) == pluginCommon.INTF_STATE_UP {
		mplsIntf.OperState = "UP"
	} else {
		mplsIntf.OperState = "DOWN"
	}
	iMgr.dbMutex.Lock()
	iMgr.mplsIntfDB = append(iMgr.mplsIntfDB, &mplsIntf)
	iMgr.dbMutex.Unlock()
	// Send mpls interface created notification
	SendMplsCreateDelNotification(ifIndex, pluginCommon.NOTIFY_MPLSINTF_CREATE)
	// send mpls state notifcation
	SendMplsIntfStateNotification(ifIndex)
	return true, nil
}

func (iMgr *MplsIntfManager) DeleteMplsIntf(mplsIntfObj *asicdServices.MplsIntf) (bool, error) {
	//Convert intf ref to ifindex
	ifIndexList, err := iMgr.intfMgr.ConvertIntfStrListToIfIndexList([]string{mplsIntfObj.IntfRef})
	if err != nil {
		return false, errors.New("Invalid interface reference provided in mpls intf config object")
	}
	ifIndex := ifIndexList[0]
	//Update Sw State
	iMgr.dbMutex.Lock()
	//Locate record
	for idx, elem := range iMgr.mplsIntfDB {
		if elem.IfIndex == ifIndex {
			//Set element to nil for gc
			iMgr.mplsIntfDB[idx] = nil
			iMgr.mplsIntfDB = append(iMgr.mplsIntfDB[:idx], iMgr.mplsIntfDB[idx+1:]...)
			break
		}
	}
	iMgr.dbMutex.Unlock()
	//Send out state notification
	SendMplsIntfStateNotification(ifIndex)
	//Send out notification regarding mpls intf deletion - LMd listens to this
	SendMplsCreateDelNotification(ifIndex, pluginCommon.NOTIFY_MPLSINTF_DELETE)
	return true, nil
}

func (iMgr *MplsIntfManager) GetMplsIntfState(intfRef string) (*asicdServices.MplsIntfState, error) {
	if iMgr.mplsIntfDB == nil {
		MplsIntfMgr.logger.Err(fmt.Sprintln("mplsIntfDB nil"))
		return nil, errors.New("Nil DB")
	}
	//Convert intf ref to ifindex
	ifIndexList, err := iMgr.intfMgr.ConvertIntfStrListToIfIndexList([]string{intfRef})
	if err != nil {
		return nil, errors.New("Invalid interface reference provided during get mpls intf state")
	}
	ifIndex := ifIndexList[0]
	iMgr.dbMutex.RLock()
	for i := 0; i < len(iMgr.mplsIntfDB); i++ {
		mplsIntf := iMgr.mplsIntfDB[i]
		if mplsIntf.IfIndex == ifIndex {
			iMgr.dbMutex.RUnlock()
			return &asicdServices.MplsIntfState{
				IntfRef:           mplsIntf.IntfRef,
				IfIndex:           mplsIntf.IfIndex,
				OperState:         mplsIntf.OperState,
				NumUpEvents:       mplsIntf.NumUpEvents,
				LastUpEventTime:   mplsIntf.LastUpEventTime,
				NumDownEvents:     mplsIntf.NumDownEvents,
				LastDownEventTime: mplsIntf.LastDownEventTime,
			}, nil
		}
	}
	iMgr.dbMutex.RUnlock()
	return nil, errors.New(fmt.Sprintln("Unable to locate ipv4 interface corresponding to intf - ", intfRef))
}

func (iMgr *MplsIntfManager) GetBulkMplsIntfState(start, count int) (end, listLen int, more bool, mplsIntfList []*asicdServices.MplsIntfState) {
	if start < 0 {
		iMgr.logger.Err("Invalid start index argument in get bulk mplsintf")
		return -1, 0, false, nil
	}
	if count < 0 {
		iMgr.logger.Err("Invalid count in get bulk mplsintf")
		return -1, 0, false, nil
	}
	if (start + count) < len(iMgr.mplsIntfDB) {
		more = true
		end = start + count
	} else {
		more = false
		end = len(iMgr.mplsIntfDB)
	}
	iMgr.dbMutex.RLock()
	for idx := start; idx < len(iMgr.mplsIntfDB); idx++ {
		mplsIntfList = append(mplsIntfList, &asicdServices.MplsIntfState{
			IntfRef:           iMgr.mplsIntfDB[idx].IntfRef,
			IfIndex:           iMgr.mplsIntfDB[idx].IfIndex,
			OperState:         iMgr.mplsIntfDB[idx].OperState,
			NumUpEvents:       iMgr.mplsIntfDB[idx].NumUpEvents,
			LastUpEventTime:   iMgr.mplsIntfDB[idx].LastUpEventTime,
			NumDownEvents:     iMgr.mplsIntfDB[idx].NumDownEvents,
			LastDownEventTime: iMgr.mplsIntfDB[idx].LastDownEventTime,
		})
	}
	iMgr.dbMutex.RUnlock()
	return end, len(mplsIntfList), more, mplsIntfList
}
*/
