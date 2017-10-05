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
	"encoding/json"
	"errors"
	"fmt"
	"models/events"
	"models/objects"
	"strconv"
	"sync"
	"utils/commonDefs"
	"utils/dbutils"
	"utils/eventUtils"
	"utils/logging"
)

const (
	OPER_STATE_DOWN_DUE_TO_ADMIN_STATE_DOWN = "Interface oper state is DOWN due to Admin state being DOWN"
)

type vlanInfo struct {
	adminState       string
	operState        string
	vlanId           int
	ifIndex          int32
	vlanName         string
	activeIntfCount  int
	ifIndexList      []int32
	untagIfIndexList []int32
}

type VlanManager struct {
	dbMutex        sync.RWMutex
	ifMgr          *IfManager
	portMgr        *PortManager
	lagMgr         *LagManager
	l3IntfMgr      *L3IntfManager
	sysRsvdVlanMin int
	sysRsvdVlanMax int
	vlanDB         [pluginCommon.MAX_VLAN_ID]*vlanInfo
	plugins        []PluginIntf
	logger         *logging.Writer
	notifyChannel  chan<- []byte
}

//Vlan manager db
var VlanMgr VlanManager

func GetPortLagIdsFromVlanIfIndexList(ifIndexList []int32) (ports []int32, lags []int32) {
	for _, ifIndex := range ifIndexList {
		ifType := pluginCommon.GetTypeFromIfIndex(ifIndex)
		switch ifType {
		case commonDefs.IfTypePort:
			ports = append(ports, ifIndex)
		case commonDefs.IfTypeLag:
			lags = append(lags, ifIndex)
		}
	}
	return ports, lags
}

func (vMgr *VlanManager) Init(rsrcMgrs *ResourceManagers, dbHdl *dbutils.DBUtil, bootMode, rsvdVlanMin, rsvdVlanMax int, pluginList []PluginIntf, logger *logging.Writer, notifyChannel chan<- []byte) {
	vMgr.ifMgr = rsrcMgrs.IfManager
	vMgr.portMgr = rsrcMgrs.PortManager
	vMgr.lagMgr = rsrcMgrs.LagManager
	vMgr.l3IntfMgr = rsrcMgrs.L3IntfManager
	vMgr.logger = logger
	vMgr.plugins = pluginList
	vMgr.notifyChannel = notifyChannel

	if (rsvdVlanMin < 0) || (rsvdVlanMax < 0) ||
		(rsvdVlanMin > pluginCommon.MAX_VLAN_ID) || (rsvdVlanMax > pluginCommon.MAX_VLAN_ID) ||
		(rsvdVlanMin == pluginCommon.DEFAULT_VLAN_ID) || (rsvdVlanMax == pluginCommon.DEFAULT_VLAN_ID) {
		vMgr.logger.Err("Invalid limits specified for system reserved vlans. Overriding using default range (3835 - 4090)")
		vMgr.sysRsvdVlanMin = pluginCommon.SYS_RSVD_VLAN_MIN
		vMgr.sysRsvdVlanMax = pluginCommon.SYS_RSVD_VLAN_MAX
	} else {
		vMgr.sysRsvdVlanMin = rsvdVlanMin
		vMgr.sysRsvdVlanMax = rsvdVlanMax
	}

	if dbHdl != nil {
		var dbObj objects.Vlan
		objList, err := dbHdl.GetAllObjFromDb(dbObj)
		if err != nil {
			logger.Err(fmt.Sprintln("DB Query failed during Vlan Init, Error:", err))
			logger.Err(fmt.Sprintln("object list is", objList))
			return
		}
		for idx := 0; idx < len(objList); idx++ {
			obj := asicdServices.NewVlan()
			dbObject := objList[idx].(objects.Vlan)
			objects.ConvertasicdVlanObjToThrift(&dbObject, obj)
			rv, _, _ := vMgr.CreateVlan(obj)
			if rv < 0 {
				logger.Err("Vlan create failed during Vlan Init")
				return
			}
		}
	}
}

func (vMgr *VlanManager) Deinit() {
	var plugin PluginIntf
	/* Cleanup all SVI's created by linux plugin */
	for _, plugin = range vMgr.plugins {
		if isPluginAsicDriver(plugin) == false {
			break
		}
	}
	vMgr.dbMutex.RLock()
	for vlanId, info := range vMgr.vlanDB {
		rsvd_vid := false
		if info != nil {
			if vlanId >= vMgr.sysRsvdVlanMin && vlanId <= vMgr.sysRsvdVlanMax {
				rsvd_vid = true
			}
			//FIXME : Construct list containing port num and lag ifindex
			portList := vMgr.ConvertIfIndexListToPortList(info.ifIndexList)
			untagPortList := vMgr.ConvertIfIndexListToPortList(info.untagIfIndexList)
			rv := plugin.DeleteVlan(rsvd_vid, vlanId, portList, untagPortList)
			if rv < 0 {
				vMgr.logger.Err("Failed to cleanup vlan interfaces")
			}
		}
	}
	vMgr.dbMutex.RUnlock()
}

