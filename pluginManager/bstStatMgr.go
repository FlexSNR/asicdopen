package pluginManager

import (
	"asicd/pluginManager/pluginCommon"
	"asicdServices"
	"errors"
	"fmt"
	"sync"
	"utils/logging"
)

type BstStatManager struct {
	dbMutex             sync.RWMutex
	plugins             []PluginIntf
	logger              *logging.Writer
	bufferGlobalStateDb map[int]*asicdServices.BufferGlobalStatState
	bMgr                *BstStatManager
	initComplete        bool
}

var BstStatMgr BstStatManager

func (bMgr *BstStatManager) Init(rsrcMgrs *ResourceManagers, pluginList []PluginIntf, logger *logging.Writer) {
	bMgr.bufferGlobalStateDb = make(map[int]*asicdServices.BufferGlobalStatState)
	bMgr.logger = logger
	bMgr.plugins = pluginList
	for _, plugin := range bMgr.plugins {
		if isControllingPlugin(plugin) == true {
			rv := plugin.InitBufferGlobalStateDB(0)
			if rv < 0 {
				bMgr.logger.Err("Failed to initialise buffer global stat db")
			}
		}
	}
	bMgr.logger.Info("Bst Manager Init is done")
}

func (bMgr *BstStatManager) Deinit() {
}

func UpdateBufferGlobalStateDB(bState *pluginCommon.BufferGlobalState) {
	return
}

func InitBufferGlobalStateDB(deviceId int) int {
	//check lock is initialized.
	BstStatMgr.dbMutex.Lock()
	BstStatMgr.bufferGlobalStateDb[deviceId] = asicdServices.NewBufferGlobalStatState()
	BstStatMgr.dbMutex.Unlock()
	return 0
}

func (bMgr *BstStatManager) GetBulkBufferGlobalStatState(start, count int) (end, listLen int, more bool, bStateList []*asicdServices.BufferGlobalStatState) {
	bMgr.logger.Debug(fmt.Sprintln("BUFFERGLBAL: called get bulk start, count", start, count))
	if count < 0 {
		bMgr.logger.Err("Invalid count during get bulk global buffer stats. ")
		return -1, 0, false, nil
	}

	return end, len(bStateList), more, bStateList
}

func (bMgr *BstStatManager) GetBufferGlobalStatState(deviceId int32) (*asicdServices.BufferGlobalStatState, error) {
	return nil, errors.New("Added for future")
}
