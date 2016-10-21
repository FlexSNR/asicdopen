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

// This file defines all interfaces provided for the Vlan service
package rpc

import (
	"asicdInt"
	"asicdServices"
	"errors"
	"fmt"
)

func (svcHdlr AsicDaemonServiceHandler) CreateVlan(vlanObj *asicdServices.Vlan) (bool, error) {
	svcHdlr.logger.Debug("Thrift request received to create vlan - ", vlanObj)
	_, rv, err := svcHdlr.pluginMgr.CreateVlan(vlanObj)
	svcHdlr.logger.Debug("Thrift create vlan returning - ", rv, err)
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) UpdateVlan(oldVlanObj, newVlanObj *asicdServices.Vlan, attrset []bool, op []*asicdServices.PatchOpInfo) (rv bool, err error) {
	svcHdlr.logger.Debug("Thrift request received to update vlan - ", oldVlanObj, newVlanObj, " attrset:", attrset)
	if op == nil || len(op) == 0 {
		rv, err = svcHdlr.pluginMgr.UpdateVlan(oldVlanObj, newVlanObj)
	} else {
		rv, err = svcHdlr.pluginMgr.UpdateVlanForPatchUpdate(oldVlanObj, newVlanObj, op)
	}
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift update vlan returning - ", rv, err))
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) DeleteVlan(vlanObj *asicdServices.Vlan) (bool, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to delete vlan - ", vlanObj))
	_, rv, err := svcHdlr.pluginMgr.DeleteVlan(vlanObj)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift delete vlan returning - ", rv, err))
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkVlan(currMarker, count asicdInt.Int) (*asicdInt.VlanGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received for GetBulkVlan")
	end, listLen, more, vlanObjList := svcHdlr.pluginMgr.GetBulkVlan(int(currMarker), int(count))
	if end < 0 {
		return nil, errors.New("Failed to retrieve vlan information")
	}
	bulkObj := asicdInt.NewVlanGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdInt.Int(listLen)
	bulkObj.StartIdx = asicdInt.Int(currMarker)
	bulkObj.EndIdx = asicdInt.Int(end)
	bulkObj.VlanList = vlanObjList
	return bulkObj, nil
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkVlanState(currMarker, count asicdServices.Int) (*asicdServices.VlanStateGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received for GetBulkVlan")
	end, listLen, more, vlanStateObjList := svcHdlr.pluginMgr.GetBulkVlanState(int(currMarker), int(count))
	if end < 0 {
		return nil, errors.New("Failed to retrieve vlan information")
	}
	bulkObj := asicdServices.NewVlanStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.VlanStateList = vlanStateObjList
	return bulkObj, nil
}

func (svcHdlr AsicDaemonServiceHandler) GetVlanState(vlanId int32) (*asicdServices.VlanState, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get vlan state obj- ", vlanId))
	return svcHdlr.pluginMgr.GetVlanState(vlanId)
}