func (vMgr *VlanManager) ConvertIfIndexListToPortList(ifIndexList []int32) []int32 {
	var portList []int32
	for _, ifIndex := range ifIndexList {
		ifType := pluginCommon.GetTypeFromIfIndex(ifIndex)
		switch ifType {
		case commonDefs.IfTypePort:
			port := vMgr.portMgr.GetPortNumFromIfIndex(ifIndex)
			portList = append(portList, port)
		case commonDefs.IfTypeLag:
			//Get LAG members
			var pList []int32
			portIfIndexList := vMgr.lagMgr.GetLagMembers(ifIndex)
			for _, val := range portIfIndexList {
				port := vMgr.portMgr.GetPortNumFromIfIndex(val)
				pList = append(pList, port)
			}
			portList = append(portList, pList...)
		}
	}
	return portList
}

func (vMgr *VlanManager) GetNumVlans() int {
	var cnt int
	vMgr.dbMutex.RLock()
	defer vMgr.dbMutex.RUnlock()
	for _, val := range vMgr.vlanDB {
		if val != nil {
			cnt++
		}
	}
	return cnt
}

func (vMgr *VlanManager) publishVlanCfgChgNotification(msgType, vlanId int, vlanIfIndex int32, ifIndexList, untagIfIndexList []int32) {
	msg := pluginCommon.VlanNotifyMsg{
		VlanId:      uint16(vlanId),
		VlanIfIndex: vlanIfIndex,
		VlanName:    pluginCommon.SVI_PREFIX + strconv.Itoa(vlanId),
		TagPorts:    ifIndexList,
		UntagPorts:  untagIfIndexList,
	}
	msgBuf, err := json.Marshal(msg)
	if err != nil {
		vMgr.logger.Err("Failed to marshal vlan create/delete message")
	}
	notification := pluginCommon.AsicdNotification{
		MsgType: uint8(msgType),
		Msg:     msgBuf,
	}
	notificationBuf, err := json.Marshal(notification)
	if err != nil {
		vMgr.logger.Err("Failed to marshal vlan create/delete message")
	}
	vMgr.notifyChannel <- notificationBuf
}

func (vMgr *VlanManager) parseIntfStrListToIfIndexList(intfList []string) ([]int32, error) {
	var list []int32
	for idx := 0; idx < len(intfList); idx++ {
		intfListStr, err := ParseUsrStrToList(intfList[idx])
		if err != nil {
			return nil, errors.New("Failed to parse intf list string during CreateVlan")
		}
		ifIndexList, err := vMgr.ifMgr.ConvertIntfStrListToIfIndexList(intfListStr)
		if err != nil {
			return nil, errors.New("Failed to map int list string to ifindex list")
		}
		list = append(list, ifIndexList...)
	}
	return list, nil
}

func (vMgr *VlanManager) AcceptUntagIfIndexConfig(vlanId int, untagIfIndexList []int32) bool {
	var rv bool = true
	for _, ifIndex := range untagIfIndexList {
		pvid := vMgr.portMgr.GetPvidForIfIndex(ifIndex)
		if pvid != pluginCommon.DEFAULT_VLAN_ID && pvid != vlanId {
			rv = false
			break
		}
	}
	return rv
}

func (vMgr *VlanManager) ValidatePortConfigMode(ifIndexList, untagIfIndexList []int32) bool {
	var ok bool = true
	for _, ifIndex := range append(ifIndexList, untagIfIndexList...) {
		if commonDefs.IfTypePort == pluginCommon.GetTypeFromIfIndex(ifIndex) {
			if vMgr.portMgr.GetPortConfigMode(ifIndex) == pluginCommon.PORT_MODE_L3 ||
				vMgr.portMgr.GetPortConfigMode(ifIndex) == pluginCommon.PORT_MODE_INTERNAL {
				ok = false
				break
			}
		}
	}
	return ok
}

func (vMgr *VlanManager) ValidateVlanConfig(vlanId int, ifIndexList, untagIfIndexList []int32) (bool, error) {
	var err error
	ok := vMgr.AcceptUntagIfIndexConfig(vlanId, untagIfIndexList)
	if ok {
		ok = vMgr.ValidatePortConfigMode(ifIndexList, untagIfIndexList)
		if !ok {
			err = errors.New("Please remove all L3 configuration on member ports before creating vlan")
		}
	} else {
		err = errors.New("UntagIntfList, contains interfaces that are already untagged members of other vlans")
	}
	return ok, err
}

