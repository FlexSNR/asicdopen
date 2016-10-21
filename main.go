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
	"asicd/publisher"
	"asicd/rpc"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"utils/dbutils"
	"utils/keepalive"
	"utils/logging"
)

var pub *publisher.PublisherInfo
var logger *logging.Writer
var asicdServer *rpc.AsicDaemonServerInfo

func sigHandler(dbHdl *dbutils.DBUtil, pMgr *pluginManager.PluginManager) {
	var saveState bool
	//List of signals to handle
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGUSR1)

	signal := <-sigChan
	switch signal {
	case syscall.SIGHUP,
		syscall.SIGUSR1:
		//Stop thrift server
		asicdServer.Server.Stop()
		//Cleanup plugins
		if signal == syscall.SIGUSR1 {
			saveState = false
		} else {
			saveState = true
		}
		pMgr.Deinit(saveState)
		//Terminate publisher
		pub.DeinitPublisher()
		//Close DB
		dbHdl.Disconnect()
		os.Exit(0)
	default:
		logger.Err(fmt.Sprintln("Unhandled signal : ", signal))
	}
}

func main() {
	var err error
	fmt.Println("Starting asicd daemon")
	paramsDirStr := flag.String("params", "", "Directory Location for config file")
	flag.Parse()
	paramsDir := *paramsDirStr
	if paramsDir[len(paramsDir)-1] != '/' {
		paramsDir = paramsDir + "/"
	}
	baseDir := paramsDir + "../"
	//Initialize logger
	logger, err = logging.NewLogger("asicd", "ASICD :", true)
	if err != nil {
		fmt.Println("Failed to start the logger. Nothing will be logged...")
	}
	//Instantiate publisher
	pub = publisher.NewPublisher()
	pub.InitPublisher(logger)
	//Open connection to DB
	dbHdl := dbutils.NewDBUtil(logger)
	err = dbHdl.Connect()
	if err != nil {
		logger.Err("Failed to dial out to Redis server")
		return
	}
	//Parse cfg file and instantiate/initialize the plugin manager
	cfgFileInfo := parseConfigFile(paramsDir)
	pluginMgr := pluginManager.NewPluginMgr(cfgFileInfo.pluginList, logger, pub.PubChan)
	pluginMgr.Init(&pluginManager.InitParams{
		DbHdl:              dbHdl,
		BaseDir:            baseDir,
		BootMode:           cfgFileInfo.bootMode,
		SysRsvdVlanMin:     cfgFileInfo.sysRsvdVlanMin,
		SysRsvdVlanMax:     cfgFileInfo.sysRsvdVlanMax,
		IfMap:              cfgFileInfo.ifMap,
		SwitchMac:          cfgFileInfo.switchMac,
		MacFlapDisablePort: cfgFileInfo.MacFlapDisablePort,
		EnablePM:           cfgFileInfo.EnablePM,
	})
	go sigHandler(dbHdl, pluginMgr)
	//Start dev shell
	for _, plugin := range cfgFileInfo.pluginList {
		if plugin == "opennsl" {
			go DevShell(pluginMgr)
		} else if plugin == "bcmsdk" {
			go DevShell(pluginMgr)
		}
	}
	// Start keepalive routine
	go keepalive.InitKeepAlive("asicd", paramsDir)
	//Start rpc server
	asicdServer = rpc.NewAsicdServer("localhost:"+strconv.Itoa(cfgFileInfo.thriftServerPort), pluginMgr, logger, pub.PubChan)
	logger.Info("ASICD: server started")
	asicdServer.Server.Serve()
}
