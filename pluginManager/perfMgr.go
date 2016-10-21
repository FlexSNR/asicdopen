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
	"asicdServices"
	"errors"
	"models/events"
	"sync"
	"time"
	"utils/eventUtils"
	"utils/logging"
	"utils/ringBuffer"
)

const (
	CLASS_A                      = "CLASS-A"
	CLASS_A_INTVL  time.Duration = time.Duration(1) * time.Second // Polling Interval 1 sec
	CLASS_A_BUF_SZ int           = 60 * 60 * 24                   //Storage for 24 hrs
	CLASS_B                      = "CLASS-B"
	CLASS_B_INTVL  time.Duration = time.Duration(15) * time.Minute // Polling Interval 15 min
	CLASS_B_BUF_SZ int           = 4 * 24                          //Storage for 24 hrs
	CLASS_C                      = "CLASS-C"
	CLASS_C_INTVL  time.Duration = time.Duration(24) * time.Hour // Polling Interval 24 hrs
	CLASS_C_BUF_SZ int           = 365                           //Storage for 365 days
)

//Asic global PM resource list
const (
	ASIC_GBL_RSRC_TEMP = "Temperature"
)

//Ethernet PM resource list
const (
	ETH_PM_RSRC_UNDER_SIZE_PKTS = "StatUnderSizePkts"
	ETH_PM_RSRC_OVER_SIZE_PKTS  = "StatOverSizePkts"
	ETH_PM_RSRC_FRAG            = "StatFragments"
	ETH_PM_RSRC_CRC_ALIGN_ERR   = "StatCRCAlignErrors"
	ETH_PM_RSRC_JBR             = "StatJabber"
	ETH_PM_RSRC_PKTS            = "StatEtherPkts"
	ETH_PM_RSRC_MC_PKTS         = "StatMCPkts"
	ETH_PM_RSRC_BC_PKTS         = "StatBCPkts"
	ETH_PM_RSRC_64B             = "Stat64OctOrLess"
	ETH_PM_RSRC_65B_TO_127B     = "Stat65OctTo127Oct"
	ETH_PM_RSRC_128B_TO_255B    = "Stat128OctTo255Oct"
	ETH_PM_RSRC_256B_TO_511B    = "Stat256OctTo511Oct"
	ETH_PM_RSRC_512B_TO_1023B   = "Stat512OctTo1023Oct"
	ETH_PM_RSRC_1024B_TO_1518B  = "Statc1024OctTo1518Oct"
)

//Index values for event id's
const (
	HI_ALARM_EVT_IDX = iota
	HI_ALARM_CLR_EVT_IDX
	HI_WARN_EVT_IDX
	HI_WARN_CLR_EVT_IDX
	LO_ALARM_EVT_IDX
	LO_ALARM_CLR_EVT_IDX
	LO_WARN_EVT_IDX
	LO_WARN_CLR_EVT_IDX
)

type getPMData func() float64

type pmEnObj struct {
	pmClassAEnable bool
	pmClassBEnable bool
	pmClassCEnable bool
}

type tcaState struct {
	loWarnThreshActive  bool
	loAlarmThreshActive bool
	hiWarnThreshActive  bool
	hiAlarmThreshActive bool
}

type tcaObj struct {
	highAlarmThreshold float64
	highWarnThreshold  float64
	lowAlarmThreshold  float64
	lowWarnThreshold   float64
}

type sendTCAEventsInArg struct {
	pmVal  float64
	tcaCfg *tcaObj
	curTca *tcaState
	evtKey interface{}
	evtIds []events.EventId
}

type PerfManager struct {
	logger            logging.LoggerIntf
	rsrcMgrs          *ResourceManagers
	plugin            PluginIntf
	enabled           bool
	classAPMTick      *time.Ticker
	classBPMTick      *time.Ticker
	classCPMTick      *time.Ticker
	gblCfgMutex       sync.RWMutex
	gblCfg            map[string]*asicdServices.AsicGlobalPM
	gblPMResourceList []string
	tca               map[string]*tcaState
	pmDataMutex       map[string]map[string]*sync.RWMutex
	pmData            map[string]map[string]*ringBuffer.RingBuffer    //map[RsrcName]map[PMClass]Data
	ethCfgMutex       map[string]*sync.RWMutex                        //1 mutex per intf
	ethCfg            map[string]map[string]*asicdServices.EthernetPM //map[Rsrc]Cfg
	ethPMDataMutex    map[string]map[string]map[string]*sync.RWMutex
	ethPMData         map[string]map[string]map[string]*ringBuffer.RingBuffer
	ethIntfList       []string
	ethTca            map[string]map[string]*tcaState
	ethPMResourceList []string
}