func (vMgr *VlanManager) CreateLagVlan(vlanObj *asicdServices.Vlan) (bool, error) {
	var vlanId int = int(vlanObj.VlanId)
	ifIndexList, _ := vMgr.parseIntfStrListToIfIndexList(vlanObj.IntfList)
	untagIfIndexList, _ := vMgr.parseIntfStrListToIfIndexList(vlanObj.UntagIntfList)

	portList := vMgr.ConvertIfIndexListToPortList(ifIndexList)
	untagPortList := vMgr.ConvertIfIndexListToPortList(untagIfIndexList)
	if false == arePortListsMutuallyExclusive(portList, untagPortList) {
		return false, errors.New("Failed to create LAG vlan. IntfList, UntagIntfList need to contain mutually exclusive port lists")
	}

	//Set pvid for all untagged ports
	vMgr.portMgr.SetPvidForIfIndexList(vlanId, untagIfIndexList)
	//Set port mode to L2 only if this is not an internal vlan
	if !((vlanId >= vMgr.sysRsvdVlanMin) && (vlanId <= vMgr.sysRsvdVlanMax)) {
		vMgr.portMgr.SetPortConfigMode(pluginCommon.PORT_MODE_L2, append(ifIndexList, untagIfIndexList...))
	}
	//Create list of ports, lags from ifindex list
	ports, lags := GetPortLagIdsFromVlanIfIndexList(ifIndexList)
	activePortCount := vMgr.portMgr.GetActiveIntfCountFromIfIndexList(ports)
	activeLagCount := vMgr.lagMgr.GetActiveLagCountFromLagList(lags)
	ports, lags = GetPortLagIdsFromVlanIfIndexList(untagIfIndexList)
	activePortCount += vMgr.portMgr.GetActiveIntfCountFromIfIndexList(ports)
	activeLagCount += vMgr.lagMgr.GetActiveLagCountFromLagList(lags)
	//Update SW cache
	vlanName := pluginCommon.SVI_PREFIX + strconv.Itoa(vlanId)
	vlanIfIndex := vMgr.ifMgr.AllocateIfIndex(vlanName, vlanId, commonDefs.IfTypeVlan)
	var state string
	if (activePortCount+activeLagCount > 0) && (vlanObj.AdminState != INTF_STATE_DOWN) {
		state = INTF_STATE_UP
	} else {
		state = INTF_STATE_DOWN
	}
	vMgr.dbMutex.Lock()
	vMgr.vlanDB[vlanId] = &vlanInfo{
		adminState:       vlanObj.AdminState,
		operState:        state,
		vlanId:           vlanId,
		ifIndex:          vlanIfIndex,
		vlanName:         vlanName,
		activeIntfCount:  activePortCount + activeLagCount,
		ifIndexList:      ifIndexList,
		untagIfIndexList: untagIfIndexList,
	}
	vMgr.dbMutex.Unlock()
	//Publish notification - ARPd listens to vlan create notification
	vMgr.publishVlanCfgChgNotification(pluginCommon.NOTIFY_VLAN_CREATE, vlanId, vlanIfIndex, ifIndexList, untagIfIndexList)
	return true, nil
}

func (vMgr *VlanManager) CreateVlan(vlanObj *asicdServices.Vlan) (int, bool, error) {
	var vlanId int = int(vlanObj.VlanId)
	if (vlanId < pluginCommon.SYS_RSVD_VLAN) || (vlanId > pluginCommon.MAX_VLAN_ID) ||
		((vlanId > vMgr.sysRsvdVlanMin) && (vlanId < vMgr.sysRsvdVlanMax)) {
		return -1, false, errors.New("Invalid vlan id specified during vlan create)")
	}
	if (vlanObj.AdminState != INTF_STATE_UP) && (vlanObj.AdminState != INTF_STATE_DOWN) {
		return -1, false, errors.New("Invalid AdminState value specified during CreateVlan")
	}
	ifIndexList, err := vMgr.parseIntfStrListToIfIndexList(vlanObj.IntfList)
	if err != nil {
		return -1, false, errors.New("Failed to map intf list string to ifindex list")
	}
	untagIfIndexList, err := vMgr.parseIntfStrListToIfIndexList(vlanObj.UntagIntfList)
	if err != nil {
		return -1, false, errors.New("Failed to map untagged intf list string to ifindex list")
	}
	rsvd_vid := false
	if vlanId == pluginCommon.SYS_RSVD_VLAN {
		rsvd_vid = true
		vlanId = vMgr.GetNextAvailableSysVlan()
		if vlanId == -1 {
			return -1, false, errors.New("No system reserved vlan available")
		}
	}
	//Convert ifindex list and untag ifindex list into corresponding port lists
	portList := vMgr.ConvertIfIndexListToPortList(ifIndexList)
	untagPortList := vMgr.ConvertIfIndexListToPortList(untagIfIndexList)
	if false == arePortListsMutuallyExclusive(portList, untagPortList) {
		return -1, false, errors.New("Failed to create vlan. IntfList, UntagIntfList need to contain mutually exclusive port lists")
	}
	ok, err := vMgr.ValidateVlanConfig(vlanId, ifIndexList, untagIfIndexList)
	if !ok {
		return -1, false, err
	}
	//Update HW state
	for _, plugin := range vMgr.plugins {
		//Asic plugin needs port list, while linux plugin needs ifIndex list
		var list, untagList []int32
		//Add ports to vlan only if vlan interface adminstate is set to up
		if vlanObj.AdminState == INTF_STATE_UP {
			if isControllingPlugin(plugin) {
				list = portList
				untagList = untagPortList
			} else {
				list = ifIndexList
				untagList = untagIfIndexList
			}
		}
		rv := plugin.CreateVlan(rsvd_vid, vlanId, list, untagList)
		if rv < 0 {
			vMgr.logger.Err(fmt.Sprintln("Failed to create vlan - ", vlanId, list, untagList))
			return -1, false, errors.New("Plugin failed to create vlan")
		}
	}
	//Set pvid for all untagged ports
	vMgr.portMgr.SetPvidForIfIndexList(vlanId, untagIfIndexList)
	//Set port mode to L2 only if this is not an internal vlan
	if !((vlanId >= vMgr.sysRsvdVlanMin) && (vlanId <= vMgr.sysRsvdVlanMax)) {
		vMgr.portMgr.SetPortConfigMode(pluginCommon.PORT_MODE_L2, append(ifIndexList, untagIfIndexList...))
	}
	//Create list of ports, lags from ifindex list
	ports, lags := GetPortLagIdsFromVlanIfIndexList(ifIndexList)
	activePortCount := vMgr.portMgr.GetActiveIntfCountFromIfIndexList(ports)
	activeLagCount := vMgr.lagMgr.GetActiveLagCountFromLagList(lags)
	ports, lags = GetPortLagIdsFromVlanIfIndexList(untagIfIndexList)
	activePortCount += vMgr.portMgr.GetActiveIntfCountFromIfIndexList(ports)
	activeLagCount += vMgr.lagMgr.GetActiveLagCountFromLagList(lags)
	//Update SW cache
	vlanName := pluginCommon.SVI_PREFIX + strconv.Itoa(vlanId)
	vlanIfIndex := vMgr.ifMgr.AllocateIfIndex(vlanName, vlanId, commonDefs.IfTypeVlan)
	var state string
	if (activePortCount+activeLagCount > 0) && (vlanObj.AdminState != INTF_STATE_DOWN) {
		state = INTF_STATE_UP
	} else {
		state = INTF_STATE_DOWN
	}
	vMgr.dbMutex.Lock()
	vMgr.vlanDB[vlanId] = &vlanInfo{
		adminState:       vlanObj.AdminState,
		operState:        state,
		vlanId:           vlanId,
		ifIndex:          vlanIfIndex,
		vlanName:         vlanName,
		activeIntfCount:  activePortCount + activeLagCount,
		ifIndexList:      ifIndexList,
		untagIfIndexList: untagIfIndexList,
	}
	vMgr.dbMutex.Unlock()
	if rsvd_vid == true {
		return vlanId, true, nil
	}
	//Publish notification - ARPd listens to vlan create notification
	vMgr.publishVlanCfgChgNotification(pluginCommon.NOTIFY_VLAN_CREATE, vlanId, vlanIfIndex, ifIndexList, untagIfIndexList)
	return vlanId, true, nil
}

