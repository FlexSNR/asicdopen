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
	"asicd/pluginManager/pluginCommon"
	"asicdInt"
	"errors"
	"fmt"
	"strconv"
	"utils/commonDefs"
	"utils/logging"
)

type IfManager struct {
	logger                  *logging.Writer
	ifNameToIfIndex         map[string]int32
	ifIndexToIfName         map[int32]string
	keyCache                []string
	nextIfIdForIfType       map[int]int
	freeIfIndexListByIfType map[int][]int32
}

var IfMgr IfManager

func (iMgr *IfManager) Init(logger *logging.Writer) {
	iMgr.logger = logger
	iMgr.nextIfIdForIfType = make(map[int]int)
	iMgr.ifNameToIfIndex = make(map[string]int32)
	iMgr.ifIndexToIfName = make(map[int32]string)
	iMgr.freeIfIndexListByIfType = make(map[int][]int32)
}

func (iMgr *IfManager) Deinit() {
	/* Currently no-op */
}

func (iMgr *IfManager) AllocateIfIndex(objName string, objId, objType int) int32 {
	var ifIndex int32
	switch objType {
	case commonDefs.IfTypeLag, commonDefs.IfTypeVlan:
		ifIndex = pluginCommon.GetIfIndexFromIdType(objId, objType)

	case commonDefs.IfTypePort, commonDefs.IfTypeLoopback, commonDefs.IfTypeVirtual, commonDefs.IfTypeVtep:
		listLen := len(iMgr.freeIfIndexListByIfType[objType])
		if listLen != 0 {
			//Pop off tail of free index slice
			ifIndex, iMgr.freeIfIndexListByIfType[objType] = iMgr.freeIfIndexListByIfType[objType][listLen-1], iMgr.freeIfIndexListByIfType[objType][:listLen-1]
		} else {
			if val, ok := iMgr.nextIfIdForIfType[objType]; !ok {
				ifIndex = pluginCommon.GetIfIndexFromIdType(0, objType)
				iMgr.nextIfIdForIfType[objType] = 1
			} else {
				ifIndex = pluginCommon.GetIfIndexFromIdType(val, objType)
				iMgr.nextIfIdForIfType[objType] += 1
			}
		}

	case commonDefs.IfTypeP2P, commonDefs.IfTypeBcast,
		commonDefs.IfTypeSecondary, commonDefs.IfTypeNull:
		//No allocation needed for these types currently
		return int32(0)
	}
	iMgr.logger.Debug(fmt.Sprintln("IfMgr allocated ifindex : ifindex, objname, objid, objtype", ifIndex, objName, objId, objType))
	//Insert record into cache
	iMgr.keyCache = append(iMgr.keyCache, objName)
	iMgr.ifNameToIfIndex[objName] = ifIndex
	iMgr.ifIndexToIfName[ifIndex] = objName
	return ifIndex
}

func (iMgr *IfManager) FreeIfIndex(ifIndex int32) {
	if _, ok := iMgr.ifIndexToIfName[ifIndex]; !ok {
		iMgr.logger.Err(fmt.Sprintln("FreeIfIndex call received for invalid ifindex - ", ifIndex))
	}

	objType := pluginCommon.GetTypeFromIfIndex(ifIndex)
	switch objType {
	case commonDefs.IfTypePort, commonDefs.IfTypeLag, commonDefs.IfTypeVlan:
		//no-op

	case commonDefs.IfTypeLoopback, commonDefs.IfTypeVirtual, commonDefs.IfTypeVtep:
		iMgr.freeIfIndexListByIfType[objType] = append(iMgr.freeIfIndexListByIfType[objType], ifIndex)

	case commonDefs.IfTypeP2P, commonDefs.IfTypeBcast,
		commonDefs.IfTypeSecondary, commonDefs.IfTypeNull:
		//no-op
		return
	}
	for idx := 0; idx < len(iMgr.keyCache); idx++ {
		if iMgr.keyCache[idx] == iMgr.ifIndexToIfName[ifIndex] {
			iMgr.keyCache = append(iMgr.keyCache[:idx], iMgr.keyCache[idx+1:]...)
			break
		}
	}
	//Delete record from cache
	delete(iMgr.ifNameToIfIndex, iMgr.ifIndexToIfName[ifIndex])
	delete(iMgr.ifIndexToIfName, ifIndex)
	iMgr.logger.Debug(fmt.Sprintln("IfMgr freed ifindex : ifindex", ifIndex))
	return
}

func (iMgr *IfManager) ConvertIntfStrListToIfIndexList(intfStrings []string) (ifIndexList []int32, err error) {
	for idx := 0; idx < len(intfStrings); idx++ {
		if val, err := strconv.Atoi(intfStrings[idx]); err == nil {
			//Verify ifIndex is valid
			if _, ok := iMgr.ifIndexToIfName[int32(val)]; !ok {
				return nil, errors.New(fmt.Sprintln("Invalid ifIndex value:", val))
			} else {
				ifIndexList = append(ifIndexList, int32(val))
			}
		} else {
			//Verify ifName is valid
			if ifIndex, ok := iMgr.ifNameToIfIndex[intfStrings[idx]]; !ok {
				return nil, errors.New(fmt.Sprintln("Invalid ifName value:", intfStrings[idx]))
			} else {
				ifIndexList = append(ifIndexList, ifIndex)
			}
		}
	}
	return ifIndexList, nil
}

func (iMgr *IfManager) UpdateIfNameForIfIndex(ifIndex int32, oldIfName, newIfName string) {
	iMgr.ifIndexToIfName[ifIndex] = newIfName
	delete(iMgr.ifNameToIfIndex, oldIfName)
	iMgr.ifNameToIfIndex[newIfName] = ifIndex
}
func (iMgr *IfManager) GetIfNameForIfIndex(ifIndex int32) (string, error) {
	ifName, ok := iMgr.ifIndexToIfName[ifIndex]
	if !ok {
		return ifName, errors.New("IfName not found")
	}
	return ifName, nil
}
func (iMgr *IfManager) GetIfIndexForIfName(ifName string) (int32, error) {
	ifIndex, ok := iMgr.ifNameToIfIndex[ifName]
	if !ok {
		return ifIndex, errors.New("IfIndex not found")
	}
	return ifIndex, nil
}
func (iMgr *IfManager) GetBulkIntf(start, count int) (end, listLen int, more bool, intfList []*asicdInt.Intf) {
	var numEntries, idx int = 0, 0
	if len(iMgr.keyCache) == 0 {
		return 0, 0, false, nil
	}
	for idx = start; idx < len(iMgr.keyCache); idx++ {
		if numEntries == count {
			more = true
			break
		}
		intfList = append(intfList, &asicdInt.Intf{
			IfName:  iMgr.keyCache[idx],
			IfIndex: iMgr.ifNameToIfIndex[iMgr.keyCache[idx]],
		})
		numEntries += 1
	}
	end = idx
	if idx == len(iMgr.keyCache) {
		more = false
	}
	return end, len(intfList), more, intfList
}
