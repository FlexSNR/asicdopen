//
//Copyright [2016] [SnapRoute Inc]
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//       Unless required by applicable law or agreed to in writing, software
//       distributed under the License is distributed on an "AS IS" BASIS,
//       WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//       See the License for the specific language governing permissions and
//       limitations under the License.
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
	"asicdServices"
	"errors"
	"fmt"
	"models/objects"
	"net"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"utils/dbutils"
	"utils/logging"
)

const (
	INVALID_RULE_KEY = "DEADBEAF"
	ACL_VALID        = 1
	ACL_INVALID      = 0
	ACL_CLIENT_CFG   = "CONFIG"
	ACL_RULE_ADD     = 2
	ACL_RULE_UPDATE  = 3
	ACL_RULE_DELETE  = 4
)

type AclManager struct {
	dbMutex               sync.RWMutex
	plugins               []PluginIntf
	logger                *logging.Writer
	dbHdl                 *dbutils.DBUtil
	aMgr                  *AclManager
	pMgr                  *PortManager
	ifMgr                 *IfManager
	ruleToAclKeyMap       map[string][]string
	aclKeyToRulesMap      map[string][]string
	aclToIfIndexMap       map[string][]int32
	aclRuleKeySlice       []string
	aclRuleStateMap       map[string]*asicdServices.AclRuleState
	aclNameToRuleIndexMap map[string]int
	CoPPMap               map[string]*asicdServices.CoppStatState
	initComplete          bool
}

var AclMgr AclManager

func (aMgr *AclManager) Init(rsrcMgrs *ResourceManagers, dbHdl *dbutils.DBUtil, pluginList []PluginIntf, logger *logging.Writer) {

	aMgr.logger = logger
	aMgr.dbHdl = dbHdl
	aMgr.pMgr = rsrcMgrs.PortManager
	aMgr.ifMgr = rsrcMgrs.IfManager
	aMgr.plugins = pluginList
	aMgr.ruleToAclKeyMap = make(map[string][]string)
	aMgr.aclKeyToRulesMap = make(map[string][]string)
	aMgr.aclToIfIndexMap = make(map[string][]int32)
	aMgr.CoPPMap = make(map[string]*asicdServices.CoppStatState)
	aMgr.aclRuleKeySlice = []string{}
	aMgr.aclRuleStateMap = make(map[string]*asicdServices.AclRuleState)
	aMgr.aclNameToRuleIndexMap = make(map[string]int)
	for index := 0; index < pluginCommon.MAX_COPP_CLASS_COUNT; index++ {
		coppClass := pluginCommon.CoppType[index]
		aMgr.CoPPMap[coppClass] = asicdServices.NewCoppStatState()
	}
	aMgr.initComplete = true
	if dbHdl != nil {
		// TODO add for restart logic.
		logger.Debug("Run DB query to apply ACls.")
		aMgr.RestoreAclRuleDB(dbHdl)
		aMgr.RestoreAclDB(dbHdl)
	}
	aMgr.logger.Info("Acl Manger Init is done")
}

func (aMgr *AclManager) RestoreAclRuleDB(dbHdl *dbutils.DBUtil) {
	var objAclRule objects.AclRule
	aclRules, err := dbHdl.GetAllObjFromDb(objAclRule)
	if err == nil {
		for idx := 0; idx < len(aclRules); idx++ {
			obj := asicdServices.NewAclRule()
			dbObj := aclRules[idx].(objects.AclRule)
			objects.ConvertasicdAclRuleObjToThrift(&dbObj, obj)
			rv, _ := aMgr.CreateAclRuleConfig(obj, ACL_CLIENT_CFG)
			if rv != true {
				aMgr.logger.Err("Acl : Restart Failed to create acl rule ", obj.RuleName)
			}

		}
	} else {
		aMgr.logger.Err("Acl : DB querry failed during Acl rule : acl init")
	}
}

func (aMgr *AclManager) RestoreAclDB(dbHdl *dbutils.DBUtil) {
	var objAcl objects.Acl
	acls, err := dbHdl.GetAllObjFromDb(objAcl)
	if err == nil {
		for idx := 0; idx < len(acls); idx++ {
			obj := asicdServices.NewAcl()
			dbObj := acls[idx].(objects.Acl)
			objects.ConvertasicdAclObjToThrift(&dbObj, obj)
			rv, _ := aMgr.CreateAclConfig(obj, ACL_CLIENT_CFG)
			if rv != false {
				aMgr.logger.Err("Acl : Restart Failed to create acl ", obj.AclName)
			}

		}
	} else {
		aMgr.logger.Err("Acl : DB querry failed during Acl rule : acl init")
	}
}

