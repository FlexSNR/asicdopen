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

// This file defines all interfaces provided for the LAG service
package rpc

import (
	"asicdServices"
	"errors"
	"fmt"
)

func (svcHdlr AsicDaemonServiceHandler) CreatePort(portObj *asicdServices.Port) (bool, error) {
	return false, errors.New("Create operation not supported on PortConfig object")
}

func (svcHdlr AsicDaemonServiceHandler) DeletePort(portObj *asicdServices.Port) (bool, error) {
	return false, errors.New("Delete operation not supported on PortConfig object")
}

func (svcHdlr AsicDaemonServiceHandler) UpdatePort(oldPortObj, newPortObj *asicdServices.Port, attrset []bool, op []*asicdServices.PatchOpInfo) (bool, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Received thrift request to update port config : old, new -", oldPortObj, newPortObj))
	rv, err := svcHdlr.pluginMgr.UpdatePortConfig(oldPortObj, newPortObj, attrset)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift update port call returning -", rv, err))
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkPort(currMarker, count asicdServices.Int) (*asicdServices.PortGetInfo, error) {
	svcHdlr.logger.Debug("Received GetBulkPort thrift request")
	endMarker, listLen, more, pCfgList := svcHdlr.pluginMgr.GetBulkPortConfig(int(currMarker), int(count))
	if endMarker < 0 {
		return nil, errors.New("Failed to retrieve port information")
	}
	bulkObj := asicdServices.NewPortGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(endMarker)
	bulkObj.PortList = pCfgList
	svcHdlr.logger.Debug("Thrift getbulk port call returning")
	return bulkObj, nil
}

func (svcHdlr AsicDaemonServiceHandler) GetPort(intfRef string) (*asicdServices.Port, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get port config for port", intfRef))
	portCfg, err := svcHdlr.pluginMgr.GetPortConfig(intfRef)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift get port call returning", intfRef, err))
	return portCfg, err
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkPortState(currMarker, count asicdServices.Int) (*asicdServices.PortStateGetInfo, error) {
	svcHdlr.logger.Debug("Received GetBulkPortState thrift request")
	endMarker, listLen, more, pStateList := svcHdlr.pluginMgr.GetBulkPortState(int(currMarker), int(count))
	if endMarker < 0 {
		return nil, errors.New("Failed to retrieve port stats information")
	}
	bulkObj := asicdServices.NewPortStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(endMarker)
	bulkObj.PortStateList = pStateList
	svcHdlr.logger.Debug("Thrift getbulk portstate call returning")
	return bulkObj, nil
}

func (svcHdlr AsicDaemonServiceHandler) GetPortState(intfRef string) (*asicdServices.PortState, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get port state for port", intfRef))
	portState, err := svcHdlr.pluginMgr.GetPortState(intfRef)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift get port state call returning", intfRef, err))
	return portState, err
}

func (svcHdlr AsicDaemonServiceHandler) ErrorDisablePort(ifIndex int32, adminState string, errDisableReason string) (bool, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to error disable port", ifIndex, adminState, errDisableReason))
	return svcHdlr.pluginMgr.ErrorDisablePort(ifIndex, adminState, errDisableReason)
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkBufferPortStatState(currMarker, count asicdServices.Int) (*asicdServices.BufferPortStatStateGetInfo, error) {
	svcHdlr.logger.Info("Received GetBulkBufferPortStatState thrift request")
	endMarker, listLen, more, bStateList := svcHdlr.pluginMgr.GetBulkBufferPortStatState(int(currMarker), int(count))
	if endMarker < 0 {
		return nil, errors.New("Failed to retrieve buffer stat information")
	}
	bulkObj := asicdServices.NewBufferPortStatStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(endMarker)
	bulkObj.BufferPortStatStateList = bStateList
	svcHdlr.logger.Debug("Thrift getbulk bufferstate call returning")
	return bulkObj, nil
	//return nil, errors.New("This is for future")
}

func (svcHdlr AsicDaemonServiceHandler) GetBufferPortStatState(intfRef string) (*asicdServices.BufferPortStatState, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get buffer stat for port", intfRef))
	bufferPortStat, err := svcHdlr.pluginMgr.GetBufferPortStatState(intfRef)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift get port state call returning", intfRef, err))
	return bufferPortStat, err
	//return nil, errors.New("this is for future")
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkBufferGlobalStatState(currMarker, count asicdServices.Int) (*asicdServices.BufferGlobalStatStateGetInfo, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received for get buffer global stats "))
	endMarker, listLen, more, bStateList := svcHdlr.pluginMgr.GetBulkBufferGlobalStatState(int(currMarker), int(count))
	if endMarker < 0 {
		return nil, errors.New("Failed to get buffer global stat information")
	}

	bulkObj := asicdServices.NewBufferGlobalStatStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(endMarker)
	bulkObj.BufferGlobalStatStateList = bStateList
	svcHdlr.logger.Debug("Thrift getbulk global bufferstate call returning")
	return bulkObj, nil

}

func (svcHdlr AsicDaemonServiceHandler) GetBufferGlobalStatState(deviceId int32) (*asicdServices.BufferGlobalStatState, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received for get buffer global stats device id ", 0))
	bufferGlobalStat, err := svcHdlr.pluginMgr.GetBufferGlobalStatState(deviceId)
	svcHdlr.logger.Debug("Thrift global bufferstate call returning")
	return bufferGlobalStat, err
}

func (svcHdlr AsicDaemonServiceHandler) CreateEthernetPM(obj *asicdServices.EthernetPM) (bool, error) {
	return false, errors.New("Create not supported for EthernetPM")
}

func (svcHdlr AsicDaemonServiceHandler) DeleteEthernetPM(obj *asicdServices.EthernetPM) (bool, error) {
	return false, errors.New("Delete not supported for EthernetPM")
}

func (svcHdlr AsicDaemonServiceHandler) UpdateEthernetPM(oldObj, newObj *asicdServices.EthernetPM, attrset []bool, op []*asicdServices.PatchOpInfo) (bool, error) {
	var ok bool
	svcHdlr.logger.Debug("Thrift request received to update ethernet PM cfg obj - ", oldObj, newObj, attrset)
	ok, err := svcHdlr.pluginMgr.UpdateEthernetPM(oldObj, newObj, attrset)
	return ok, err
}

func (svcHdlr AsicDaemonServiceHandler) GetEthernetPM(intfRef, resource string) (*asicdServices.EthernetPM, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get ethernet PM cfg obj- ", intfRef, resource))
	return svcHdlr.pluginMgr.GetEthernetPM(intfRef, resource)
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkEthernetPM(currMarker, count asicdServices.Int) (*asicdServices.EthernetPMGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received for GetBulkEthernetPM")
	start, end, cnt, more, list := svcHdlr.pluginMgr.GetBulkEthernetPM(int(currMarker), int(count))
	bulkObj := new(asicdServices.EthernetPMGetInfo)
	bulkObj.StartIdx = asicdServices.Int(start)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.Count = asicdServices.Int(cnt)
	bulkObj.More = more
	bulkObj.EthernetPMList = list
	return bulkObj, nil
}

func (svcHdlr AsicDaemonServiceHandler) GetEthernetPMState(intfRef, resource string) (*asicdServices.EthernetPMState, error) {
	svcHdlr.logger.Debug("Thrift request received to get ethernet PM state obj - ", intfRef, resource)
	return svcHdlr.pluginMgr.GetEthernetPMState(intfRef, resource)
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkEthernetPMState(currMarker, count asicdServices.Int) (*asicdServices.EthernetPMStateGetInfo, error) {
	return nil, errors.New("GetBulkEthernetPMState is not supported")
}