var gblEventMap map[string][]events.EventId = map[string][]events.EventId{
	ASIC_GBL_RSRC_TEMP: []events.EventId{
		events.TemperatureHighAlarm,
		events.TemperatureHighAlarmClear,
		events.TemperatureHighWarn,
		events.TemperatureHighWarnClear,
		events.TemperatureLowAlarm,
		events.TemperatureLowAlarmClear,
		events.TemperatureLowWarn,
		events.TemperatureLowWarnClear,
	},
}

var ethEventMap map[string][]events.EventId = map[string][]events.EventId{
	ETH_PM_RSRC_UNDER_SIZE_PKTS: []events.EventId{
		events.EthIfUnderSizePktsHighAlarm,
		events.EthIfUnderSizePktsHighAlarmClear,
		events.EthIfUnderSizePktsHighWarn,
		events.EthIfUnderSizePktsHighWarnClear,
		events.EthIfUnderSizePktsLowAlarm,
		events.EthIfUnderSizePktsLowAlarmClear,
		events.EthIfUnderSizePktsLowWarn,
		events.EthIfUnderSizePktsLowWarnClear,
	},
	ETH_PM_RSRC_OVER_SIZE_PKTS: []events.EventId{
		events.EthIfOverSizePktsHighAlarm,
		events.EthIfOverSizePktsHighAlarmClear,
		events.EthIfOverSizePktsHighWarn,
		events.EthIfOverSizePktsHighWarnClear,
		events.EthIfOverSizePktsLowAlarm,
		events.EthIfOverSizePktsLowAlarmClear,
		events.EthIfOverSizePktsLowWarn,
		events.EthIfOverSizePktsLowWarnClear,
	},
	ETH_PM_RSRC_FRAG: []events.EventId{
		events.EthIfFragmentedPktsHighAlarm,
		events.EthIfFragmentedPktsHighAlarmClear,
		events.EthIfFragmentedPktsHighWarn,
		events.EthIfFragmentedPktsHighWarnClear,
		events.EthIfFragmentedPktsLowAlarm,
		events.EthIfFragmentedPktsLowAlarmClear,
		events.EthIfFragmentedPktsLowWarn,
		events.EthIfFragmentedPktsLowWarnClear,
	},
	ETH_PM_RSRC_CRC_ALIGN_ERR: []events.EventId{
		events.EthIfCRCAlignErrHighAlarm,
		events.EthIfCRCAlignErrHighAlarmClear,
		events.EthIfCRCAlignErrHighWarn,
		events.EthIfCRCAlignErrHighWarnClear,
		events.EthIfCRCAlignErrLowAlarm,
		events.EthIfCRCAlignErrLowAlarmClear,
		events.EthIfCRCAlignErrLowWarn,
		events.EthIfCRCAlignErrLowWarnClear,
	},
	ETH_PM_RSRC_JBR: []events.EventId{
		events.EthIfJabberFramesHighAlarm,
		events.EthIfJabberFramesHighAlarmClear,
		events.EthIfJabberFramesHighWarn,
		events.EthIfJabberFramesHighWarnClear,
		events.EthIfJabberFramesLowAlarm,
		events.EthIfJabberFramesLowAlarmClear,
		events.EthIfJabberFramesLowWarn,
		events.EthIfJabberFramesLowWarnClear,
	},
	ETH_PM_RSRC_PKTS: []events.EventId{
		events.EthIfEthernetPktsHighAlarm,
		events.EthIfEthernetPktsHighAlarmClear,
		events.EthIfEthernetPktsHighWarn,
		events.EthIfEthernetPktsHighWarnClear,
		events.EthIfEthernetPktsLowAlarm,
		events.EthIfEthernetPktsLowAlarmClear,
		events.EthIfEthernetPktsLowWarn,
		events.EthIfEthernetPktsLowWarnClear,
	},
	ETH_PM_RSRC_MC_PKTS: []events.EventId{
		events.EthIfMCPktsHighAlarm,
		events.EthIfMCPktsHighAlarmClear,
		events.EthIfMCPktsHighWarn,
		events.EthIfMCPktsHighWarnClear,
		events.EthIfMCPktsLowAlarm,
		events.EthIfMCPktsLowAlarmClear,
		events.EthIfMCPktsLowWarn,
		events.EthIfMCPktsLowWarnClear,
	},
	ETH_PM_RSRC_BC_PKTS: []events.EventId{
		events.EthIfBCPktsHighAlarm,
		events.EthIfBCPktsHighAlarmClear,
		events.EthIfBCPktsHighWarn,
		events.EthIfBCPktsHighWarnClear,
		events.EthIfBCPktsLowAlarm,
		events.EthIfBCPktsLowAlarmClear,
		events.EthIfBCPktsLowWarn,
		events.EthIfBCPktsLowWarnClear,
	},
	ETH_PM_RSRC_64B: []events.EventId{
		events.EthIf64BOrLessPktsHighAlarm,
		events.EthIf64BOrLessPktsHighAlarmClear,
		events.EthIf64BOrLessPktsHighWarn,
		events.EthIf64BOrLessPktsHighWarnClear,
		events.EthIf64BOrLessPktsLowAlarm,
		events.EthIf64BOrLessPktsLowAlarmClear,
		events.EthIf64BOrLessPktsLowWarn,
		events.EthIf64BOrLessPktsLowWarnClear,
	},
	ETH_PM_RSRC_65B_TO_127B: []events.EventId{
		events.EthIf65BTo127BPktsHighAlarm,
		events.EthIf65BTo127BPktsHighAlarmClear,
		events.EthIf65BTo127BPktsHighWarn,
		events.EthIf65BTo127BPktsHighWarnClear,
		events.EthIf65BTo127BPktsLowAlarm,
		events.EthIf65BTo127BPktsLowAlarmClear,
		events.EthIf65BTo127BPktsLowWarn,
		events.EthIf65BTo127BPktsLowWarnClear,
	},
	ETH_PM_RSRC_128B_TO_255B: []events.EventId{
		events.EthIf128BTo255BPktsHighAlarm,
		events.EthIf128BTo255BPktsHighAlarmClear,
		events.EthIf128BTo255BPktsHighWarn,
		events.EthIf128BTo255BPktsHighWarnClear,
		events.EthIf128BTo255BPktsLowAlarm,
		events.EthIf128BTo255BPktsLowAlarmClear,
		events.EthIf128BTo255BPktsLowWarn,
		events.EthIf128BTo255BPktsLowWarnClear,
	},
	ETH_PM_RSRC_256B_TO_511B: []events.EventId{
		events.EthIf256BTo511BPktsHighAlarm,
		events.EthIf256BTo511BPktsHighAlarmClear,
		events.EthIf256BTo511BPktsHighWarn,
		events.EthIf256BTo511BPktsHighWarnClear,
		events.EthIf256BTo511BPktsLowAlarm,
		events.EthIf256BTo511BPktsLowAlarmClear,
		events.EthIf256BTo511BPktsLowWarn,
		events.EthIf256BTo511BPktsLowWarnClear,
	},
	ETH_PM_RSRC_512B_TO_1023B: []events.EventId{
		events.EthIf512BTo1023BPktsHighAlarm,
		events.EthIf512BTo1023BPktsHighAlarmClear,
		events.EthIf512BTo1023BPktsHighWarn,
		events.EthIf512BTo1023BPktsHighWarnClear,
		events.EthIf512BTo1023BPktsLowAlarm,
		events.EthIf512BTo1023BPktsLowAlarmClear,
		events.EthIf512BTo1023BPktsLowWarn,
		events.EthIf512BTo1023BPktsLowWarnClear,
	},
	ETH_PM_RSRC_1024B_TO_1518B: []events.EventId{
		events.EthIf1024BTo1518BPktsHighAlarm,
		events.EthIf1024BTo1518BPktsHighAlarmClear,
		events.EthIf1024BTo1518BPktsHighWarn,
		events.EthIf1024BTo1518BPktsHighWarnClear,
		events.EthIf1024BTo1518BPktsLowAlarm,
		events.EthIf1024BTo1518BPktsLowAlarmClear,
		events.EthIf1024BTo1518BPktsLowWarn,
		events.EthIf1024BTo1518BPktsLowWarnClear,
	},
}