func (aMgr *AclManager) Deinit() {
}

func (aMgr *AclManager) CreateAclConfig(acl *asicdServices.Acl, clientName string) (bool, error) {
	aMgr.logger.Debug("Acl: Received acl config create ", acl, " client ", clientName)
	if !aMgr.initComplete {
		aMgr.logger.Err("Acl : Acl manager not initialized yet.. Not applied ", acl.AclName)
		return false, errors.New("ACL isn't initialized yet")
	}

	if aMgr.dbHdl == nil {
		aMgr.logger.Err("Null db handle. Failed to apply ACl.")
		return false, errors.New("Can't connect to DB")
	}

	if len(acl.RuleNameList) == 0 {
		aMgr.logger.Info("Acl rule list is nil . Dont apply ifIndex ")
		return false, errors.New("Rule list can't be empty")
	}

	if len(acl.IntfList) == 0 {
		aMgr.logger.Info("Acl intf list is nil .Rule is not applied to any port :", acl)
		return false, errors.New("Intf List can't be empty")
	}

	// Bcm no support ACL apply in egress
	if acl.Direction == "OUT" {
		return false, errors.New("Not yet support ACL Direction(OUT)")
	}

	if _, ok := aMgr.aclKeyToRulesMap[acl.AclName]; !ok {
		aMgr.aclKeyToRulesMap[acl.AclName] = make([]string, 0)
	}

	for _, rule := range acl.RuleNameList {
		rv := aMgr.processRuleFromAcl(acl, rule)
		if rv < 0 {
			aMgr.logger.Err("Acl : Failed to process acl rule from acl ", acl)
			return false, errors.New(fmt.Sprintln("Failed to process acl rule ", rule))
		}
		rv = aMgr.addAclToDb(acl)
		if rv < 0 {
			aMgr.logger.Err("Acl: Failed to process acl rule ",
				rule, " acl ", acl)
			return false, errors.New("Failed to save acl to DB")
		}

	}
	return true, nil
}

func (aMgr *AclManager) processRuleFromAcl(acl *asicdServices.Acl, rule string) int {
	aMgr.ruleToAclKeyMap[rule] = append(aMgr.ruleToAclKeyMap[rule], acl.AclName)
	aMgr.aclKeyToRulesMap[acl.AclName] = append(aMgr.aclKeyToRulesMap[rule], rule)

	var ruleDbObj objects.AclRule
	//var ruleTemp objects.AclRule
	ruleObj := asicdServices.NewAclRule()
	key := "AclRule#" + rule
	obj, err := aMgr.dbHdl.GetObjectFromDb(ruleDbObj, key)
	if err != nil {
		aMgr.logger.Debug("Acl: Failed to get object from db for rule name. Rule will not be applied  ", rule)
		return -1
	}
	ruleTemp := obj.(objects.AclRule)
	objects.ConvertasicdAclRuleObjToThrift(&ruleTemp, ruleObj)
	aMgr.logger.Debug("Obtained the db obj as ", ruleObj)
	aclRule, err := aMgr.GetPluginAclRuleObjFromThriftObj(ruleObj)
	if err != nil {
		aMgr.logger.Err("Acl: Failed to convert thrift acl rule obj to common obj.Will still try to apply other rules ", ruleObj)
		return -1
	}
	// TODO validate ACL
	portList := []int32{}
	for _, intf := range acl.IntfList {
		ifIndex, err := aMgr.ifMgr.GetIfIndexForIfName(intf)
		if err != nil {
			aMgr.logger.Err("Acl: Invalid ifname . ", intf)
			return -1
		}
		port := aMgr.pMgr.GetPortNumFromIfIndex(ifIndex)
		aMgr.logger.Debug("Acl: Ifindex - port ", ifIndex, port)
		portList = append(portList, port)
		aMgr.logger.Debug("Acl: added port to the list intf , ifIndex, port ", intf, ifIndex, port)
	}

	for _, plugin := range aMgr.plugins {
		rv := plugin.CreateAclConfig(acl.AclName, acl.AclType, *aclRule, portList, acl.Direction)
		aMgr.logger.Debug("Acl: Call plugin createAcl config API ", acl.AclName)
		if rv < 0 {
			aMgr.logger.Err("Acl: Failed to create Acl config ", acl.AclName, " - ", rv)
			return rv
		}
		aMgr.UpdateAclRuleStateSlice(ruleObj, ACL_RULE_UPDATE, acl.AclName, acl.AclType, 0, "Applied")
	}
	return 0
}

