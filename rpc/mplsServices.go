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

// This file defines all interfaces provided for the MPLS service
package rpc

/*
 *VR : Temporarily commenting out mpls code from asicd
 *
import (
	"asicdServices"
	"errors"
	"fmt"
)

//MPLS interface related services
func (svcHdlr AsicDaemonServiceHandler) CreateMplsIntf(mplsIntfObj *asicdServices.MplsIntf) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received for create mplsintf - ", mplsIntfObj))
	rv, err = svcHdlr.pluginMgr.CreateMplsIntf(mplsIntfObj)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift create mplsintf returning - ", rv, err))
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) UpdateMplsIntf(oldMplsIntfObj, newMplsIntfObj *asicdServices.MplsIntf, attrset []bool, op []*asicdServices.PatchOpInfo) (bool, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received for update mplsintf - ", oldMplsIntfObj, newMplsIntfObj))
	_, err := svcHdlr.pluginMgr.DeleteMplsIntf(oldMplsIntfObj)
	if err != nil {
		return false, err
	}
	_, err = svcHdlr.pluginMgr.CreateMplsIntf(newMplsIntfObj)
	if err != nil {
		return false, err
	}
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift update mplsintf returning - ", err))
	return true, nil
}

func (svcHdlr AsicDaemonServiceHandler) DeleteMplsIntf(mplsIntfObj *asicdServices.MplsIntf) (rv bool, err error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received for delete mplsintf - ", mplsIntfObj))
	rv, err = svcHdlr.pluginMgr.DeleteMplsIntf(mplsIntfObj)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift delete mplsintf returning - ", rv, err))
	return rv, err
}

func (svcHdlr AsicDaemonServiceHandler) GetMplsIntfState(intfRef string) (*asicdServices.MplsIntfState, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get mpls intf state for intf - ", intfRef))
	mplsIntfState, err := svcHdlr.pluginMgr.GetMplsIntfState(intfRef)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift get mplsintf for ifindex returning - ", mplsIntfState, err))
	return mplsIntfState, err
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkMplsIntfState(currMarker, count asicdServices.Int) (*asicdServices.MplsIntfStateGetInfo, error) {
	svcHdlr.logger.Debug("Thrift request received to getbulk mplsintfstate")
	end, listLen, more, mplsIntfList := svcHdlr.pluginMgr.GetBulkMplsIntfState(int(currMarker), int(count))
	if end < 0 {
		return nil, errors.New("Failed to retrieve MplsIntf state information")
	}
	bulkObj := asicdServices.NewMplsIntfStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(end)
	bulkObj.MplsIntfStateList = mplsIntfList
	return bulkObj, nil
}
*/