var PerfMgr PerfManager

func (p *PerfManager) Init(rsrcMgrs *ResourceManagers, plugins []PluginIntf, logger logging.LoggerIntf, enable bool) {
	//Only initalize when enabled
	if !enable {
		p.enabled = enable
		return
	}
	p.logger = logger
	p.rsrcMgrs = rsrcMgrs
	for _, plugin := range plugins {
		if isControllingPlugin(plugin) {
			p.plugin = plugin
		}
	}
	p.gblCfg = make(map[string]*asicdServices.AsicGlobalPM)
	p.gblPMResourceList = []string{
		ASIC_GBL_RSRC_TEMP,
	}
	//Setup defaults for global cfg data
	p.gblCfg[ASIC_GBL_RSRC_TEMP] = &asicdServices.AsicGlobalPM{
		ModuleId:           int8(0),
		Resource:           ASIC_GBL_RSRC_TEMP,
		PMClassAEnable:     false,
		PMClassBEnable:     false,
		PMClassCEnable:     false,
		HighAlarmThreshold: 1000000,
		HighWarnThreshold:  1000000,
		LowAlarmThreshold:  -1000000,
		LowWarnThreshold:   -1000000,
	}
	p.ethCfgMutex = make(map[string]*sync.RWMutex)
	p.ethCfg = make(map[string]map[string]*asicdServices.EthernetPM)
	p.ethIntfList = p.rsrcMgrs.PortManager.GetAllIntfRef()
	p.ethPMResourceList = []string{
		ETH_PM_RSRC_UNDER_SIZE_PKTS,
		ETH_PM_RSRC_OVER_SIZE_PKTS,
		ETH_PM_RSRC_FRAG,
		ETH_PM_RSRC_CRC_ALIGN_ERR,
		ETH_PM_RSRC_JBR,
		ETH_PM_RSRC_PKTS,
		ETH_PM_RSRC_MC_PKTS,
		ETH_PM_RSRC_BC_PKTS,
		ETH_PM_RSRC_64B,
		ETH_PM_RSRC_65B_TO_127B,
		ETH_PM_RSRC_128B_TO_255B,
		ETH_PM_RSRC_256B_TO_511B,
		ETH_PM_RSRC_512B_TO_1023B,
		ETH_PM_RSRC_1024B_TO_1518B,
	}
	for _, intf := range p.ethIntfList {
		p.ethCfgMutex[intf] = new(sync.RWMutex)
		p.ethCfg[intf] = make(map[string]*asicdServices.EthernetPM)
		for _, rsrc := range p.ethPMResourceList {
			p.ethCfg[intf][rsrc] = &asicdServices.EthernetPM{
				IntfRef:            intf,
				Resource:           rsrc,
				PMClassAEnable:     false,
				PMClassBEnable:     false,
				PMClassCEnable:     false,
				HighAlarmThreshold: 1000000,
				HighWarnThreshold:  1000000,
				LowAlarmThreshold:  -1000000,
				LowWarnThreshold:   -1000000,
			}
		}
	}
	//Setup defaults for ethernet cfg data
	p.initPM()
}