func (vMgr *VlanManager) DeleteLagVlan(vlanObj *asicdServices.Vlan) (bool, error) {
	var vlanId int = int(vlanObj.VlanId)
	vMgr.logger.Info("DeleteLagVlan Vlan id for delete = " + strconv.Itoa(vlanId))
	if (vlanId < pluginCommon.SYS_RSVD_VLAN) || (vlanId > pluginCommon.MAX_VLAN_ID) ||
		((vlanId > vMgr.sysRsvdVlanMin) && (vlanId < vMgr.sysRsvdVlanMax)) {
		return false, errors.New("Invalid vlan id specified during vlan delete")
	}
	ifIndexList, err := vMgr.parseIntfStrListToIfIndexList(vlanObj.IntfList)
	if err != nil {
		return false, errors.New("Failed to map intf list string to ifindex list")
	}
	untagIfIndexList, err := vMgr.parseIntfStrListToIfIndexList(vlanObj.UntagIntfList)
	if err != nil {
		return false, errors.New("Failed to map untagged intf list string to ifindex list")
	}
	rsvd_vid := false
	if vlanId == pluginCommon.SYS_RSVD_VLAN {
		//Lookup system reserved vlans
		rsvd_vid = true
		vlanId = vMgr.GetSysVlanContainingIf(untagIfIndexList[0])
		if vlanId == -1 {
			return false, errors.New("Unable to locate system reserved vlan that contains port")
		}
	}
	//Update SW cache
	vMgr.dbMutex.Lock()
	vlanIfIndex := vMgr.vlanDB[vlanId].ifIndex
	vMgr.vlanDB[vlanId] = nil
	vMgr.dbMutex.Unlock()
	//Update pvid for untagged ports
	vMgr.portMgr.SetPvidForIfIndexList(pluginCommon.DEFAULT_VLAN_ID, untagIfIndexList)
	//Restore port mode only if this is not an internal vlan
	if !((vlanId >= vMgr.sysRsvdVlanMin) && (vlanId <= vMgr.sysRsvdVlanMax)) {
		vMgr.portMgr.SetPortConfigMode(pluginCommon.PORT_MODE_UNCONFIGURED, append(ifIndexList, untagIfIndexList...))
	}
	//Publish notification - ARPd listens to vlan delete notification
	if rsvd_vid == true {
		return true, nil
	}
	vMgr.publishVlanCfgChgNotification(pluginCommon.NOTIFY_VLAN_DELETE, vlanId, vlanIfIndex, ifIndexList, untagIfIndexList)
	return true, nil
}

