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
	"asicdServices"
	"errors"
	"fmt"
	"models/objects"
	"net"
	"strconv"
	"sync"
	"time"
	"utils/dbutils"
	"utils/logging"
)

const (
	//FIXME: Discuss and update value appropriately
	MAC_FLAP_THRESHOLD = 5.0 //num of times per sec
)

type macEntry struct {
	l2IfIndex           int32 //port or lag ifindex
	vlanId              int32
	flapCount           int
	ipAdj               bool
	timeOfEntryCreation time.Time
	timeOfLastUpdate    time.Time
}

type macEntryDB struct {
	macEntries map[string]*macEntry
}

type L2Manager struct {
	plugins            []PluginIntf
	logger             *logging.Writer
	dbHdl              *dbutils.DBUtil
	initComplete       bool
	macFlapDisablePort bool
	macDB              *macEntryDB
	dbMutex            sync.RWMutex
	nMgr               *NeighborManager
	nhMgr              *NextHopManager
	pMgr               *PortManager
}

// L2Mgr db
var L2Mgr L2Manager

func NewMacEntryDB() *macEntryDB {
	var m macEntryDB
	m.macEntries = make(map[string]*macEntry)
	return &m
}

func (lMgr *L2Manager) Init(rsrcMgrs *ResourceManagers, dbHdl *dbutils.DBUtil, pluginList []PluginIntf, logger *logging.Writer, macFlapDisablePort bool) {
	lMgr.plugins = pluginList
	lMgr.logger = logger
	lMgr.nMgr = rsrcMgrs.NeighborManager
	lMgr.nhMgr = rsrcMgrs.NextHopManager
	lMgr.pMgr = rsrcMgrs.PortManager
	lMgr.dbHdl = dbHdl
	lMgr.macFlapDisablePort = macFlapDisablePort
	lMgr.macDB = NewMacEntryDB()
	lMgr.initComplete = true
}

func (lMgr *L2Manager) Deinit() {
	/* Currently no-op */
}

func ConstructMacTableEntryStateKey(l2IfIndex, vlanId int32, macAddr string) string {
	return "mac" + "_" + strconv.Itoa(int(l2IfIndex)) + "_" + strconv.Itoa(int(vlanId)) + "_" + macAddr
}

func WriteMacTableEntryToDB(macAddr string, entry *macEntry) error {
	var dbObj objects.MacTableEntryState
	obj := asicdServices.NewMacTableEntryState()
	obj.Port = entry.l2IfIndex
	obj.VlanId = entry.vlanId
	obj.MacAddr = macAddr
	objects.ConvertThriftToasicdMacTableEntryStateObj(obj, &dbObj)
	err := L2Mgr.dbHdl.StoreObjectInDb(dbObj)
	if err != nil {
		return errors.New(fmt.Sprintln("Failed to add MacTableEntry to state db : ", entry))
	}
	return nil
}

func DelMacTableEntryFromDB(macAddr string, entry *macEntry) error {
	var dbObj objects.MacTableEntryState
	obj := asicdServices.NewMacTableEntryState()
	obj.Port = entry.l2IfIndex
	obj.VlanId = entry.vlanId
	obj.MacAddr = macAddr
	objects.ConvertThriftToasicdMacTableEntryStateObj(obj, &dbObj)
	err := L2Mgr.dbHdl.DeleteObjectFromDb(dbObj)
	if err != nil {
		return errors.New(fmt.Sprintln("Failed to delete MacTableEntry from state db : ", entry))
	}
	return nil
}

func AddMacTableEntryState(l2IfIndex, vlanId int32, macAddr net.HardwareAddr) {
	//Check if mac entry already exists
	if entry, ok := L2Mgr.macDB.macEntries[macAddr.String()]; ok {
		//Entry exists check for move, qualify based on vlan
		if entry.l2IfIndex != l2IfIndex && entry.vlanId == vlanId {
			//Mac has moved
			newEntry := entry
			newEntry.l2IfIndex = l2IfIndex
			newEntry.flapCount += 1
			newEntry.ipAdj = false
			newEntry.timeOfLastUpdate = time.Now()
			//Determine if flaprate exceeds threshold, if yes disable port
			duration := newEntry.timeOfLastUpdate.Sub(newEntry.timeOfEntryCreation).Seconds()
			if float64(newEntry.flapCount)/duration > MAC_FLAP_THRESHOLD {
				//Err disable port based on configuration
				if L2Mgr.macFlapDisablePort {
					L2Mgr.pMgr.ErrorDisablePort(l2IfIndex, "DOWN", fmt.Sprintln("Mac flap detected for mac, port - ", macAddr.String(), l2IfIndex))
				} else {
					L2Mgr.logger.Err("Mac flap detected for mac, port - ", macAddr.String(), l2IfIndex)
				}
			}
			L2Mgr.nMgr.ProcessMacMove(macAddr, l2IfIndex, vlanId)
		} else {
			//Not a mac move, just log callback info
			L2Mgr.logger.Info("UpdateMacDB : Call back received, no mac move detected - ", l2IfIndex, vlanId, macAddr.String())
		}
	} else {
		//Mac entry does not exist create it
		curTime := time.Now()
		L2Mgr.macDB.macEntries[macAddr.String()] = &macEntry{
			l2IfIndex:           l2IfIndex,
			vlanId:              vlanId,
			flapCount:           0,
			ipAdj:               false,
			timeOfEntryCreation: curTime,
			timeOfLastUpdate:    curTime,
		}
	}
	//Write entry to state DB
	ent := L2Mgr.macDB.macEntries[macAddr.String()]
	err := WriteMacTableEntryToDB(macAddr.String(), ent)
	if err != nil {
		L2Mgr.logger.Err(err.Error())
	}
}