func (p *PerfManager) Deinit() {
	if !p.enabled {
		return
	}
}

func (p *PerfManager) initPM() {
	p.pmDataMutex = make(map[string]map[string]*sync.RWMutex)
	p.pmData = make(map[string]map[string]*ringBuffer.RingBuffer)
	p.tca = make(map[string]*tcaState)
	//Allocate storage for PM data of all classes
	for _, rsrc := range p.gblPMResourceList {
		p.tca[rsrc] = new(tcaState)
		p.pmDataMutex[rsrc] = make(map[string]*sync.RWMutex)
		p.pmData[rsrc] = make(map[string]*ringBuffer.RingBuffer)
		p.pmDataMutex[rsrc][CLASS_A] = new(sync.RWMutex)
		p.pmData[rsrc][CLASS_A] = new(ringBuffer.RingBuffer)
		p.pmData[rsrc][CLASS_A].SetRingBufferCapacity(CLASS_A_BUF_SZ)
		p.pmDataMutex[rsrc][CLASS_B] = new(sync.RWMutex)
		p.pmData[rsrc][CLASS_B] = new(ringBuffer.RingBuffer)
		p.pmData[rsrc][CLASS_B].SetRingBufferCapacity(CLASS_B_BUF_SZ)
		p.pmDataMutex[rsrc][CLASS_C] = new(sync.RWMutex)
		p.pmData[rsrc][CLASS_C] = new(ringBuffer.RingBuffer)
		p.pmData[rsrc][CLASS_C].SetRingBufferCapacity(CLASS_C_BUF_SZ)
	}
	p.ethTca = make(map[string]map[string]*tcaState)
	p.ethPMDataMutex = make(map[string]map[string]map[string]*sync.RWMutex)
	p.ethPMData = make(map[string]map[string]map[string]*ringBuffer.RingBuffer)
	//Allocate storage for PM data of all classes
	for _, intf := range p.ethIntfList {
		p.ethTca[intf] = make(map[string]*tcaState)
		p.ethPMDataMutex[intf] = make(map[string]map[string]*sync.RWMutex)
		p.ethPMData[intf] = make(map[string]map[string]*ringBuffer.RingBuffer)
		for _, rsrc := range p.ethPMResourceList {
			p.ethTca[intf][rsrc] = new(tcaState)
			p.ethPMDataMutex[intf][rsrc] = make(map[string]*sync.RWMutex)
			p.ethPMData[intf][rsrc] = make(map[string]*ringBuffer.RingBuffer)
			p.ethPMDataMutex[intf][rsrc][CLASS_A] = new(sync.RWMutex)
			p.ethPMData[intf][rsrc][CLASS_A] = new(ringBuffer.RingBuffer)
			p.ethPMData[intf][rsrc][CLASS_A].SetRingBufferCapacity(CLASS_A_BUF_SZ)
			p.ethPMDataMutex[intf][rsrc][CLASS_B] = new(sync.RWMutex)
			p.ethPMData[intf][rsrc][CLASS_B] = new(ringBuffer.RingBuffer)
			p.ethPMData[intf][rsrc][CLASS_B].SetRingBufferCapacity(CLASS_B_BUF_SZ)
			p.ethPMDataMutex[intf][rsrc][CLASS_C] = new(sync.RWMutex)
			p.ethPMData[intf][rsrc][CLASS_C] = new(ringBuffer.RingBuffer)
			p.ethPMData[intf][rsrc][CLASS_C].SetRingBufferCapacity(CLASS_C_BUF_SZ)
		}
	}
	p.classAPMTick = time.NewTicker(CLASS_A_INTVL)
	p.classBPMTick = time.NewTicker(CLASS_B_INTVL)
	p.classCPMTick = time.NewTicker(CLASS_C_INTVL)
	go p.startPM()
}

