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

func (svcHdlr AsicDaemonServiceHandler) CreateAcl(aclObj *asicdServices.Acl) (bool, error) {
	rv := svcHdlr.pluginMgr.CreateAclConfig(aclObj, "CONFIG")
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift creatr Acl obj returning - ", rv))
	return true, nil
}

func (svcHdlr AsicDaemonServiceHandler) CreateAclInternal(aclObj *asicdServices.Acl, clientInt string) (bool, error) {
	rv := svcHdlr.pluginMgr.CreateAclConfig(aclObj, clientInt)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift creatr Acl obj returning - ", rv))
	return true, nil
}

func (svcHdlr AsicDaemonServiceHandler) DeleteAcl(aclObj *asicdServices.Acl) (bool, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to delete acl Obj - ", aclObj))
	//rv, err := svcHdlr.pluginMgr.DeleteAclConfig(aclObj)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift delete acl object - "))
	return true, nil

}

func (svcHdlr AsicDaemonServiceHandler) UpdateAcl(oldAclObj, newAclObj *asicdServices.Acl, attrset []bool, op []*asicdServices.PatchOpInfo) (bool, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Received thrift request to update acl config : old, new -", oldAclObj, newAclObj))
	//rv, err := svcHdlr.pluginMgr.UpdateAclConfig(oldAclObj, newAclObj, attrset)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift update acl call returning -"))
	return true, nil
}

func (svcHdlr AsicDaemonServiceHandler) CreateAclRule(aclRuleObj *asicdServices.AclRule) (bool, error) {
	rv := svcHdlr.pluginMgr.CreateAclRuleConfig(aclRuleObj, "CONFIG")
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift creatr Acl rule obj returning - ", rv))
	return true, nil

}

func (svcHdlr AsicDaemonServiceHandler) CreateAclRuleInternal(aclRuleObj *asicdServices.AclRule, clientInt string) (bool, error) {
	rv := svcHdlr.pluginMgr.CreateAclRuleConfig(aclRuleObj, clientInt)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift creatr Acl rule obj returning - ", rv))
	return true, nil

}

func (svcHdlr AsicDaemonServiceHandler) DeleteAclRule(aclRuleObj *asicdServices.AclRule) (bool, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to delete acl rule Obj - ", aclRuleObj))
	//rv, err := svcHdlr.pluginMgr.DeleteAclRuleConfig(aclRuleObj)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift delete acl rule object - "))
	return true, nil

}

func (svcHdlr AsicDaemonServiceHandler) UpdateAclRule(oldAclRuleObj, newAclRuleObj *asicdServices.AclRule, attrset []bool, op []*asicdServices.PatchOpInfo) (bool, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Received thrift request to update acl rule config : old, new -", oldAclRuleObj, newAclRuleObj))
	//rv, err := svcHdlr.pluginMgr.UpdateAclConfig(oldAclRuleObj, newAclRuleObj, attrset)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift update acl rule call returning -"))
	return true, nil

}

func (svcHdlr AsicDaemonServiceHandler) GetAclRuleState(ruleName string) (*asicdServices.AclRuleState, error) {
	return nil, nil
}

func (svcHdlr AsicDaemonServiceHandler) GetAclState(aclName string, name string) (*asicdServices.AclState, error) {
	return nil, nil
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkAclState(currMarker, count asicdServices.Int) (*asicdServices.AclStateGetInfo, error) {
	return nil, nil
}

func (svcHdlr AsicDaemonServiceHandler) GetBulkAclRuleState(currMarker, count asicdServices.Int) (*asicdServices.AclRuleStateGetInfo, error) {
	svcHdlr.logger.Debug("Received GetBulkAclRuleState ", currMarker, count)
	endMarker, listLen, more, aclStateList := svcHdlr.pluginMgr.GetBulkAclRuleState(int(currMarker), int(count))
	if endMarker < 0 {
		return nil, errors.New("Failed to retrieve Acl rule state information.")
	}
	bulkObj := asicdServices.NewAclRuleStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(endMarker)
	bulkObj.AclRuleStateList = aclStateList
	svcHdlr.logger.Debug("Thrift getbulk acl rule call returning")
	return bulkObj, nil

}

func (svcHdlr AsicDaemonServiceHandler) GetBulkCoppStatState(currMarker, count asicdServices.Int) (*asicdServices.CoppStatStateGetInfo, error) {
	svcHdlr.logger.Debug("Received GetBulkCoppStatState thrift request")
	endMarker, listLen, more, coppStateList := svcHdlr.pluginMgr.GetBulkCoppStatState(int(currMarker), int(count))
	if endMarker < 0 {
		return nil, errors.New("Failed to retrieve CoPP information")
	}
	bulkObj := asicdServices.NewCoppStatStateGetInfo()
	bulkObj.More = more
	bulkObj.Count = asicdServices.Int(listLen)
	bulkObj.StartIdx = asicdServices.Int(currMarker)
	bulkObj.EndIdx = asicdServices.Int(endMarker)
	bulkObj.CoppStatStateList = coppStateList
	svcHdlr.logger.Debug("Thrift getbulk CoPP call returning")
	return bulkObj, nil
}

func (svcHdlr AsicDaemonServiceHandler) GetCoppStatState(proto string) (*asicdServices.CoppStatState, error) {
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift request received to get Copp state for protocol", proto))
	coppState, err := svcHdlr.pluginMgr.GetCoppStatState(proto)
	svcHdlr.logger.Debug(fmt.Sprintln("Thrift get CoPP state call returning", proto, err))
	return coppState, err
}
