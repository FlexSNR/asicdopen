#include <stdio.h>
#include <syslog.h>
#include <sai.h>
#include <saitypes.h>
#include <saistatus.h>
#include "_cgo_export.h"
#include "../pluginCommon/pluginCommon.h"

sai_fdb_api_t* sai_fdb_api_tbl = NULL;
extern int maxSysPorts;
extern sai_object_id_t port_list[MAX_NUM_PORTS];

void sai_fdb_cb (uint32_t count, sai_fdb_event_notification_data_t *data)
{
#ifdef SAI_BUILD
    int idx, op, attrIdx, port, portIdx;
    for (idx = 0; idx < count; idx++) {
        if (data[idx].event_type == SAI_FDB_EVENT_LEARNED) {
            op = MAC_ENTRY_LEARNED;
        } else if (data[idx].event_type == SAI_FDB_EVENT_AGED) {
            op = MAC_ENTRY_AGED;
        } else {
            syslog(LOG_ERR, "Unsupported FDB call back operation");
            continue;
        }
        for (attrIdx = 0; attrIdx < data[idx].attr_count; attrIdx++) {
            if (data[idx].attr[attrIdx].id == SAI_FDB_ENTRY_ATTR_PORT_ID) {
                switch(sai_object_type_query(data[idx].attr[attrIdx].value.oid)) {
                    case SAI_OBJECT_TYPE_PORT:
                        for (portIdx = 0; portIdx <= maxSysPorts; portIdx++) {
                            if (port_list[portIdx] == data[idx].attr[attrIdx].value.oid) {
                                port = SaiXlateChipPortToFPanelPortNum(portIdx);
                                break;
                            }
                        }
                        break;
                    default:
                        syslog(LOG_ERR, "Unsupported sai object type");
                }
            }
        }
        SaiNotifyFDBEvent(op, port, data[idx].fdb_entry.vlan_id, data[idx].fdb_entry.mac_address);
        syslog(LOG_INFO, "FDB event sent for port %d, vlan %d, op %d", port, data[idx].fdb_entry.vlan_id, op);
    }
#endif
}

int SaiAddRsvdProtoMacEntry(uint8_t *macAddr, uint8_t *mask, int vlanid)
{
    return 0;
}

int SaiDeleteRsvdProtoMacEntry(uint8_t *macAddr, uint8_t *mask, int vlanid)
{
    return 0;
}

int SaiCreateStg(int listLen, int *vlanList) {
    return 0;
}

int SaiDeleteStg(int stgId) {
    return 0;
}

int SaiSetPortStpState(int stgId, int port, int stpState) {
    return 0;
}

int SaiGetPortStpState(int stgId, int port) {
    return 0;
}

int SaiUpdateStgVlanList(int stgId, int oldListLen, int *oldVlanList, int newListLen, int *newVlanList) {
    return 0;
}

int SaiFlushFdbByPortVlan(int vlan, int port) 
{
}