func (p *PerfManager) startPM() {
	for {
		select {
		case _ = <-p.classAPMTick.C:
			p.processPMs(CLASS_A)
			p.processEthPMs(CLASS_A)
		case _ = <-p.classBPMTick.C:
			p.processPMs(CLASS_B)
			p.processEthPMs(CLASS_B)
		case _ = <-p.classCPMTick.C:
			p.processPMs(CLASS_C)
			p.processEthPMs(CLASS_C)
		}
	}
}

func (p *PerfManager) processPMs(class string) {
	for _, rsrc := range p.gblPMResourceList {
		if p.classPMEnabled(rsrc, class) {
			//Save PM data
			pmData := asicdServices.PMData{
				TimeStamp: time.Now().String(),
				Value:     p.pmDataGetFuncs(rsrc)(),
			}
			mtx := p.pmDataMutex[rsrc][class]
			mtx.Lock()
			p.pmData[rsrc][class].InsertIntoRingBuffer(pmData)
			mtx.Unlock()
			//Process TCA
			p.processGblTCA(rsrc, pmData.Value)
		}
	}
}

func (p *PerfManager) processEthPMs(class string) {
	for _, intf := range p.ethIntfList {
		for _, rsrc := range p.ethPMResourceList {
			if p.ethClassPMEnabled(intf, rsrc, class) {
				//Save PM data
				pmData := asicdServices.PMData{
					TimeStamp: time.Now().String(),
					Value:     p.getEthPMData(intf, rsrc),
				}
				mtx := p.ethPMDataMutex[intf][rsrc][class]
				mtx.Lock()
				p.ethPMData[intf][rsrc][class].InsertIntoRingBuffer(pmData)
				mtx.Unlock()
				//Process TCA
				p.processEthTCA(intf, rsrc, pmData.Value)
			}
		}
	}
}

func (p *PerfManager) classPMEnabled(rsrc, class string) bool {
	var en bool
	var pmEn pmEnObj
	switch rsrc {
	case ASIC_GBL_RSRC_TEMP:
		p.gblCfgMutex.RLock()
		pmEn = pmEnObj{
			pmClassAEnable: p.gblCfg[rsrc].PMClassAEnable,
			pmClassBEnable: p.gblCfg[rsrc].PMClassBEnable,
			pmClassCEnable: p.gblCfg[rsrc].PMClassCEnable,
		}
		p.gblCfgMutex.RUnlock()
	}
	switch class {
	case CLASS_A:
		en = pmEn.pmClassAEnable
	case CLASS_B:
		en = pmEn.pmClassBEnable
	case CLASS_C:
		en = pmEn.pmClassCEnable
	}
	return en
}

func (p *PerfManager) ethClassPMEnabled(intfRef, rsrc, class string) bool {
	var en bool
	var pmEn pmEnObj
	switch rsrc {
	default: //All ethernet PMs
		mtx := p.ethCfgMutex[intfRef]
		mtx.RLock()
		pmEn = pmEnObj{
			pmClassAEnable: p.ethCfg[intfRef][rsrc].PMClassAEnable,
			pmClassBEnable: p.ethCfg[intfRef][rsrc].PMClassBEnable,
			pmClassCEnable: p.ethCfg[intfRef][rsrc].PMClassCEnable,
		}
		mtx.RUnlock()
	}
	switch class {
	case CLASS_A:
		en = pmEn.pmClassAEnable
	case CLASS_B:
		en = pmEn.pmClassBEnable
	case CLASS_C:
		en = pmEn.pmClassCEnable
	}
	return en
}

func (p *PerfManager) pmDataGetFuncs(rsrc string) getPMData {
	var fn getPMData
	switch rsrc {
	case ASIC_GBL_RSRC_TEMP:
		fn = p.plugin.GetModuleTemperature
	}
	return fn
}