func (aMgr *AclManager) addAclToDb(acl *asicdServices.Acl) int {
	var dbObj objects.AclState
	var aclState *asicdServices.AclState

	aclState = asicdServices.NewAclState()
	aclState.AclName = acl.AclName
	aclState.Direction = acl.Direction
	for _, intf := range acl.IntfList {
		aclState.IntfList = append(aclState.IntfList, intf)
	}
	for _, rule := range acl.RuleNameList {
		aclState.RuleNameList = append(aclState.RuleNameList, rule)
	}
	objects.ConvertThriftToasicdAclStateObj(aclState, &dbObj)
	if aMgr.dbHdl == nil {
		aMgr.logger.Err("Acl: Nil db handle acl wont be added in state db.", acl)
		return -1
	}
	err := dbObj.StoreObjectInDb(aMgr.dbHdl)
	if err != nil {
		aMgr.logger.Err("Acl : failed to add object in redis db with err ", err, " acl ", acl)
		return -1
	}
	return 0
}
func (aMgr *AclManager) ValidateAcl(aclName string, ifIndex int32) int {
	return ACL_VALID
}

func (aMgr *AclManager) CreateAclRuleConfig(aclRule *asicdServices.AclRule, aclClient string) (bool, error) {
	aMgr.logger.Debug("Acl : Received create Acl rule config aclRule", aclRule, " client ", aclClient)
	if !aMgr.initComplete {
		aMgr.logger.Err("Acl : Acl manager not initialized yet.")
		return false, errors.New("ACL isn't initialized yet")
	}
	if _, ok := aMgr.ruleToAclKeyMap[aclRule.RuleName]; !ok {
		aMgr.ruleToAclKeyMap[aclRule.RuleName] = []string{}
	}
	// Add the acl rule to db if it is internal client using ACL.
	if strings.Compare(aclClient, ACL_CLIENT_CFG) != 0 {
		var dbObj objects.AclRule
		if aMgr.dbHdl == nil {
			aMgr.logger.Err("Acl: Dbhdl is nil. Acl rule will not be stored in db. ", aclRule)
			return false, errors.New("Can't connect to DB")
		}

		objects.ConvertThriftToasicdAclRuleObj(aclRule, &dbObj)
		err := dbObj.StoreObjectInDb(aMgr.dbHdl)
		if err != nil {
			aMgr.logger.Err("Acl: Failed to save the rule in redis db. , acl rule ", aclRule, " err ", err)
			return false, errors.New("Can't save data to DB")
		}
		aMgr.logger.Debug("Acl : successfully saved rule in redis db ", aclRule)
	}
	aMgr.UpdateAclRuleStateSlice(aclRule, ACL_RULE_ADD, "NONE", "NONE", 0, "Not Applied")
	return true, nil
}

func (aMgr *AclManager) UpdateAclRuleConfig(oldAclRule, newAclRule *asicdServices.AclRule, attrset []bool) (bool, error) {
	acls, ok := aMgr.ruleToAclKeyMap[oldAclRule.RuleName]
	if ok {
		aclRule, err := aMgr.GetPluginAclRuleObjFromThriftObj(newAclRule)
		if err != nil {
			aMgr.logger.Err("Acl: Failed to convert thrift acl rule obj to common obj.Will still try to apply other rules ", newAclRule)
			return false, nil
		}
		for _, acl := range acls {
			for _, plugin := range aMgr.plugins {
				rv := plugin.UpdateAclRule(acl, *aclRule)
				aMgr.logger.Debug("Acl: Call plugin update Acl rule ", newAclRule.RuleName)
				if rv < 0 {
					aMgr.logger.Err("Acl: Failed to update Acl rule ", newAclRule.RuleName)
					return false, errors.New(fmt.Sprintln("Failed to update. ", newAclRule.RuleName))
				}
			}
		}
	} else {
		return false, errors.New(fmt.Sprintln("Acl rule does not exist ", oldAclRule.RuleName))
	}
	return true, nil
}

