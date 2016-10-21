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

package main

import (
	"asicd/pluginManager"
	"asicd/pluginManager/pluginCommon"
	"encoding/json"
	"io/ioutil"
	"strconv"
	"strings"
)

type ClientJson struct {
	Name string `json:Name`
	Port int    `json:Port`
}

type cfgFileJson struct {
	SwitchMac          string            `json:"SwitchMac"`
	PluginList         []string          `json:"PluginList"`
	IfNameMap          map[string]string `json:"IfNameMap"`
	IfNamePrefix       map[string]string `json:"IfNamePrefix"`
	SysRsvdVlanRange   string            `json:"SysRsvdVlanRange"`
	MacFlapDisablePort bool              `json:"MacFlapDisablePort"`
	EnablePM           bool              `json:"EnablePM"`
}

type initCfgParams struct {
	switchMac                      string
	pluginList                     []string
	ifMap                          []pluginCommon.IfMapInfo
	sysRsvdVlanMin, sysRsvdVlanMax int
	bootMode, thriftServerPort     int
	MacFlapDisablePort             bool
	EnablePM                       bool
}

func parseConfigFile(paramsDir string) *initCfgParams {
	var cfgFile cfgFileJson
	var initCfg initCfgParams
	var vlanRange []string
	var defPluginList = make([]string, 1)
	var defIfMap = make([]pluginCommon.IfMapInfo, 1)
	var clientsList []ClientJson

	//Retrieve thrift port number
	initCfg.thriftServerPort = 4000
	bytes, err := ioutil.ReadFile(paramsDir + "clients.json")
	if err != nil {
		logger.Err("Error retrieving thrift server port number using default port 4000")
	} else {
		err := json.Unmarshal(bytes, &clientsList)
		if err != nil {
			logger.Err("Error retrieving thrift server port number using default port 4000")
		} else {
			for _, client := range clientsList {
				if client.Name == "asicd" {
					initCfg.thriftServerPort = client.Port
				}
			}
		}
	}

	//Retrieve boot mode
	bootModeFile, err := ioutil.ReadFile(paramsDir + "asicdBootMode.conf")
	if err != nil {
		initCfg.bootMode = pluginCommon.BOOT_MODE_COLDBOOT
	} else {
		val, err := strconv.Atoi(string(bootModeFile))
		if err != nil {
			initCfg.bootMode = pluginCommon.BOOT_MODE_COLDBOOT
		}
		if val == 0 {
			initCfg.bootMode = pluginCommon.BOOT_MODE_COLDBOOT
		} else {
			initCfg.bootMode = pluginCommon.BOOT_MODE_WARMBOOT
		}
	}

	//Setup defaults that can be overriden by config
	initCfg.switchMac = pluginCommon.DEFAULT_SWITCH_MAC_ADDR
	initCfg.pluginList = defPluginList
	initCfg.ifMap = defIfMap
	initCfg.sysRsvdVlanMin = pluginCommon.SYS_RSVD_VLAN_MIN
	initCfg.sysRsvdVlanMax = pluginCommon.SYS_RSVD_VLAN_MAX
	//Parse asicd specific config options
	defPluginList[0] = "linux"
	defIfMap[0] = pluginCommon.IfMapInfo{IfName: "fpPort-", Port: -1}
	cfgFileData, err := ioutil.ReadFile(paramsDir + pluginCommon.ASICD_CONFIG_FILE)
	if err != nil {
		logger.Err("Error reading config file - ", pluginCommon.ASICD_CONFIG_FILE,
			". Using defaults (linux plugin only)")
		return &initCfg
	}
	err = json.Unmarshal(cfgFileData, &cfgFile)
	if err != nil {
		logger.Err("Error parsing config file, using defaults (linux plugin only)")
		return &initCfg
	}
	initCfg.EnablePM = cfgFile.EnablePM
	initCfg.MacFlapDisablePort = cfgFile.MacFlapDisablePort
	//Set swith mac
	initCfg.switchMac = cfgFile.SwitchMac
	//Set plugin list
	initCfg.pluginList = cfgFile.PluginList
	//Set intf name map
	initCfg.ifMap = make([]pluginCommon.IfMapInfo, 0)
	if len(cfgFile.IfNameMap) != 0 {
		//Format => portnum:ifname
		for key, val := range cfgFile.IfNameMap {
			keyNum, err := strconv.Atoi(key)
			if err != nil {
				logger.Err("Error parsing interface mapping info. Mapping all front panel ports.")
			}
			initCfg.ifMap = append(initCfg.ifMap, pluginCommon.IfMapInfo{IfName: val, Port: keyNum})
		}
	} else if len(cfgFile.IfNamePrefix) != 0 {
		//Format => ifname_prefix : port_list
		for prefix, portStr := range cfgFile.IfNamePrefix {
			if portStr == "all" {
				initCfg.ifMap = append(initCfg.ifMap, pluginCommon.IfMapInfo{IfName: prefix, Port: -1})
			} else {
				portList, err := pluginManager.ParseUsrStrToList(portStr)
				if err != nil {
					logger.Err("Error parsing interface mapping info. Mapping all front panel ports.")
					initCfg.ifMap = append(initCfg.ifMap, pluginCommon.IfMapInfo{IfName: prefix, Port: -1})
				} else {
					for _, port := range portList {
						portVal, _ := strconv.Atoi(port)
						initCfg.ifMap = append(initCfg.ifMap, pluginCommon.IfMapInfo{IfName: prefix + port, Port: portVal})
					}
				}
			}
		}
	}
	//Set rsvd vlan range
	vlanRange = strings.Split(cfgFile.SysRsvdVlanRange, "-")
	initCfg.sysRsvdVlanMin, err = strconv.Atoi(vlanRange[0])
	if err != nil {
		logger.Err("Error parsing system reserved vlan range, using defaults (3835 - 4090)")
	}
	initCfg.sysRsvdVlanMax, err = strconv.Atoi(vlanRange[1])
	if err != nil {
		logger.Err("Error parsing system reserved vlan range, using defaults (3835 - 4090)")
	}
	return &initCfg
}