func (p *PerfManager) getEthPMData(intf, rsrc string) float64 {
	var data float64
	obj, err := p.rsrcMgrs.PortManager.GetPortState(intf)
	if err == nil {
		switch rsrc {
		case ETH_PM_RSRC_UNDER_SIZE_PKTS:
			data = float64(obj.IfEtherUnderSizePktCnt)
		case ETH_PM_RSRC_OVER_SIZE_PKTS:
			data = float64(obj.IfEtherOverSizePktCnt)
		case ETH_PM_RSRC_FRAG:
			data = float64(obj.IfEtherFragments)
		case ETH_PM_RSRC_CRC_ALIGN_ERR:
			data = float64(obj.IfEtherCRCAlignError)
		case ETH_PM_RSRC_JBR:
			data = float64(obj.IfEtherJabber)
		case ETH_PM_RSRC_PKTS:
			data = float64(obj.IfEtherPkts)
		case ETH_PM_RSRC_MC_PKTS:
			data = float64(obj.IfEtherMCPkts)
		case ETH_PM_RSRC_BC_PKTS:
			data = float64(obj.IfEtherBcastPkts)
		case ETH_PM_RSRC_64B:
			data = float64(obj.IfEtherPkts64OrLessOctets)
		case ETH_PM_RSRC_65B_TO_127B:
			data = float64(obj.IfEtherPkts65To127Octets)
		case ETH_PM_RSRC_128B_TO_255B:
			data = float64(obj.IfEtherPkts128To255Octets)
		case ETH_PM_RSRC_256B_TO_511B:
			data = float64(obj.IfEtherPkts256To511Octets)
		case ETH_PM_RSRC_512B_TO_1023B:
			data = float64(obj.IfEtherPkts512To1023Octets)
		case ETH_PM_RSRC_1024B_TO_1518B:
			data = float64(obj.IfEtherPkts1024To1518Octets)
		}
	}
	return data
}

func (p *PerfManager) processGblTCA(rsrc string, pmVal float64) {
	p.gblCfgMutex.RLock()
	tcaCfg := &tcaObj{
		highAlarmThreshold: p.gblCfg[rsrc].HighAlarmThreshold,
		highWarnThreshold:  p.gblCfg[rsrc].HighWarnThreshold,
		lowAlarmThreshold:  p.gblCfg[rsrc].LowAlarmThreshold,
		lowWarnThreshold:   p.gblCfg[rsrc].LowWarnThreshold,
	}
	p.gblCfgMutex.RUnlock()
	curTca := p.tca[rsrc]
	p.sendTCAEvents(&sendTCAEventsInArg{
		pmVal:  pmVal,
		tcaCfg: tcaCfg,
		curTca: curTca,
		evtKey: events.AsicGlobalPMTCAKey{
			ModuleId: 0,
			Resource: rsrc,
		},
		evtIds: gblEventMap[rsrc],
	})
}

func (p *PerfManager) processEthTCA(intf, rsrc string, pmVal float64) {
	p.ethCfgMutex[intf].RLock()
	tcaCfg := &tcaObj{
		highAlarmThreshold: p.ethCfg[intf][rsrc].HighAlarmThreshold,
		highWarnThreshold:  p.ethCfg[intf][rsrc].HighWarnThreshold,
		lowAlarmThreshold:  p.ethCfg[intf][rsrc].LowAlarmThreshold,
		lowWarnThreshold:   p.ethCfg[intf][rsrc].LowWarnThreshold,
	}
	p.ethCfgMutex[intf].RUnlock()
	curTca := p.ethTca[intf][rsrc]
	p.sendTCAEvents(&sendTCAEventsInArg{
		pmVal:  pmVal,
		tcaCfg: tcaCfg,
		curTca: curTca,
		evtKey: events.EthernetPMTCAKey{
			IntfRef:  intf,
			Resource: rsrc,
		},
		evtIds: ethEventMap[rsrc],
	})
}

func (p *PerfManager) sendTCAEvents(obj *sendTCAEventsInArg) {
	var evts []events.EventId
	if obj.pmVal > obj.tcaCfg.highWarnThreshold {
		if !obj.curTca.hiWarnThreshActive {
			//Send high warn TCA
			evts = append(evts, obj.evtIds[HI_WARN_EVT_IDX])
			obj.curTca.hiWarnThreshActive = true
		}
	} else {
		if obj.curTca.hiWarnThreshActive {
			//Send high warn clear TCA
			evts = append(evts, obj.evtIds[HI_WARN_CLR_EVT_IDX])
			obj.curTca.hiWarnThreshActive = false
		}
	}
	if obj.pmVal > obj.tcaCfg.highAlarmThreshold {
		if !obj.curTca.hiAlarmThreshActive {
			//Send high alarm TCA
			evts = append(evts, obj.evtIds[HI_ALARM_EVT_IDX])
			obj.curTca.hiAlarmThreshActive = true
		}
	} else {
		if obj.curTca.hiAlarmThreshActive {
			//Send high alarm clear TCA
			evts = append(evts, obj.evtIds[HI_ALARM_CLR_EVT_IDX])
			obj.curTca.hiAlarmThreshActive = false
		}
	}
	if obj.pmVal < obj.tcaCfg.lowWarnThreshold {
		if !obj.curTca.loWarnThreshActive {
			//Send low warn TCA
			evts = append(evts, obj.evtIds[LO_WARN_EVT_IDX])
			obj.curTca.loWarnThreshActive = true
		}
	} else {
		if obj.curTca.loWarnThreshActive {
			//Send low warn clear TCA
			evts = append(evts, obj.evtIds[LO_WARN_CLR_EVT_IDX])
			obj.curTca.loWarnThreshActive = false
		}
	}
	if obj.pmVal < obj.tcaCfg.lowAlarmThreshold {
		if !obj.curTca.loAlarmThreshActive {
			//Send low alarm TCA
			evts = append(evts, obj.evtIds[LO_ALARM_EVT_IDX])
			obj.curTca.loAlarmThreshActive = true
		}
	} else {
		if obj.curTca.loAlarmThreshActive {
			//Send low alarm clear TCA
			evts = append(evts, obj.evtIds[LO_ALARM_CLR_EVT_IDX])
			obj.curTca.loAlarmThreshActive = false
		}
	}
	//Publish events
	evt := eventUtils.TxEvent{
		Key: obj.evtKey,
	}
	for _, evtId := range evts {
		evt.EventId = evtId
		err := eventUtils.PublishEvents(&evt)
		if err != nil {
			p.logger.Err("Error publishing TCA event : ", evtId)
		}
	}
}

