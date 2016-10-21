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

// This file defines all interfaces provided for the AsicGlobal service

package rpc

import (
	"asicdServices"
	"errors"
	"fmt"
)

func (svcHdlr AsicDaemonServiceHandler) GetAsicGlobalState(moduleId int8) (*asicdServices.AsicGlobalState, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get asic global state obj- ", moduleId))
	return svcHdlr.pluginMgr.GetAsicGlobalState(uint8(moduleId))
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkAsicGlobalState(currMarker, count asicdServices.Int) (*asicdServices.AsicGlobalStateGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received for GetBulkAsicGlobal")
	start, end, cnt, more, list := svcHdlr.pluginMgr.GetBulkAsicGlobalState(int(currMarker), int(count))
	bulkObj := new(asicdServices.AsicGlobalStateGetInfo)
	bulkObj.StartIdx = asicdServices.Int(start)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.Count = asicdServices.Int(cnt)
	bulkObj.More = more
	bulkObj.AsicGlobalStateList = list
	return bulkObj, nil
}

func (svcHdlr AsicDaemonServiceHandler) GetAsicSummaryState(moduleId int8) (*asicdServices.AsicSummaryState, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get asic summary state obj- ", moduleId))
	return svcHdlr.pluginMgr.GetAsicSummaryState(uint8(moduleId))
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkAsicSummaryState(currMarker, count asicdServices.Int) (*asicdServices.AsicSummaryStateGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received for GetBulkAsicSummary")
	start, end, cnt, more, list := svcHdlr.pluginMgr.GetBulkAsicSummaryState(int(currMarker), int(count))
	bulkObj := new(asicdServices.AsicSummaryStateGetInfo)
	bulkObj.StartIdx = asicdServices.Int(start)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.Count = asicdServices.Int(cnt)
	bulkObj.More = more
	bulkObj.AsicSummaryStateList = list
	return bulkObj, nil
}

func (svcHdlr AsicDaemonServiceHandler) CreateAsicGlobalPM(obj *asicdServices.AsicGlobalPM) (bool, error) {
	return false, errors.New("Create not supported for AsicGlobalPM")
}

func (svcHdlr AsicDaemonServiceHandler) DeleteAsicGlobalPM(obj *asicdServices.AsicGlobalPM) (bool, error) {
	return false, errors.New("Delete not supported for AsicGlobalPM")
}

func (svcHdlr AsicDaemonServiceHandler) UpdateAsicGlobalPM(oldObj, newObj *asicdServices.AsicGlobalPM, attrset []bool, op []*asicdServices.PatchOpInfo) (bool, error) {
	var ok bool
	svcHdlr.logger.Debug("Thrift request received to update asic global cfg obj - ", oldObj, newObj, attrset)
	ok, err := svcHdlr.pluginMgr.UpdateAsicGlobalPM(oldObj, newObj, attrset)
	return ok, err
}

func (svcHdlr AsicDaemonServiceHandler) GetAsicGlobalPM(moduleId int8, resource string) (*asicdServices.AsicGlobalPM, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get asic global PM cfg obj- ", moduleId, resource))
	return svcHdlr.pluginMgr.GetAsicGlobalPM(uint8(moduleId), resource)
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkAsicGlobalPM(currMarker, count asicdServices.Int) (*asicdServices.AsicGlobalPMGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received for GetBulkAsicGlobalPM")
	start, end, cnt, more, list := svcHdlr.pluginMgr.GetBulkAsicGlobalPM(int(currMarker), int(count))
	bulkObj := new(asicdServices.AsicGlobalPMGetInfo)
	bulkObj.StartIdx = asicdServices.Int(start)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.Count = asicdServices.Int(cnt)
	bulkObj.More = more
	bulkObj.AsicGlobalPMList = list
	return bulkObj, nil
}

func (svcHdlr AsicDaemonServiceHandler) GetAsicGlobalPMState(moduleId int8, resource string) (*asicdServices.AsicGlobalPMState, error) {
	svcHdlr.logger.Debug("Thrift request received to get asic global PM state obj - ", resource)
	return svcHdlr.pluginMgr.GetAsicGlobalPMState(uint8(moduleId), resource)
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkAsicGlobalPMState(currMarker, count asicdServices.Int) (*asicdServices.AsicGlobalPMStateGetInfo, error) {
	return nil, errors.New("GetBulkAsicGlobalPMState is not supported")
}
