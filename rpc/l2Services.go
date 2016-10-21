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

// This file defines all interfaces provided for L2 service
package rpc

import (
	"asicdInt"
	"asicdServices"
	"errors"
	"fmt"
)

//LAG related APIs
func (svcHdlr AsicDaemonServiceHandler) CreateLag(ifName string, hashType int32, ports string) (rval int32, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to create lag - ", hashType, ports))
	rv, err := svcHdlr.pluginMgr.CreateLag(ifName, int(hashType), ports)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift create lag call returning - ", rv))
	return (int32)(rv), err
}

func (svcHdlr AsicDaemonServiceHandler) DeleteLag(ifIndex int32) (rval int32, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to delete lag - ", ifIndex))
	rv := svcHdlr.pluginMgr.DeleteLag(ifIndex)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift delete lag call returning - ", rv))
	return (int32)(rv), nil
}

func (svcHdlr AsicDaemonServiceHandler) UpdateLag(ifIndex, hashType int32, ports string) (rval int32, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to update lag - ", ifIndex, hashType, ports))
	rv, err := svcHdlr.pluginMgr.UpdateLag(ifIndex, int(hashType), ports)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift update lag returning - ", ifIndex, hashType, ports))
	return (int32)(rv), err
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkLag(currMarker, count asicdInt.Int) (*asicdInt.LagGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received to get bulk lag")
	endMarker, listLen, more, lagList := svcHdlr.pluginMgr.GetBulkLag(int(currMarker), int(count))
	if endMarker < 0 {
		return nil, errors.New("Failed to retrieve lag information")
	}
	bulkObj := asicdInt.NewLagGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdInt.Int(listLen)
	bulkObj.StartIdx = asicdInt.Int(currMarker)
	bulkObj.EndIdx = asicdInt.Int(endMarker)
	bulkObj.LagList = lagList
	svcHdlr.logger.Debug("Thrift getbulk lag call returning")
	return bulkObj, nil
}

//STP related APIs
func (svcHdlr AsicDaemonServiceHandler) CreateStg(vlanList []int32) (stgId int32, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to create STG - ", vlanList))
	stgId, err = svcHdlr.pluginMgr.CreateStg(vlanList)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift create STG returning", stgId, err))
	return stgId, err
}

func (svcHdlr AsicDaemonServiceHandler) DeleteStg(stgId int32) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to delete STG - ", stgId))
	rv, err = svcHdlr.pluginMgr.DeleteStg(stgId)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift delete STG returning - ", rv, err))
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) SetPortStpState(stgId, port, stpState int32) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to set port stp state - ", stgId, port, stpState))
	rv, err = svcHdlr.pluginMgr.SetPortStpState(stgId, port, stpState)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift set port stp state returning - ", stgId, port, stpState))
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) GetPortStpState(stgId, port int32) (stpState int32, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get port stp state - ", stgId, port))
	stpState, err = svcHdlr.pluginMgr.GetPortStpState(stgId, port)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift get port stp state returning - ", stpState, err))
	return stpState, err
}

func (svcHdlr AsicDaemonServiceHandler) UpdateStgVlanList(stgId int32, vlanList []int32) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to update stg vlan list - ", stgId, vlanList))
	rv, err = svcHdlr.pluginMgr.UpdateStgVlanList(stgId, vlanList)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift update stg vlan list returning - ", rv, err))
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) FlushFdbStgGroup(stgId, port int32) error {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to flush fdb stg group/port- ", stgId, port))
	svcHdlr.pluginMgr.FlushFdbStgGroup(stgId, port)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift flush fdb stg group returning"))
	return nil
}

//Protocol Mac Addr related APIs
func (svcHdlr AsicDaemonServiceHandler) EnablePacketReception(
	macObj *asicdInt.RsvdProtocolMacConfig) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to enable packet reception - ", macObj))
	rv, err = svcHdlr.pluginMgr.EnablePacketReception(macObj)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift enable packet reception returning - ", rv, err))
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) DisablePacketReception(
	macObj *asicdInt.RsvdProtocolMacConfig) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to disable packet reception - ", macObj))
	rv, err = svcHdlr.pluginMgr.DisablePacketReception(macObj)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift disable packet reception returning - ", rv, err))
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) GetMacTableEntryState(macAddr string) (*asicdServices.MacTableEntryState, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get mac table entry for mac - ", macAddr))
	return svcHdlr.pluginMgr.GetMacTableEntryState(macAddr)
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkMacTableEntryState(currMarker, count asicdServices.Int) (*asicdServices.MacTableEntryStateGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received for GetBulk MacTableEntry")
	end, listLen, more, objList := svcHdlr.pluginMgr.GetBulkMacTableEntryState(int(currMarker), int(count))
	if end < 0 {
		return nil, errors.New("Failed to retrieve mac table information")
	}
	bulkObj := asicdServices.NewMacTableEntryStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.MacTableEntryStateList = objList
	return bulkObj, nil
}
