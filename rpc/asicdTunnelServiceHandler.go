// This file defines all interfaces provided for L2/L3 tunnel service
package rpc

import (
	"asicdInt"
	//"errors"
	"fmt"
)

// vxlan related apis
func (svcHdlr AsicDaemonServiceHandler) CreateVxlan(config *asicdInt.Vxlan) (bool, error) {
	svcHdlr.logger.Info(fmt.Sprintf("Thrift request received to create vxlan %#v", config))
	return svcHdlr.pluginMgr.CreateVxlan(config)
}

func (svcHdlr AsicDaemonServiceHandler) DeleteVxlan(config *asicdInt.Vxlan) (bool, error) {
	svcHdlr.logger.Info(fmt.Sprintf("Thrift request received to delete vxlan %#v", config))
	return svcHdlr.pluginMgr.DeleteVxlan(config)
}

func (svcHdlr AsicDaemonServiceHandler) CreateVxlanVtep(config *asicdInt.Vtep) (bool, error) {
	svcHdlr.logger.Info(fmt.Sprintf("Thrift request received to create vtep %#v", config))
	return svcHdlr.pluginMgr.CreateVtepIntf(config)
}

func (svcHdlr AsicDaemonServiceHandler) DeleteVxlanVtep(config *asicdInt.Vtep) (bool, error) {
	svcHdlr.logger.Info(fmt.Sprintf("Thrift request received to delete vtep %#v", config))
	return svcHdlr.pluginMgr.DeleteVtepIntf(config)
}

func (svcHdlr AsicDaemonServiceHandler) AddHostInterfaceToVxlan(vni int32, IntfRefList, UntagIntfRefList []string) (bool, error) {
	svcHdlr.logger.Info(fmt.Sprintf("Thrift request received to add hosts %s %s to vni vtep %d", IntfRefList, UntagIntfRefList, vni))
	return svcHdlr.pluginMgr.AddHostInterfaceToVxlan(vni, IntfRefList, UntagIntfRefList)
}

func (svcHdlr AsicDaemonServiceHandler) DelHostInterfaceFromVxlan(vni int32, IntfRefList, UntagIntfRefList []string) (bool, error) {
	svcHdlr.logger.Info(fmt.Sprintf("Thrift request received to del hosts %s %s to vni vtep %d", IntfRefList, UntagIntfRefList, vni))
	return svcHdlr.pluginMgr.DelFromHostInterfaceFromVxlan(vni, IntfRefList, UntagIntfRefList)
}

func (svcHdlr AsicDaemonServiceHandler) LearnFdbVtep(mac string, name string, ifIndex int32) (rval bool, err error) {
	svcHdlr.logger.Info(fmt.Sprintf("Thrift request learn mac against vtep %s  %s", mac, name))
	svcHdlr.pluginMgr.LearnFdbVtep(mac, name, ifIndex)
	return rval, err
}