func (vMgr *VlanManager) DeleteVlan(vlanObj *asicdServices.Vlan) (int, bool, error) {
	var vlanId int = int(vlanObj.VlanId)
	vMgr.logger.Info("DeleteVlan Vlan id for delete = " + strconv.Itoa(vlanId))
	if (vlanId < pluginCommon.SYS_RSVD_VLAN) || (vlanId > pluginCommon.MAX_VLAN_ID) ||
		((vlanId > vMgr.sysRsvdVlanMin) && (vlanId < vMgr.sysRsvdVlanMax)) {
		return -1, false, errors.New("Invalid vlan id specified during vlan delete")
	}
	ifIndexList, err := vMgr.parseIntfStrListToIfIndexList(vlanObj.IntfList)
	if err != nil {
		return -1, false, errors.New("Failed to map intf list string to ifindex list")
	}
	untagIfIndexList, err := vMgr.parseIntfStrListToIfIndexList(vlanObj.UntagIntfList)
	if err != nil {
		return -1, false, errors.New("Failed to map untagged intf list string to ifindex list")
	}
	rsvd_vid := false
	if vlanId == pluginCommon.SYS_RSVD_VLAN {
		//Lookup system reserved vlans
		rsvd_vid = true
		vlanId = vMgr.GetSysVlanContainingIf(untagIfIndexList[0])
		if vlanId == -1 {
			return -1, false, errors.New("Unable to locate system reserved vlan that contains port")
		}
	}
	//Convert ifindex list and untag ifindex list into corresponding port lists
	portList := vMgr.ConvertIfIndexListToPortList(ifIndexList)
	untagPortList := vMgr.ConvertIfIndexListToPortList(untagIfIndexList)
	//Update HW state
	for _, plugin := range vMgr.plugins {
		//Asic plugin needs port list, while linux plugin needs ifIndex list
		var list, untagList []int32
		if isControllingPlugin(plugin) {
			list = portList
			untagList = untagPortList
		} else {
			list = ifIndexList
			untagList = untagIfIndexList
		}
		vMgr.logger.Info("Delete vlan " + strconv.Itoa(vlanId) + "rsvd_id:" + strconv.FormatBool(rsvd_vid))
		rv := plugin.DeleteVlan(rsvd_vid, vlanId, list, untagList)
		if rv < 0 {
			return -1, false, errors.New("Plugin failed to delete vlan")
		}
	}
	//Update SW cache
	vMgr.dbMutex.Lock()
	vlanIfIndex := vMgr.vlanDB[vlanId].ifIndex
	vMgr.ifMgr.FreeIfIndex(vMgr.vlanDB[vlanId].ifIndex)
	vMgr.vlanDB[vlanId] = nil
	vMgr.dbMutex.Unlock()
	//Update pvid for untagged ports
	vMgr.portMgr.SetPvidForIfIndexList(pluginCommon.DEFAULT_VLAN_ID, untagIfIndexList)
	//Restore port mode only if this is not an internal vlan
	if !((vlanId >= vMgr.sysRsvdVlanMin) && (vlanId <= vMgr.sysRsvdVlanMax)) {
		vMgr.portMgr.SetPortConfigMode(pluginCommon.PORT_MODE_UNCONFIGURED, append(ifIndexList, untagIfIndexList...))
	}
	//Publish notification - ARPd listens to vlan delete notification
	if rsvd_vid == true {
		return vlanId, true, nil
	}
	vMgr.publishVlanCfgChgNotification(pluginCommon.NOTIFY_VLAN_DELETE, vlanId, vlanIfIndex, ifIndexList, untagIfIndexList)
	return vlanId, true, nil
}
func (vMgr *VlanManager) UpdateVlanForPatchUpdate(oldVlanObj, newVlanObj *asicdServices.Vlan, op []*asicdServices.PatchOpInfo) (bool, error) {
	vMgr.logger.Debug("patch update vlan")
	var vlanId int = int(oldVlanObj.VlanId)
	if (vlanId < pluginCommon.SYS_RSVD_VLAN) || (vlanId > pluginCommon.MAX_VLAN_ID) ||
		((vlanId > vMgr.sysRsvdVlanMin) && (vlanId < vMgr.sysRsvdVlanMax)) {
		return false, errors.New("Attempting to update vlan membership info for invalid vlan")
	}
	for idx := 0; idx < len(op); idx++ {
		switch op[idx].Path {
		case "IntfList":
			intfList := make([]string, 0)
			vMgr.logger.Debug("Patch update for IntfList")
			var valueObjArr []string
			err := json.Unmarshal([]byte(op[idx].Value), &valueObjArr)
			if err != nil {
				vMgr.logger.Debug("error unmarshaling value:", err)
				return false, errors.New(fmt.Sprintln("error unmarshaling value:", err))
			}
			vMgr.logger.Debug("Number of intfs:", len(valueObjArr))
			for _, val := range valueObjArr {
				vMgr.logger.Debug("intf: ", val)
				intfList = append(intfList, val)
			}
			switch op[idx].Op {
			case "add":
				newVlanObj.IntfList = append(newVlanObj.IntfList, oldVlanObj.IntfList[0:]...)
				newVlanObj.IntfList = append(newVlanObj.IntfList, intfList[0:]...)
			case "remove":
				delPortMap := make(map[string]bool)
				for _, intf := range intfList {
					delPortMap[intf] = true
				}
				newVlanObj.IntfList = make([]string, 0)
				for _, curr := range oldVlanObj.IntfList {
					if delPortMap[curr] != true {
						newVlanObj.IntfList = append(newVlanObj.IntfList, curr)
					}
				}
			default:
				vMgr.logger.Err("Operation ", op[idx].Op, " not supported")
				return false, errors.New(fmt.Sprintln("Operation ", op[idx].Op, " not supported"))
			}
		case "UntagIntfList":
			intfList := make([]string, 0)
			vMgr.logger.Debug("Patch update for UntagIntfList")
			var valueObjArr []string
			err := json.Unmarshal([]byte(op[idx].Value), &valueObjArr)
			if err != nil {
				vMgr.logger.Debug("error unmarshaling value:", err)
				return false, errors.New(fmt.Sprintln("error unmarshaling value:", err))
			}
			vMgr.logger.Debug("Number of untagged intfs:", len(valueObjArr))
			for _, val := range valueObjArr {
				vMgr.logger.Debug("intf: ", val)
				intfList = append(intfList, val)
			}
			switch op[idx].Op {
			case "add":
				newVlanObj.UntagIntfList = append(newVlanObj.UntagIntfList, oldVlanObj.UntagIntfList[0:]...)
				newVlanObj.UntagIntfList = append(newVlanObj.UntagIntfList, intfList[0:]...)
			case "remove":
				delPortMap := make(map[string]bool)
				for _, intf := range intfList {
					delPortMap[intf] = true
				}
				newVlanObj.UntagIntfList = make([]string, 0)
				for _, curr := range oldVlanObj.UntagIntfList {
					if delPortMap[curr] != true {
						newVlanObj.UntagIntfList = append(newVlanObj.UntagIntfList, curr)
					}
				}
			default:
				vMgr.logger.Err("Operation ", op[idx].Op, " not supported")
				return false, errors.New(fmt.Sprintln("Operation ", op[idx].Op, " not supported"))
			}
		default:
			vMgr.logger.Err("Patch update for attribute:", op[idx].Path, " not supported")
			return false, errors.New(fmt.Sprintln("Operation ", op[idx].Op, " not supported"))
		}
	}
	val, err := vMgr.UpdateVlan(oldVlanObj, newVlanObj)
	return val, err
}
func (vMgr *VlanManager) UpdateVlan(oldVlanObj, newVlanObj *asicdServices.Vlan) (bool, error) {
	vMgr.logger.Debug("UpdateVlan: oldVlanObj:", oldVlanObj, " newVlanObj:", newVlanObj)
	var vlanId int = int(oldVlanObj.VlanId)
	if (vlanId < pluginCommon.SYS_RSVD_VLAN) || (vlanId > pluginCommon.MAX_VLAN_ID) ||
		((vlanId > vMgr.sysRsvdVlanMin) && (vlanId < vMgr.sysRsvdVlanMax)) {
		return false, errors.New("Attempting to update vlan membership info for invalid vlan")
	}
	if (newVlanObj.AdminState != INTF_STATE_UP) && (newVlanObj.AdminState != INTF_STATE_DOWN) {
		return false, errors.New("Invalid AdminState value specified during UpdateVlan")
	}
	oldIfIndexList, err := vMgr.parseIntfStrListToIfIndexList(oldVlanObj.IntfList)
	if err != nil {
		return false, errors.New("Failed to map intf list string to ifindex list")
	}
	oldUntagIfIndexList, err := vMgr.parseIntfStrListToIfIndexList(oldVlanObj.UntagIntfList)
	if err != nil {
		return false, errors.New("Failed to map untagged intf list string to ifindex list")
	}
	newIfIndexList, err := vMgr.parseIntfStrListToIfIndexList(newVlanObj.IntfList)
	if err != nil {
		return false, errors.New("Failed to map intf list string to ifindex list")
	}
	newUntagIfIndexList, err := vMgr.parseIntfStrListToIfIndexList(newVlanObj.UntagIntfList)
	if err != nil {
		return false, errors.New("Failed to map untagged intf list string to ifindex list")
	}
	//Convert ifindex list and untag ifindex list into corresponding port lists
	oldPortList := vMgr.ConvertIfIndexListToPortList(oldIfIndexList)
	oldUntagPortList := vMgr.ConvertIfIndexListToPortList(oldUntagIfIndexList)
	newPortList := vMgr.ConvertIfIndexListToPortList(newIfIndexList)
	newUntagPortList := vMgr.ConvertIfIndexListToPortList(newUntagIfIndexList)
	//verify if new tag/untagged port lists are mutually exclusive
	if false == arePortListsMutuallyExclusive(newPortList, newUntagPortList) {
		return false, errors.New("Failed to update vlan. IfIndexList, UntagIfIndexList need to contain mutually exclusive port lists")
	}
	ok, err := vMgr.ValidateVlanConfig(vlanId, newIfIndexList, newUntagIfIndexList)
	if !ok {
		return false, err
	}
	//Update HW state
	for _, plugin := range vMgr.plugins {
		//Asic plugin needs port list, while linux plugin needs ifIndex list
		var oldList, oldUntagList, newList, newUntagList []int32
		if isControllingPlugin(plugin) {
			oldList = oldPortList
			oldUntagList = oldUntagPortList
			newList = newPortList
			newUntagList = newUntagPortList
		} else {
			oldList = oldIfIndexList
			oldUntagList = oldUntagIfIndexList
			newList = newIfIndexList
			newUntagList = newUntagIfIndexList
		}
		//Old port list should be empty if old admin state is down
		if oldVlanObj.AdminState == INTF_STATE_DOWN {
			oldList = nil
			oldUntagList = nil
		}
		//Do not add ports to vlan if admin state is down
		if newVlanObj.AdminState == INTF_STATE_DOWN {
			newList = nil
			newUntagList = nil
		}
		rv := plugin.UpdateVlan(vlanId, oldList, oldUntagList, newList, newUntagList)
		if rv < 0 {
			return false, errors.New("Plugin failed to update vlan")
		}
	}
	//Update pvid for untagged ports
	vMgr.portMgr.SetPvidForIfIndexList(vlanId, newUntagIfIndexList)
	vMgr.portMgr.SetPvidForIfIndexList(pluginCommon.DEFAULT_VLAN_ID, oldUntagIfIndexList)
	//Set port mode to L2 only if this is not an internal vlan
	if !((vlanId >= vMgr.sysRsvdVlanMin) && (vlanId <= vMgr.sysRsvdVlanMax)) {
		vMgr.portMgr.SetPortConfigMode(pluginCommon.PORT_MODE_UNCONFIGURED, append(oldIfIndexList, oldUntagIfIndexList...))
		vMgr.portMgr.SetPortConfigMode(pluginCommon.PORT_MODE_L2, append(newIfIndexList, newUntagIfIndexList...))
	}
	//Create list of ports, lags from ifindex list
	ports, lags := GetPortLagIdsFromVlanIfIndexList(newIfIndexList)
	activePortCount := vMgr.portMgr.GetActiveIntfCountFromIfIndexList(ports)
	activeLagCount := vMgr.lagMgr.GetActiveLagCountFromLagList(lags)
	ports, lags = GetPortLagIdsFromVlanIfIndexList(newUntagIfIndexList)
	activePortCount += vMgr.portMgr.GetActiveIntfCountFromIfIndexList(ports)
	activeLagCount += vMgr.lagMgr.GetActiveLagCountFromLagList(lags)
	var state string
	if (activePortCount+activeLagCount > 0) && (newVlanObj.AdminState == INTF_STATE_UP) {
		state = INTF_STATE_UP
	} else {
		state = INTF_STATE_DOWN
	}
	//Update SW cache
	vMgr.dbMutex.Lock()
	vlanIfIndex := vMgr.vlanDB[vlanId].ifIndex
	vMgr.vlanDB[vlanId] = &vlanInfo{
		adminState:       newVlanObj.AdminState,
		operState:        state,
		vlanId:           vlanId,
		ifIndex:          vMgr.vlanDB[vlanId].ifIndex,
		vlanName:         vMgr.vlanDB[vlanId].vlanName,
		activeIntfCount:  activePortCount + activeLagCount,
		ifIndexList:      newIfIndexList,
		untagIfIndexList: newUntagIfIndexList,
	}
	vMgr.dbMutex.Unlock()
	//Publish notification - ARPd listens to vlan update notification
	vMgr.publishVlanCfgChgNotification(pluginCommon.NOTIFY_VLAN_UPDATE, vlanId, vlanIfIndex, newIfIndexList, newUntagIfIndexList)
	//Have L3 determine if there is a state change if an IP is configured on this vlan
	var plState int = pluginCommon.INTF_STATE_UP
	if state == INTF_STATE_DOWN {
		plState = pluginCommon.INTF_STATE_DOWN
	}
	vMgr.l3IntfMgr.L3IntfProcessL2IntfStateChange(vlanIfIndex, plState)
	return true, nil
}

