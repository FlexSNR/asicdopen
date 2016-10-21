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
	"errors"
	"utils/logging"
)

type StpManager struct {
	portMgr    *PortManager
	stgVlanMap map[int32][]int32
	logger     *logging.Writer
	plugins    []PluginIntf
}

var StpMgr StpManager

func (sMgr *StpManager) Init(rsrcMgrs *ResourceManagers, pluginList []PluginIntf, logger *logging.Writer) {
	sMgr.portMgr = rsrcMgrs.PortManager
	sMgr.logger = logger
	sMgr.plugins = pluginList
	sMgr.stgVlanMap = make(map[int32][]int32, 0)
}

func (sMgr *StpManager) Deinit() {
}

func (sMgr *StpManager) CreateStg(vlanList []int32) (stgId int32, err error) {
	for _, plugin := range sMgr.plugins {
		if isPluginAsicDriver(plugin) == true {
			rv := plugin.CreateStg(vlanList)
			if rv < 0 {
				return -1, errors.New("Failed to create STG")
			}
			stgId = int32(rv)
			break
		}
	}
	sMgr.stgVlanMap[stgId] = vlanList
	return stgId, nil
}

func (sMgr *StpManager) DeleteStg(stgId int32) (bool, error) {
	for _, plugin := range sMgr.plugins {
		if isPluginAsicDriver(plugin) == true {
			rv := plugin.DeleteStg(stgId)
			if rv < 0 {
				return false, errors.New("Failed to delete STG")
			}
			break
		}
	}
	sMgr.FlushFdbStgGroup(stgId, -1)
	delete(sMgr.stgVlanMap, stgId)
	return true, nil
}

func (sMgr *StpManager) SetPortStpState(stgId, portIfIndex, stpState int32) (bool, error) {
	port := sMgr.portMgr.GetPortNumFromIfIndex(portIfIndex)
	for _, plugin := range sMgr.plugins {
		if isPluginAsicDriver(plugin) == true {
			rv := plugin.SetPortStpState(stgId, port, stpState)
			if rv < 0 {
				return false, errors.New("Failed to set STP port state")
			}
			break
		}
	}
	return true, nil
}

func (sMgr *StpManager) GetPortStpState(stgId, portIfIndex int32) (stpState int32, err error) {
	port := sMgr.portMgr.GetPortNumFromIfIndex(portIfIndex)
	for _, plugin := range sMgr.plugins {
		if isPluginAsicDriver(plugin) == true {
			rv := plugin.GetPortStpState(stgId, port)
			if rv < 0 {
				return -1, errors.New("Failed to retrieve STP port state")
			}
			stpState = int32(rv)
			break
		}
	}
	return stpState, nil
}

func (sMgr *StpManager) UpdateStgVlanList(stgId int32, vlanList []int32) (bool, error) {
	for _, plugin := range sMgr.plugins {
		if isPluginAsicDriver(plugin) == true {
			rv := plugin.UpdateStgVlanList(stgId, nil, vlanList)
			if rv < 0 {
				return false, errors.New("Failed to update STG vlan list")
			}
			break
		}
	}
	sMgr.stgVlanMap[stgId] = vlanList
	return true, nil
}

func (sMgr *StpManager) FlushFdbStgGroup(stgId, portIfIndex int32) error {
	port := sMgr.portMgr.GetPortNumFromIfIndex(portIfIndex)
	for _, plugin := range sMgr.plugins {
		if isPluginAsicDriver(plugin) == true {
			plugin.FlushFdbStgGroup(sMgr.stgVlanMap[stgId], port)
			return nil
		}
	}
	return nil
}