func (p *PerfManager) UpdateAsicGlobalPM(oldObj, newObj *asicdServices.AsicGlobalPM, attrset []bool) (bool, error) {
	var ok bool
	var err error
	var obj *asicdServices.AsicGlobalPM
	if !p.enabled {
		return false, errors.New("PM not enabled")
	}
	p.gblCfgMutex.Lock()
	defer p.gblCfgMutex.Unlock()
	if obj, ok = p.gblCfg[newObj.Resource]; ok {
		*obj = *newObj
	} else {
		err = errors.New("UpdateAsicGlobalPMCfg failed, resource name unrecognized")
	}
	return ok, err
}

func (p *PerfManager) GetAsicGlobalPM(moduleId uint8, resource string) (*asicdServices.AsicGlobalPM, error) {
	var ok bool
	var err error
	var obj *asicdServices.AsicGlobalPM
	if !p.enabled {
		return nil, errors.New("PM not enabled")
	}
	p.gblCfgMutex.RLock()
	defer p.gblCfgMutex.RUnlock()
	if obj, ok = p.gblCfg[resource]; !ok {
		err = errors.New("GetAsicGlobalPMCfg failed, resource name unrecognized")
	}
	return obj, err
}

func (p *PerfManager) GetBulkAsicGlobalPM(currMarker, count int) (int, int, int, bool, []*asicdServices.AsicGlobalPM) {
	if !p.enabled {
		return 0, 0, 0, false, nil
	}
	p.gblCfgMutex.RLock()
	defer p.gblCfgMutex.RUnlock()
	return 0, 0, 1, false, []*asicdServices.AsicGlobalPM{p.gblCfg[ASIC_GBL_RSRC_TEMP]}
}

func (p *PerfManager) GetAsicGlobalPMState(moduleId uint8, resource string) (*asicdServices.AsicGlobalPMState, error) {
	var dataA, dataB, dataC []*asicdServices.PMData
	if !p.enabled {
		return nil, errors.New("PM not enabled")
	}
	if _, exists := p.gblCfg[resource]; !exists {
		return nil, errors.New("Invalid resource name specified in GetAsicGlobalPMState")
	}
	mtx := p.pmDataMutex[resource][CLASS_A]
	mtx.RLock()
	entriesPMA := p.pmData[resource][CLASS_A].GetListOfEntriesFromRingBuffer()
	mtx.RUnlock()
	for _, val := range entriesPMA {
		dataA = append(dataA, &asicdServices.PMData{
			TimeStamp: val.(asicdServices.PMData).TimeStamp,
			Value:     val.(asicdServices.PMData).Value,
		})
	}
	mtx = p.pmDataMutex[resource][CLASS_B]
	mtx.RLock()
	entriesPMB := p.pmData[resource][CLASS_B].GetListOfEntriesFromRingBuffer()
	mtx.RUnlock()
	for _, val := range entriesPMB {
		dataB = append(dataB, &asicdServices.PMData{
			TimeStamp: val.(asicdServices.PMData).TimeStamp,
			Value:     val.(asicdServices.PMData).Value,
		})
	}
	mtx = p.pmDataMutex[resource][CLASS_C]
	mtx.RLock()
	entriesPMC := p.pmData[resource][CLASS_C].GetListOfEntriesFromRingBuffer()
	mtx.RUnlock()
	for _, val := range entriesPMC {
		dataC = append(dataC, &asicdServices.PMData{
			TimeStamp: val.(asicdServices.PMData).TimeStamp,
			Value:     val.(asicdServices.PMData).Value,
		})
	}
	return &asicdServices.AsicGlobalPMState{
		ModuleId:     int8(moduleId),
		Resource:     resource,
		ClassAPMData: dataA,
		ClassBPMData: dataB,
		ClassCPMData: dataC,
	}, nil
}