func (vMgr *VlanManager) GetBulkVlan(start, count int) (end, listLen int, more bool, vlanList []*asicdInt.Vlan) {
	var numEntries, idx int = 0, 0
	if (start < 0) || (start >= pluginCommon.MAX_VLAN_ID) {
		vMgr.logger.Err("Invalid start index argument in get bulk vlan")
		return -1, 0, false, nil
	}
	if count < 0 {
		vMgr.logger.Err("Invalid count in get bulk vlan")
		return -1, 0, false, nil
	}
	vMgr.dbMutex.RLock()
	for idx = start; idx < pluginCommon.MAX_VLAN_ID; idx++ {
		if vMgr.vlanDB[idx] != nil {
			if numEntries == count {
				more = true
				break
			}
			vlanList = append(vlanList, &asicdInt.Vlan{
				VlanId:           int32(vMgr.vlanDB[idx].vlanId),
				IfIndexList:      vMgr.vlanDB[idx].ifIndexList,
				UntagIfIndexList: vMgr.vlanDB[idx].untagIfIndexList,
			})
			numEntries += 1
		}
	}
	vMgr.dbMutex.RUnlock()
	end = idx
	if idx == pluginCommon.MAX_VLAN_ID {
		more = false
	}
	listLen = len(vlanList)
	return end, listLen, more, vlanList
}