func (aMgr *AclManager) DeleteAcl(acl *asicdServices.Acl) (bool, error) {
	aclName := acl.AclName
	_, ok := aMgr.aclKeyToRulesMap[aclName]
	if ok {
		for _, plugin := range aMgr.plugins {
			rv := plugin.DeleteAcl(aclName, acl.Direction)
			aMgr.logger.Debug("Acl: Call plugin delete Acl ", aclName)
			if rv < 0 {
				aMgr.logger.Err("Acl: Failed to delete Acl ", aclName)
				return false, errors.New(fmt.Sprintln("Failed to delete. ", aclName))
			}
		}
		delete(aMgr.aclToIfIndexMap, aclName)
		delete(aMgr.aclKeyToRulesMap, aclName)
		// delete data from redis db
	} else {
		aMgr.logger.Err(fmt.Sprintln("Acl : Delete - not found in the map. failed for acl ", aclName))
		return false, errors.New(fmt.Sprintln("Acl not found . ", aclName))
	}
	return true, nil
}

func (aMgr *AclManager) DeleteAclRuleFromAcl(acl *asicdServices.Acl, rule string) (bool, error) {

	var ruleDbObj objects.AclRule
	ruleObj := asicdServices.NewAclRule()
	key := "AclRule#" + rule
	obj, err := aMgr.dbHdl.GetObjectFromDb(ruleDbObj, key)
	if err != nil {
		aMgr.logger.Debug("Acl: Failed to get object from db for rule name. Rule will not be applied  ", rule)
		return false, nil
	}
	ruleTemp := obj.(objects.AclRule)
	objects.ConvertasicdAclRuleObjToThrift(&ruleTemp, ruleObj)
	aMgr.logger.Debug("Obtained the db obj as ", ruleObj)
	aclRule, err := aMgr.GetPluginAclRuleObjFromThriftObj(ruleObj)
	if err != nil {
		aMgr.logger.Err("Acl: Failed to convert thrift acl rule obj to common obj.Will still try to apply other rules ", ruleObj)
		return false, nil
	}

	portList := []int32{}
	for _, intf := range acl.IntfList {
		ifIndex, err := aMgr.ifMgr.GetIfIndexForIfName(intf)
		if err != nil {
			aMgr.logger.Err("Acl: Invalid ifname . ", intf)
			return false, nil
		}
		port := aMgr.pMgr.GetPortNumFromIfIndex(ifIndex)
		aMgr.logger.Debug("Acl: Ifindex - port ", ifIndex, port)
		portList = append(portList, port)
		aMgr.logger.Debug("Acl: added port to the list intf , ifIndex, port ", intf, ifIndex, port)
	}

	for _, plugin := range aMgr.plugins {
		rv := plugin.DeleteAclRuleFromAcl(acl.AclName, *aclRule, portList, acl.Direction)
		aMgr.logger.Debug("Acl: Call plugin delete AclRule ", rule, " from ", acl.AclName)
		if rv < 0 {
			aMgr.logger.Err("Acl: Failed to delete AclRule ", rule)
			return false, errors.New(fmt.Sprintln("Failed to delete. ", rule))
		}
	}
	return true, nil
}

/* Allow to delete acl rule only if it
is not attached to any Acl.
*/
func (aMgr *AclManager) DeleteAclRule(aclRuleObj *asicdServices.AclRule) (bool, error) {
	ruleName := aclRuleObj.RuleName
	acls, ok := aMgr.ruleToAclKeyMap[ruleName]
	if ok {
		if len(acls) > 0 {
			return false, errors.New(fmt.Sprintln("Acl rule is still attached to acl(s). Remove from acl first. ", acls))
		}
		delete(aMgr.ruleToAclKeyMap, ruleName)
		delete(aMgr.aclRuleStateMap, ruleName)
		for index, sliceRuleName := range aMgr.aclRuleKeySlice {
			if sliceRuleName == ruleName {
				aMgr.aclRuleKeySlice = append(aMgr.aclRuleKeySlice[:index], aMgr.aclRuleKeySlice[index+1:]...)
				break
			}
		}

	} else {
		return false, errors.New(fmt.Sprintln("Acl rule does not exist ", ruleName))
	}
	return true, nil
}