func (p *PerfManager) UpdateEthernetPM(oldObj, newObj *asicdServices.EthernetPM, attrset []bool) (bool, error) {
	var ok bool
	var err error
	var obj *asicdServices.EthernetPM
	var cfgMap map[string]*asicdServices.EthernetPM
	if !p.enabled {
		return false, errors.New("PM not enabled")
	}
	if cfgMap, ok = p.ethCfg[newObj.IntfRef]; ok {
		if obj, ok = cfgMap[newObj.Resource]; ok {
			mtx := p.ethCfgMutex[newObj.IntfRef]
			mtx.Lock()
			*obj = *newObj
			mtx.Unlock()
		} else {
			err = errors.New("UpdateEthernetPMCfg failed, resource name unrecognized")
		}
	} else {
		err = errors.New("UpdateEthernetPMCfg failed, invalid intfref provided")
	}
	return ok, err
}

func (p *PerfManager) GetEthernetPM(intfRef, resource string) (*asicdServices.EthernetPM, error) {
	var ok bool
	var err error
	var obj *asicdServices.EthernetPM
	var cfgMap map[string]*asicdServices.EthernetPM
	if !p.enabled {
		return nil, errors.New("PM not enabled")
	}
	mtx := p.ethCfgMutex[intfRef]
	mtx.RLock()
	defer mtx.RUnlock()
	if cfgMap, ok = p.ethCfg[intfRef]; ok {
		if obj, ok = cfgMap[resource]; !ok {
			err = errors.New("GetEthernetPMCfg failed, resource name unrecognized")
		}
	} else {
		err = errors.New("UpdateEthernetPMCfg failed, invalid intfref provided")
	}
	return obj, err
}

func (p *PerfManager) GetBulkEthernetPM(currMarker, count int) (int, int, int, bool, []*asicdServices.EthernetPM) {
	var end int
	var more bool
	var idx int
	var rsrc string
	var objList []*asicdServices.EthernetPM
	if !p.enabled {
		return 0, 0, 0, false, nil
	}
	for _, intf := range p.ethIntfList {
		for _, rsrc = range p.ethPMResourceList {
			if len(objList) == count {
				end = idx
				more = true
				break
			}
			if idx >= currMarker {
				mtx := p.ethCfgMutex[intf]
				mtx.RLock()
				objList = append(objList, p.ethCfg[intf][rsrc])
				mtx.RUnlock()
			}
			idx++
		}
	}
	if end == 0 {
		end = len(p.ethPMResourceList)
	}
	return currMarker, end, len(objList), more, objList
}

func (p *PerfManager) GetEthernetPMState(intfRef, resource string) (*asicdServices.EthernetPMState, error) {
	var dataA, dataB, dataC []*asicdServices.PMData
	if !p.enabled {
		return nil, errors.New("PM not enabled")
	}
	if _, exists := p.ethCfg[intfRef]; exists {
		if _, exists = p.ethCfg[intfRef][resource]; !exists {
			return nil, errors.New("Invalid resource name specified in GetEthernetPMState")
		}
	} else {
		return nil, errors.New("Invalid intf ref name specified in GetEthernetPMState")
	}
	mtx := p.ethPMDataMutex[intfRef][resource][CLASS_A]
	mtx.RLock()
	entriesPMA := p.ethPMData[intfRef][resource][CLASS_A].GetListOfEntriesFromRingBuffer()
	mtx.RUnlock()
	for _, val := range entriesPMA {
		dataA = append(dataA, &asicdServices.PMData{
			TimeStamp: val.(asicdServices.PMData).TimeStamp,
			Value:     val.(asicdServices.PMData).Value,
		})
	}
	mtx = p.ethPMDataMutex[intfRef][resource][CLASS_B]
	mtx.RLock()
	entriesPMB := p.ethPMData[intfRef][resource][CLASS_B].GetListOfEntriesFromRingBuffer()
	mtx.RUnlock()
	for _, val := range entriesPMB {
		dataB = append(dataB, &asicdServices.PMData{
			TimeStamp: val.(asicdServices.PMData).TimeStamp,
			Value:     val.(asicdServices.PMData).Value,
		})
	}
	mtx = p.ethPMDataMutex[intfRef][resource][CLASS_C]
	mtx.RLock()
	entriesPMC := p.ethPMData[intfRef][resource][CLASS_C].GetListOfEntriesFromRingBuffer()
	mtx.RUnlock()
	for _, val := range entriesPMC {
		dataC = append(dataC, &asicdServices.PMData{
			TimeStamp: val.(asicdServices.PMData).TimeStamp,
			Value:     val.(asicdServices.PMData).Value,
		})
	}
	return &asicdServices.EthernetPMState{
		IntfRef:      intfRef,
		Resource:     resource,
		ClassAPMData: dataA,
		ClassBPMData: dataB,
		ClassCPMData: dataC,
	}, nil
}