func (vMgr *VlanManager) GetVlanState(vlanId int32) (*asicdServices.VlanState, error) {
	var vlanState *asicdServices.VlanState
	vMgr.dbMutex.RLock()
	defer vMgr.dbMutex.RUnlock()
	if (vlanId < 0) || (vlanId >= pluginCommon.MAX_VLAN_ID) {
		return nil, errors.New("Invalid vlan id key provided for get request")
	}
	if vMgr.vlanDB[vlanId] != nil {
		var desc string
		if vMgr.vlanDB[vlanId].adminState == INTF_STATE_DOWN {
			desc = OPER_STATE_DOWN_DUE_TO_ADMIN_STATE_DOWN
		}
		vlanState = &asicdServices.VlanState{
			VlanId:                 int32(vMgr.vlanDB[vlanId].vlanId),
			IfIndex:                vMgr.vlanDB[vlanId].ifIndex,
			VlanName:               vMgr.vlanDB[vlanId].vlanName,
			OperState:              vMgr.vlanDB[vlanId].operState,
			SysInternalDescription: desc,
		}
		return vlanState, nil
	} else {
		return nil, errors.New(fmt.Sprintln("Vlan state object cannot be located for vlan - ", vlanId))
	}
}

func (vMgr *VlanManager) GetBulkVlanState(start, count int) (end, listLen int, more bool, vlanList []*asicdServices.VlanState) {
	var desc string
	var numEntries, idx int = 0, 0
	if (start < 0) || (start >= pluginCommon.MAX_VLAN_ID) {
		vMgr.logger.Err("Invalid start index argument in get bulk vlan")
		return -1, 0, false, nil
	}
	if count < 0 {
		vMgr.logger.Err("Invalid count in get bulk vlan")
		return -1, 0, false, nil
	}
	vMgr.dbMutex.RLock()
	for idx = start; idx < pluginCommon.MAX_VLAN_ID; idx++ {
		if vMgr.vlanDB[idx] != nil {
			if numEntries == count {
				more = true
				break
			}
			if vMgr.vlanDB[idx].adminState == INTF_STATE_DOWN {
				desc = OPER_STATE_DOWN_DUE_TO_ADMIN_STATE_DOWN
			}
			vlanList = append(vlanList, &asicdServices.VlanState{
				VlanId:                 int32(vMgr.vlanDB[idx].vlanId),
				IfIndex:                vMgr.vlanDB[idx].ifIndex,
				VlanName:               vMgr.vlanDB[idx].vlanName,
				OperState:              vMgr.vlanDB[idx].operState,
				SysInternalDescription: desc,
			})
			numEntries += 1
		}
	}
	end = idx
	if idx == pluginCommon.MAX_VLAN_ID {
		more = false
	}
	listLen = len(vlanList)
	vMgr.dbMutex.RUnlock()
	return end, listLen, more, vlanList
}