func (aMgr *AclManager) DeleteAclRuleFromAclMap(aclName string, ruleName string) int {
	acls, ok := aMgr.ruleToAclKeyMap[ruleName]
	if ok {
		for index, acl := range acls {
			if acl == aclName {
				acls = append(acls[:index], acls[index+1:]...)
				aMgr.ruleToAclKeyMap[ruleName] = acls
				break
			}
		}
	} else {
		aMgr.logger.Info("Acl : Failed to delete rule from map ",
			aclName, " rule ", ruleName)
	}
	return 0
}

/*
 * If aclRule is not attached to any acl
 * delete it from hardware.
 */
func (aMgr *AclManager) UpdateAcl(aclOld *asicdServices.Acl, aclNew *asicdServices.Acl) (bool, error) {
	_, ok := aMgr.aclKeyToRulesMap[aclOld.AclName]
	aMgr.logger.Debug("Acl: Acl update old acl ", aclOld, " new acl ", aclNew)
	if ok {
		// Port list has been changed
		if !reflect.DeepEqual(aclOld.IntfList, aclNew.IntfList) {
			return false, errors.New("No support to change IntfList")
		}
		// find newly created rules
		for _, ruleName := range aclNew.RuleNameList {
			newRule := true
			for _, oldruleName := range aclOld.RuleNameList {
				if oldruleName == ruleName {
					newRule = false
					break
				}
			}
			if newRule {
				//create new rule
				rv := aMgr.processRuleFromAcl(aclNew, ruleName)
				if rv < 0 {
					aMgr.logger.Err("Acl : Update acl - failed to create new rule",
						ruleName, " acl ", aclNew)
					return false, errors.New(fmt.Sprintln("Failed to create new rule ", ruleName))
				}
			}
		}
		// find deleted rules
		for _, oldRuleName := range aclOld.RuleNameList {
			deleteRule := true
			for _, newRuleName := range aclNew.RuleNameList {
				if newRuleName == oldRuleName {
					deleteRule = false
					break
				}
			}
			if deleteRule {
				//delete rule from s/w cache and h/w
				rv := aMgr.DeleteAclRuleFromAclMap(aclNew.AclName, oldRuleName)
				if rv < 0 {
					aMgr.logger.Err(fmt.Sprintln("Acl : Failed to delete from s/w cache ", aclNew.AclName, ":", oldRuleName))
				}
				_, err := aMgr.DeleteAclRuleFromAcl(aclOld, oldRuleName)
				if err != nil {
					return false, errors.New(fmt.Sprintln("Acl : Failed to delete rule ", oldRuleName,
						"acl ", aclNew))
				}
			}
		}
	} else {
		return false, errors.New(fmt.Sprintln("Acl doest not exist. ", aclOld.AclName))
	}
	return true, nil
}

func (aMgr *AclManager) GetPortListFromIntfList(intfList []string) (portList []int32, err error) {
	portList = []int32{}
	for _, intf := range intfList {
		ifIndex, rv := aMgr.ifMgr.GetIfIndexForIfName(intf)
		if rv != nil {
			aMgr.logger.Err("ACL: Invalid ifname . ", intf)
			continue
		}
		port := aMgr.pMgr.GetPortNumFromIfIndex(ifIndex)
		aMgr.logger.Debug("Acl: Ifindex - port ", ifIndex, port)
		portList = append(portList, port)
		aMgr.logger.Debug("ACL: added port to the list intf , ifIndex, port ", intf, ifIndex, port)
	}
	return portList, nil

}
func (aMgr *AclManager) GetPluginAclObjFromThriftObj(aclTh *asicdServices.Acl) (*pluginCommon.Acl, error) {

	obj := new(pluginCommon.Acl)
	// add rest of the fields
	obj.AclName = aclTh.AclName
	obj.Direction = aclTh.Direction
	obj.RuleNameList = make([]string, 0)
	for _, rule := range aclTh.RuleNameList {
		obj.RuleNameList = append(obj.RuleNameList, rule)
	}
	return obj, nil
}