func DelMacTableEntryState(l2IfIndex, vlanId int32, macAddr net.HardwareAddr) {
	//Check if mac entry exists
	if entry, ok := L2Mgr.macDB.macEntries[macAddr.String()]; ok {
		//Delete entry from state DB
		ent := L2Mgr.macDB.macEntries[macAddr.String()]
		err := DelMacTableEntryFromDB(macAddr.String(), ent)
		if err != nil {
			L2Mgr.logger.Err(err.Error())
		}
		if !L2Mgr.nMgr.HasIpAdj(macAddr.String()) {
			//Entry exists delete from sw cache
			delete(L2Mgr.macDB.macEntries, macAddr.String())
		} else {
			entry.ipAdj = true
		}
	}
}

func UpdateMacDB(op int, port, vlanId int32, macAddr net.HardwareAddr) {
	if L2Mgr.initComplete == true {
		//Only record mac entries learned on configured ports
		if vlanId != pluginCommon.DEFAULT_VLAN_ID {
			l2IfIndex, err := L2Mgr.pMgr.GetIfIndexFromPortNum(port)
			if err == nil {
				L2Mgr.dbMutex.Lock()
				if op == pluginCommon.MAC_ENTRY_LEARNED {
					AddMacTableEntryState(l2IfIndex, vlanId, macAddr)
				} else if op == pluginCommon.MAC_ENTRY_AGED {
					DelMacTableEntryState(l2IfIndex, vlanId, macAddr)
				}
				L2Mgr.dbMutex.Unlock()
			}
		}
	}
}

func (lMgr *L2Manager) RemoveMacIpAdjEntry(macAddr string) {
	lMgr.logger.Debug("RemoveMacIpAdjEntry called for macAddr:", macAddr)
	entry, ok := lMgr.macDB.macEntries[macAddr]
	// check if mac ---> ip exists
	if !ok {
		return
	}
	// check if ipAdj flag is set
	if !entry.ipAdj {
		return
	}
	lMgr.logger.Debug("Ip Adjacency was present for macAddr:", macAddr, "deleting it")
	// entry is still present as IPADJ even without any l2...
	delete(L2Mgr.macDB.macEntries, macAddr)
}

func (lMgr *L2Manager) GetMacTableEntryState(macAddr string) (*asicdServices.MacTableEntryState, error) {
	lMgr.dbMutex.RLock()
	defer lMgr.dbMutex.RUnlock()
	if entry, ok := lMgr.macDB.macEntries[macAddr]; ok {
		obj := &asicdServices.MacTableEntryState{
			MacAddr: macAddr,
			VlanId:  entry.vlanId,
			Port:    entry.l2IfIndex,
		}
		return obj, nil
	} else {
		return nil, errors.New(fmt.Sprintln("Mac table entry does not exist for mac - ", macAddr))
	}
}

//FIXME: The following code does not implement maintaining a list of keys to service get bulk requests.
func (lMgr *L2Manager) GetBulkMacTableEntryState(start, count int) (end, listLen int, more bool, objList []*asicdServices.MacTableEntryState) {
	var numEntries, idx int = 0, 0
	if len(lMgr.macDB.macEntries) == 0 {
		//No entries in sw cache
		return 0, 0, false, nil
	}
	lMgr.dbMutex.RLock()
	for macAddr, entry := range lMgr.macDB.macEntries {
		if idx >= start {
			objList = append(objList, &asicdServices.MacTableEntryState{
				MacAddr: macAddr,
				VlanId:  entry.vlanId,
				Port:    entry.l2IfIndex,
			})
			numEntries += 1
		}
		idx += 1
	}
	lMgr.dbMutex.RUnlock()
	if idx < len(lMgr.macDB.macEntries) {
		more = false
	} else {
		more = true
	}
	end = idx
	listLen = len(objList)
	return end, listLen, more, objList
}

func (lMgr *L2Manager) EnablePacketReception(macObj *asicdInt.RsvdProtocolMacConfig) (bool, error) {
	for _, plugin := range lMgr.plugins {
		retVal := plugin.AddProtocolMacEntry(macObj.MacAddr,
			macObj.MacAddrMask, macObj.VlanId)
		if retVal < 0 {
			return false, errors.New("Plugin failed to configure " +
				macObj.MacAddr)
		}
	}
	return true, nil
}

func (lMgr *L2Manager) DisablePacketReception(macObj *asicdInt.RsvdProtocolMacConfig) (bool, error) {
	for _, plugin := range lMgr.plugins {
		retVal := plugin.DeleteProtocolMacEntry(macObj.MacAddr,
			macObj.MacAddrMask, macObj.VlanId)
		if retVal < 0 {
			return false, errors.New("Plugin failed to configure " +
				macObj.MacAddr)
		}
	}
	return true, nil
}