/*
 * Utility function to retrieve system assigned vlan id, due to configuring
 * a L3 interface on a physical port
 */
func (vMgr *VlanManager) GetSysVlanContainingIf(ifIndex int32) int {
	var idx, vlanId int = 0, 0
	for idx = vMgr.sysRsvdVlanMin; idx <= vMgr.sysRsvdVlanMax; idx++ {
		vMgr.dbMutex.RLock()
		val := vMgr.vlanDB[idx]
		if val != nil {
			for _, intf := range val.untagIfIndexList {
				if intf == ifIndex {
					vlanId = int(idx)
				}
			}
		}
		vMgr.dbMutex.RUnlock()
		if vlanId != 0 {
			break
		}
	}
	if idx > vMgr.sysRsvdVlanMax {
		vMgr.logger.Err("Unable to locate system reserved vlan")
		return -1
	}
	return vlanId
}

func (vMgr *VlanManager) GetNextAvailableSysVlan() int {
	var idx, vlanId int = 0, 0
	//Assign a system reserved vlanid
	vMgr.dbMutex.RLock()
	for idx = vMgr.sysRsvdVlanMin; idx <= vMgr.sysRsvdVlanMax; idx++ {
		if vMgr.vlanDB[idx] == nil {
			//Found an available sys rsvd vlan
			vlanId = idx
			break
		}
	}
	vMgr.dbMutex.RUnlock()
	if idx > vMgr.sysRsvdVlanMax {
		return -1
	}
	return vlanId
}

func (vMgr *VlanManager) VlanProcessIntfStateChange(ifIndex int32, state int) {
	var operState string
	var activeIfCnt, notifyState int
	for vlanId, info := range VlanMgr.vlanDB {
		if info != nil {
			//Create a map containing all tagged/untagged ports in vlan for subsequent lookup
			var portMap map[int32]bool = make(map[int32]bool)
			for _, idx := range info.ifIndexList {
				portMap[idx] = true
			}
			for _, idx := range info.untagIfIndexList {
				portMap[idx] = true
			}
			if _, ok := portMap[ifIndex]; ok {
				evtKey := events.VlanKey{
					VlanId: int32(vlanId),
				}
				vMgr.dbMutex.Lock()
				oldOperState := info.operState
				if state == pluginCommon.INTF_STATE_DOWN {
					activeIfCnt = VlanMgr.vlanDB[vlanId].activeIntfCount - 1
				} else {
					activeIfCnt = VlanMgr.vlanDB[vlanId].activeIntfCount + 1
				}
				//Global override : disregard computed operstate if adminstate is down
				if info.adminState != INTF_STATE_DOWN {
					if activeIfCnt > 0 {
						operState = INTF_STATE_UP
						notifyState = pluginCommon.INTF_STATE_UP
					} else {
						operState = INTF_STATE_DOWN
						notifyState = pluginCommon.INTF_STATE_DOWN
					}
				} else {
					operState = INTF_STATE_DOWN
					notifyState = pluginCommon.INTF_STATE_DOWN
				}
				VlanMgr.vlanDB[vlanId] = &vlanInfo{
					vlanId:           vlanId,
					adminState:       info.adminState,
					operState:        operState,
					ifIndex:          info.ifIndex,
					vlanName:         info.vlanName,
					activeIntfCount:  activeIfCnt,
					ifIndexList:      info.ifIndexList,
					untagIfIndexList: info.untagIfIndexList,
				}
				vMgr.dbMutex.Unlock()
				if oldOperState != operState {
					var evt events.EventId
					switch operState {
					case INTF_STATE_DOWN:
						evt = events.VlanOperStateDown
					case INTF_STATE_UP:
						evt = events.VlanOperStateUp
					}
					txEvent := eventUtils.TxEvent{
						EventId:        evt,
						Key:            evtKey,
						AdditionalInfo: "",
						AdditionalData: nil,
					}
					err := eventUtils.PublishEvents(&txEvent)
					if err != nil {
						vMgr.logger.Err(fmt.Sprintln("Error publishing vlan oper state event"))
					}
				}
				//Have l3 determine if it has a state change
				vMgr.l3IntfMgr.L3IntfProcessL2IntfStateChange(info.ifIndex, notifyState)
			}
		}
	}
}

func (vMgr *VlanManager) VlanGetActiveIntfCount(vlanId int) int {
	vMgr.dbMutex.RLock()
	defer vMgr.dbMutex.RUnlock()
	if vMgr.vlanDB[vlanId] != nil {
		return vMgr.vlanDB[vlanId].activeIntfCount
	} else {
		return 0
	}
}