func (aMgr *AclManager) GetPluginAclRuleObjFromThriftObj(aclRuleTh *asicdServices.AclRule) (*pluginCommon.AclRule, error) {
	var err error
	obj := new(pluginCommon.AclRule)
	obj.RuleName = aclRuleTh.RuleName
	obj.SourceMac, _ = net.ParseMAC(aclRuleTh.SourceMac)
	obj.DestMac, _ = net.ParseMAC(aclRuleTh.DestMac)

	_, obj.SourceIp, _, err = IPAddrStringToU8List(aclRuleTh.SourceIp)
	if err != nil {
		aMgr.logger.Debug("Acl : Null source ip . Assign null ", aclRuleTh.RuleName)
		obj.SourceIp = []uint8{}
	}

	_, obj.DestIp, _, err = IPAddrStringToU8List(aclRuleTh.DestIp)
	if err != nil {
		aMgr.logger.Debug("Acl : Null dest ip assign null ", aclRuleTh.RuleName)
		obj.DestIp = []uint8{}
	}

	_, obj.SourceMask, _, err = IPAddrStringToU8List(aclRuleTh.SourceMask)

	if err != nil {
		obj.SourceMask = []uint8{}
		aMgr.logger.Debug("Acl : Null source mask ip assign null ", aclRuleTh.RuleName)
	}

	_, obj.DestMask, _, err = IPAddrStringToU8List(aclRuleTh.DestMask)
	if err != nil {
		obj.DestMask = []uint8{}
		aMgr.logger.Debug("Acl : Null dest mask ip assign null ", aclRuleTh.RuleName)
	}

	obj.Action = aclRuleTh.Action
	protoUpper := strings.ToUpper(aclRuleTh.Proto)
	protoType, exist := pluginCommon.AclProtoType[protoUpper]
	if exist {
		obj.Proto = protoType
	} else {
		obj.Proto = -1
	}
	if len(aclRuleTh.SrcPort) != 0 && (strings.Compare("0", aclRuleTh.SrcPort) != 0) {
		sp, err := aMgr.ifMgr.GetIfIndexForIfName(aclRuleTh.SrcPort)
		if err != nil {
			aMgr.logger.Err("Acl : Failed to get IfIndex for src port ", aclRuleTh.SrcPort, aclRuleTh.RuleName)
			return obj, err
		}
		obj.SrcPort = aMgr.pMgr.GetPortNumFromIfIndex(sp)
	}

	if len(aclRuleTh.DstPort) != 0 && (strings.Compare("0", aclRuleTh.DstPort) != 0) {
		dp, err := aMgr.ifMgr.GetIfIndexForIfName(aclRuleTh.DstPort)
		if err != nil {
			aMgr.logger.Err("acl : Failed to get ifIndex from dstport ", aclRuleTh.DstPort, aclRuleTh.RuleName)
			return obj, err
		}
		obj.DstPort = aMgr.pMgr.GetPortNumFromIfIndex(dp)
	}
	obj.L4SrcPort = aclRuleTh.L4SrcPort
	obj.L4DstPort = aclRuleTh.L4DstPort
	obj.L4PortMatch = aclRuleTh.L4PortMatch
	obj.L4MinPort = aclRuleTh.L4MinPort
	obj.L4MaxPort = aclRuleTh.L4MaxPort
	_, ok := aMgr.aclNameToRuleIndexMap[aclRuleTh.RuleName]
	if !ok {
		aMgr.logger.Err("Acl : Rule is not present in the aclState map .Stats wont be updated.", aclRuleTh.RuleName)
		return obj, nil
	}
	obj.RuleIndex = aMgr.aclNameToRuleIndexMap[aclRuleTh.RuleName]
	return obj, nil

}

/*** Get bulk APIs ***/

