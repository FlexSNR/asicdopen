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
	"asicdServices"
	"utils/logging"
)

type InfraManager struct {
	logger          *logging.Writer
	plugins         []PluginIntf
	portMgr         *PortManager
	vlanMgr         *VlanManager
	l3IntfMgr       *L3IntfManager
	neighborMgr     *NeighborManager
	routeMgr        *RouteManager
	asicGlobalState *asicdServices.AsicGlobalState
}

var InfraMgr InfraManager

func (iMgr *InfraManager) Init(rsrcMgrs *ResourceManagers, plugins []PluginIntf, logger *logging.Writer) {
	iMgr.logger = logger
	iMgr.plugins = plugins
	iMgr.portMgr = rsrcMgrs.PortManager
	iMgr.vlanMgr = rsrcMgrs.VlanManager
	iMgr.l3IntfMgr = rsrcMgrs.L3IntfManager
	iMgr.neighborMgr = rsrcMgrs.NeighborManager
	iMgr.routeMgr = rsrcMgrs.RouteManager
	iMgr.asicGlobalState = new(asicdServices.AsicGlobalState)
	//Read and populate state
	iMgr.PopulateAsicGlobalState()
}

func (iMgr *InfraManager) Deinit() {
	//No op
}

func (iMgr *InfraManager) PopulateAsicGlobalState() {
	for _, plugin := range iMgr.plugins {
		if isPluginAsicDriver(plugin) {
			temp := plugin.GetModuleTemperature()
			iMgr.asicGlobalState.ModuleTemp = temp

			invInfo := plugin.GetModuleInventory()
			if invInfo != nil {
				iMgr.asicGlobalState.VendorId = invInfo.VendorId
				iMgr.asicGlobalState.PartNumber = invInfo.PartNumber
				iMgr.asicGlobalState.RevisionId = invInfo.RevisionId
			}
		}
	}
}

func (iMgr *InfraManager) GetAsicGlobalState(moduleId uint8) (*asicdServices.AsicGlobalState, error) {
	//Update state prior to returning
	for _, plugin := range iMgr.plugins {
		if isPluginAsicDriver(plugin) {
			iMgr.asicGlobalState.ModuleTemp = plugin.GetModuleTemperature()
		}
	}
	return iMgr.asicGlobalState, nil
}

func (iMgr *InfraManager) GetBulkAsicGlobalState(currMarker, count int) (start, end, cnt int, more bool, list []*asicdServices.AsicGlobalState) {
	start = 0
	end = 0
	cnt = 1
	more = false
	gblState, _ := iMgr.GetAsicGlobalState(uint8(0))
	list = []*asicdServices.AsicGlobalState{gblState}
	return
}

func (iMgr *InfraManager) GetAsicSummaryState(moduleId uint8) (*asicdServices.AsicSummaryState, error) {
	asicSummaryState := new(asicdServices.AsicSummaryState)
	asicSummaryState.NumPortsUp = int32(iMgr.portMgr.GetNumOperStateUPPorts())
	asicSummaryState.NumPortsDown = int32(iMgr.portMgr.GetNumOperStateDOWNPorts())
	asicSummaryState.NumVlans = int32(iMgr.vlanMgr.GetNumVlans())
	asicSummaryState.NumV4Intfs = int32(iMgr.l3IntfMgr.GetNumV4Intfs())
	asicSummaryState.NumV6Intfs = int32(iMgr.l3IntfMgr.GetNumV6Intfs())
	asicSummaryState.NumV4Adjs = int32(iMgr.neighborMgr.GetNumV4Adjs())
	asicSummaryState.NumV6Adjs = int32(iMgr.neighborMgr.GetNumV6Adjs())
	asicSummaryState.NumV4Routes = int32(iMgr.routeMgr.GetNumV4Routes())
	asicSummaryState.NumV6Routes = int32(iMgr.routeMgr.GetNumV6Routes())
	asicSummaryState.NumECMPRoutes = int32(iMgr.routeMgr.GetNumECMPRoutes())
	return asicSummaryState, nil
}

func (iMgr *InfraManager) GetBulkAsicSummaryState(currMarker, count int) (start, end, cnt int, more bool, list []*asicdServices.AsicSummaryState) {
	start = 0
	end = 0
	cnt = 1
	more = false
	gblState, _ := iMgr.GetAsicSummaryState(uint8(0))
	list = []*asicdServices.AsicSummaryState{gblState}
	return
}
