#ifndef SAI_CHIP_H
#define SAI_CHIP_H
#include "../pluginCommon/pluginCommon.h"
#include <stdbool.h>
int SaiInit(int bootMode, int ifMapCount, ifMapInfo_t *ifMap, uint8_t *macAddr);
int SaiDeinit(int cacheSwState);
void SaiDevShell(void);

int SaiGetMaxSysPorts();
int SaiUpdatePortConfig(int flags, portConfig *info);
int SaiInitPortConfigDB();
int SaiInitPortStateDB();
int SaiUpdatePortStateDB(int startPort, int endPort);

int SaiCreateVlan(int vlanId);
int SaiDeleteVlan(int vlanId);
int SaiUpdateVlanAddPorts(int vlanId, int portCount, int untagPortCount, int *portList, int *untagPortList);
int SaiUpdateVlanDeletePorts(int vlanId, int portCount, int untagPortCount, int *portList, int *untagPortList);

int SaiRouteAddDel(uint32_t *ipAddr, int ip_type, bool add);
int SaiCreateIPIntf(uint32_t *ipAddr, int maskLen, int vlanId);
int SaiDeleteIPIntf(uint32_t *ipAddr, int maskLen, int vlanId);

uint64_t SaiCreateIPNextHop(uint32_t *ipAddr, uint32_t nextHopFlags, int vlanId, int routerPhyIntf, uint8_t *macAddr, int ip_type);
int SaiDeleteIPNextHop(uint64_t nextHopId);
int SaiUpdateIPNextHop(uint32_t ipAddr, uint64_t nextHopId, int vlanId, int routerPhyIntf, uint8_t *macAddr);
int SaiRestoreIPv4NextHop();

uint64_t SaiCreateIPNextHopGroup(int numOfNh, uint64_t *nhIdArr); 
int SaiDeleteIPNextHopGroup(uint64_t ecmpGrpId);
int SaiUpdateIPNextHopGroup(int numOfNh, uint64_t *nhIdArr, uint64_t ecmpGrpId);

int SaiCreateIPNeighbor(uint32_t *ipAddr, uint32_t neighborFlags, uint8_t *macAddr, int vlanId, int ip_type);
int SaiUpdateIPNeighbor(uint32_t *ipAddr, uint8_t *macAddr, int vlanId, int ip_type);
int SaiDeleteIPNeighbor(uint32_t *ipAddr, uint8_t *macAddr, int vlanId, int ip_type);
int SaiRestoreIPNeighborDB();

int SaiRestoreIPv4RouteDB();
int SaiRestoreIPv6RouteDB();
int SaiDeleteIPRoute(uint8_t *ipPrefix, uint8_t *ipMask, uint32_t routeFlags);
int SaiCreateIPRoute(uint8_t *ipPrefix, uint8_t *ipMask, uint32_t routeFlags, uint64_t nextHopId, int rifId);

int SaiCreateLag(int hashType, int portCount, int *ports);
int SaiDeleteLag(int lagId);
int SaiUpdateLag(int lagId, int hashType, int oldPortCount, int *oldPorts, int portCount, int *ports);
int SaiRestoreLagDB();

int SaiCreateStg(int listLen, int *vlanList);
int SaiDeleteStg(int stgId);
int SaiSetPortStpState(int stgId, int port, int stpState);
int SaiGetPortStpState(int stgId, int port);
int SaiAddRsvdProtoMacEntry(uint8_t *macAddr, uint8_t *mask, int vlanid);
int SaiDeleteRsvdProtoMacEntry(uint8_t *macAddr, uint8_t *mask, int vlanid);
int SaiUpdateStgVlanList(int stgId, int oldListLen, int *oldVlanList, int newListLen, int *newVlanList); 
int SaiFlushFdbByPortVlan(int vlan, int port);
int SaiUpdateSubIPv4Intf(uint32_t ipAddr, bool state);

int SaiUpdateBufferGlobalStatDB(int startId, int endId);
int SaiProcessAcl(char* aclName, int aclType, aclRule acl, int length, int* portList, int direction);
int SaiClearPortStat(int port);
#endif //SAI_CHIP_H