func (aMgr *AclManager) UpdateAclRuleStateSlice(aclRule *asicdServices.AclRule,
	msg int,
	aclName string,
	aclType string,
	hitCount int64,
	hwApplied string) {
	aMgr.logger.Debug("Acl : Update acl rule state slice ", aclRule.RuleName)
	//ruleState asicdServices.AclRuleState
	switch msg {
	case ACL_RULE_ADD:
		ruleState := asicdServices.NewAclRuleState()
		ruleState.RuleName = aclRule.RuleName
		ruleState.AclType = "NA"
		// ruleState.IntfList = ""
		ruleState.HwPresence = hwApplied
		ruleState.HitCount = 0
		aMgr.aclRuleStateMap[aclRule.RuleName] = ruleState
		aMgr.aclRuleKeySlice = append(aMgr.aclRuleKeySlice, aclRule.RuleName)
		aMgr.aclNameToRuleIndexMap[aclRule.RuleName] = len(aMgr.aclRuleKeySlice) - 1
		aMgr.logger.Debug("Acl : added acl slice name ", aclRule.RuleName, " index ", len(aMgr.aclRuleKeySlice))
		return
	case ACL_RULE_UPDATE:
		var ruleState *asicdServices.AclRuleState
		var ok bool
		if ruleState, ok = aMgr.aclRuleStateMap[aclRule.RuleName]; !ok {
			ruleState = asicdServices.NewAclRuleState()
		}
		ruleState.RuleName = aclRule.RuleName
		ruleState.AclType = aclType
		ruleState.HwPresence = hwApplied
		ruleState.HitCount = hitCount
		aMgr.aclRuleStateMap[aclRule.RuleName] = ruleState
		aMgr.logger.Debug("Acl : Updated acl slice ", aclRule.RuleName, " rule ", ruleState)
		return

	case ACL_RULE_DELETE:
		var index int
		for index = 0; index < len(aMgr.aclRuleKeySlice); index++ {
			if aMgr.aclRuleKeySlice[index] == aclRule.RuleName {
				break
			}
		}
		aMgr.aclRuleKeySlice = append(aMgr.aclRuleKeySlice[:index], aMgr.aclRuleKeySlice[index+1:]...)
		delete(aMgr.aclRuleStateMap, aclRule.RuleName)
		return

	default:
		aMgr.logger.Info("Acl : ", aclName, " invalid action , No state updated. ", msg)
		return
	}
}

func UpdateAclStateDB(ruleStat *pluginCommon.AclRuleState) {
	AclMgr.logger.Debug("Acl : Received stats for ", ruleStat.SliceIndex,
		" stat ", ruleStat.Stat, " index ", ruleStat.SliceIndex)
	if ruleStat.SliceIndex > len(AclMgr.aclRuleKeySlice) {
		AclMgr.logger.Info("Acl : Slice index exceeds max size of acl rule stats. index , len ",
			ruleStat.SliceIndex, len(AclMgr.aclRuleKeySlice))
	}
	var statObj *asicdServices.AclRuleState
	var ok bool
	ruleName := AclMgr.aclRuleKeySlice[ruleStat.SliceIndex]
	if statObj, ok = AclMgr.aclRuleStateMap[ruleName]; !ok {
		return
	}
	statObj.HitCount = int64(ruleStat.Stat)
	statObj.HwPresence = "Applied"
	AclMgr.aclRuleStateMap[ruleName] = statObj
	AclMgr.logger.Debug("Acl : Success to update stat index - stat ", ruleStat.SliceIndex, ruleStat.Stat)
}

func (aMgr *AclManager) GetBulkAclRuleState(idx, cnt int) (end, listLen int, more bool, bStateList []*asicdServices.AclRuleState) {
	if !aMgr.initComplete {
		aMgr.logger.Err("Acl : Acl manager not initialized yet..")
		return 0, 0, false, nil
	}
	var nextIdx int
	var count int
	more = false
	length := len(aMgr.aclRuleKeySlice)
	if idx+cnt >= length {
		count = length - idx
		nextIdx = 0
	} else {
		nextIdx = idx + cnt + 1
	}

	/*
		for _, plugin := range aMgr.pMgr.plugins {
			if isControllingPlugin(plugin) == true {
				rv := plugin.UpdateAclStateDB(idx, count)
				if rv < 0 {
					aMgr.logger.Debug("Acl : Failed to get acl rule state from plugin start id ",
						idx, " count ", count)
				}
			}
		} */
	aMgr.logger.Debug("Acl : count ", count)
	for _, plugin := range aMgr.plugins {
		for i := idx; i < count; i++ {
			ruleKey := aMgr.aclRuleKeySlice[i]
			var aclState *asicdServices.AclRuleState
			var ok bool
			if isControllingPlugin(plugin) == true {
				rv := plugin.UpdateAclStateDB(ruleKey)
				if rv < 0 {
					aMgr.logger.Debug("Acl : Can not get acl rule state for ", ruleKey)
				}
			}
			if aclState, ok = aMgr.aclRuleStateMap[ruleKey]; !ok {
				continue
			}
			bStateList = append(bStateList, aclState)
		}
	}
	if idx >= len(aMgr.aclRuleKeySlice) {
		more = false
	}
	aMgr.logger.Debug("Acl: Return from get bulk acl rule more ", more)
	return nextIdx, count, more, bStateList
}

func (aMgr *AclManager) GetAclRuleState(ruleName string) (*asicdServices.AclRuleState, error) {
	if !aMgr.initComplete {
		aMgr.logger.Err("Acl : Acl manager not initialized yet..")
		return nil, errors.New("Acl manager is not initialized yet.")
	}
	var aclState *asicdServices.AclRuleState
	var ok bool
	result := asicdServices.NewAclRuleState()
	if aclState, ok = aMgr.aclRuleStateMap[ruleName]; !ok {
		return nil, errors.New("Acl : Acl rule key does not exist ")
	}
	result.RuleName = aclState.RuleName
	result.AclType = aclState.AclType
	result.HwPresence = aclState.HwPresence
	result.HitCount = aclState.HitCount
	result.IntfList = aclState.IntfList

	return result, nil
}

func (aMgr *AclManager) GetBulkCoppStatState(start, count int) (end, listLen int, more bool, cStateList []*asicdServices.CoppStatState) {
	if !aMgr.initComplete {
		aMgr.logger.Err("Acl : Acl manager not initialized yet..")
		return -1, 0, false, nil
	}
	var numEntries = 0
	if (start > pluginCommon.MAX_COPP_CLASS_COUNT) || (start < 0) {
		aMgr.logger.Err("Invalid start CoPP argument during get bulk Copp state start", start,
			" Max CoPP classes ", pluginCommon.MAX_COPP_CLASS_COUNT)
		return -1, 0, false, nil
	}
	if count < 0 {
		aMgr.logger.Err("Invalid count during get bulk CoPP state")
		return -1, 0, false, nil
	}
	if count > pluginCommon.MAX_COPP_CLASS_COUNT {
		count = pluginCommon.MAX_COPP_CLASS_COUNT
	}
	for _, plugin := range aMgr.pMgr.plugins {
		if isControllingPlugin(plugin) == true {
			rv := plugin.UpdateCoppStatStateDB(0, count)
			if rv < 0 {
				aMgr.logger.Err("Failed to retrieve CoPP stats from the plugin ")
				return -1, 0, false, nil
			}
		}
	}

	for idx := 0; idx < pluginCommon.MAX_COPP_CLASS_COUNT; idx++ {
		coppClass := pluginCommon.CoppType[idx]
		stat := aMgr.CoPPMap[coppClass]
		cStateList = append(cStateList, stat)
		numEntries += 1
	}
	end = pluginCommon.MAX_COPP_CLASS_COUNT
	more = false
	return end, len(cStateList), more, cStateList
}

func (aMgr *AclManager) GetCoppStatState(proto string) (*asicdServices.CoppStatState, error) {
	if !aMgr.initComplete {
		aMgr.logger.Err("Acl : Acl manager not initialized yet..")
		return nil, errors.New("Acl manager is not initialized yet.")
	}
	stat := aMgr.CoPPMap[proto]
	return stat, nil

}

func UpdateCoppStatStateDB(cState *pluginCommon.CoppStatState) {

	if coppStat, ok := AclMgr.CoPPMap[cState.CoppClass]; ok {
		coppStat.Protocol = cState.CoppClass
		coppStat.PeakRate = cState.PeakRate
		coppStat.BurstRate = cState.BurstRate
		coppStat.GreenPackets = cState.GreenPackets
		coppStat.RedPackets = cState.RedPackets
		AclMgr.CoPPMap[cState.CoppClass] = coppStat
	}
}

func IPAddrStringToU8List(ipAddr string) ([]uint8, []uint8, int, error) {
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return ip, ip, -1, errors.New("Invalid IP Address")
	}
	ipv4 := ip.To4()
	if ipv4 != nil {
		//this is an ip address
		return ip, ipv4, syscall.AF_INET, nil
	}
	ipv6 := ip.To16()
	return ip, ipv6, syscall.AF_INET6, nil
}
