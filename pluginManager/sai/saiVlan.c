#include <stdio.h>
#include <syslog.h>
#include <sai.h>
#include <saitypes.h>
#include <saistatus.h>
#include "../pluginCommon/pluginCommon.h"

sai_vlan_api_t* sai_vlan_api_tbl = NULL;
extern sai_port_api_t* sai_port_api_tbl;
extern sai_object_id_t port_list[MAX_NUM_PORTS];
/* Vlan member object id's */
#ifdef MLNX_SAI
sai_object_id_t vlan_member_oid[MAX_VLAN_ID-1][MAX_NUM_PORTS];
#endif

int SaiCreateVlan(int vlanId)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    rv = sai_vlan_api_tbl->create_vlan(vlanId);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to create vlan - %d", vlanId);
        return -1;
    }
#endif
    return 0;
}

int SaiDeleteVlan(int vlanId)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    rv = sai_vlan_api_tbl->remove_vlan(vlanId);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to remove vlan - %d", vlanId);
        return -1;
    }
#endif
    return 0;
}

int SaiUpdateVlanAddPorts(int vlanId, int portCount, int untagPortCount, int *portList, int *untagPortList)
{
#ifdef SAI_BUILD
#ifdef MLNX_SAI
    int i, cport;
    sai_status_t rv;
    sai_attribute_t vlan_member_attr[3];
    sai_attribute_t port_attr;

    vlan_member_attr[0].id = SAI_VLAN_MEMBER_ATTR_VLAN_ID;
    vlan_member_attr[0].value.u16 = vlanId;
    for (i = 0; i < portCount; i++) {
        cport = SaiXlateFPanelPortToChipPortNum(portList[i]);
        vlan_member_attr[1].id = SAI_VLAN_MEMBER_ATTR_PORT_ID;
        vlan_member_attr[1].value.oid = port_list[cport];
        vlan_member_attr[2].id = SAI_VLAN_MEMBER_ATTR_TAGGING_MODE;
        vlan_member_attr[2].value.s32 = SAI_VLAN_PORT_TAGGED;
        rv = sai_vlan_api_tbl->create_vlan_member(&vlan_member_oid[vlanId][cport], 3, vlan_member_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unable to add port %d to vlan %d", portList[i], vlanId);
            return -1;
        }
    }
    for (i = 0; i < untagPortCount; i++) {
        cport = SaiXlateFPanelPortToChipPortNum(untagPortList[i]);
        vlan_member_attr[1].id = SAI_VLAN_MEMBER_ATTR_PORT_ID;
        vlan_member_attr[1].value.oid = port_list[cport];
        vlan_member_attr[2].id = SAI_VLAN_MEMBER_ATTR_TAGGING_MODE;
        vlan_member_attr[2].value.s32 = SAI_VLAN_PORT_UNTAGGED;
        rv = sai_vlan_api_tbl->create_vlan_member(&vlan_member_oid[vlanId][cport], 3, vlan_member_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unable to add port %d to vlan %d", untagPortList[i], vlanId);
            return -1;
        }
        port_attr.id = SAI_PORT_ATTR_PORT_VLAN_ID;
        port_attr.value.u16 = vlanId;
        rv = sai_port_api_tbl->set_port_attribute(port_list[cport], &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unable to set pvid of port %d to %d", untagPortList[i], vlanId);
            return -1;
        }
    }
#else
    int i, cport;
    sai_status_t rv;
    sai_attribute_t attr;
    sai_vlan_port_t vlan_port_list[MAX_NUM_PORTS];

    for (i = 0; i < portCount; i++) {
        cport = SaiXlateFPanelPortToChipPortNum(portList[i]);
        vlan_port_list[i].port_id = portList[cport];
        vlan_port_list[i].tagging_mode = SAI_VLAN_PORT_TAGGED;
    }
    for (i = 0; i < untagPortCount; i++) {
        cport = SaiXlateFPanelPortToChipPortNum(untagPortList[i]);
        vlan_port_list[portCount + i].port_id = port_list[cport];
        vlan_port_list[portCount + i].tagging_mode = SAI_VLAN_PORT_UNTAGGED;
    }
    rv = sai_vlan_api_tbl->add_ports_to_vlan(vlanId, portCount + untagPortCount, vlan_port_list);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Unable to add ports to vlan %d", vlanId);
        return -1;
    }
    //Set port pvid for untagged ports
    attr.id = SAI_PORT_ATTR_PORT_VLAN_ID;
    attr.value.u16 = vlanId;
    for (i = 0; i < untagPortCount; i++) {
        cport = SaiXlateFPanelPortToChipPortNum(untagPortList[i]);
        rv = sai_port_api_tbl->set_port_attribute(port_list[cport], &attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unable to set pvid of port %d to %d", untagPortList[i], vlanId);
            return -1;
        }
    }
#endif
#endif
    return 0;
}

int SaiUpdateVlanDeletePorts(int vlanId, int portCount, int untagPortCount, int *portList, int *untagPortList)
{
#ifdef SAI_BUILD
#ifdef MLNX_SAI
    int i, cport;
    sai_status_t rv;
    sai_attribute_t port_attr;

    for (i = 0; i < portCount; i++) {
        cport = SaiXlateFPanelPortToChipPortNum(portList[i]);
        rv = sai_vlan_api_tbl->remove_vlan_member(vlan_member_oid[vlanId][cport]);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unable to remove port %d from vlan %d", portList[i], vlanId);
            return -1;
        }
        //Clear object id
        vlan_member_oid[vlanId][cport] = 0;
    }
    for (i = 0; i < untagPortCount; i++) {
        cport = SaiXlateFPanelPortToChipPortNum(untagPortList[i]);
        rv = sai_vlan_api_tbl->remove_vlan_member(vlan_member_oid[vlanId][cport]);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unable to remove port %d from vlan %d", portList[i], vlanId);
            return -1;
        }
        //Clear object id
        vlan_member_oid[vlanId][cport] = 0;
        //Set port vid to default vlan id
        port_attr.id = SAI_PORT_ATTR_PORT_VLAN_ID;
        port_attr.value.u16 = DEFAULT_VLAN_ID;
        rv = sai_port_api_tbl->set_port_attribute(port_list[cport], &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unable to restore default pvid for port %d", untagPortList[i]);
            return -1;
        }
    }
#else
    int i, cport;
    sai_status_t rv;
    sai_attribute_t attr;
    sai_vlan_port_t vlan_port_list[MAX_NUM_PORTS];

    for (i = 0; i < portCount; i++) {
        cport = SaiXlateFPanelPortToChipPortNum(portList[i]);
        vlan_port_list[i].port_id = portList[cport];
        vlan_port_list[i].tagging_mode = SAI_VLAN_PORT_TAGGED;
    }
    for (i = 0; i < untagPortCount; i++) {
        cport = SaiXlateFPanelPortToChipPortNum(untagPortList[i]);
        vlan_port_list[portCount + i].port_id = port_list[cport];
        vlan_port_list[portCount + i].tagging_mode = SAI_VLAN_PORT_UNTAGGED;
    }
    rv = sai_vlan_api_tbl->remove_ports_from_vlan(vlanId, portCount + untagPortCount, vlan_port_list);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Unable to delete ports from vlan %d", vlanId);
        return -1;
    }
    //Set port pvid for untagged ports
    attr.id = SAI_PORT_ATTR_PORT_VLAN_ID;
    attr.value.u16 = DEFAULT_VLAN_ID;
    for (i = 0; i < untagPortCount; i++) {
        cport = SaiXlateFPanelPortToChipPortNum(untagPortList[i]);
        rv = sai_port_api_tbl->set_port_attribute(port_list[cport], &attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unable to set pvid of port %d to %d", untagPortList[i], vlanId);
            return -1;
        }
    }
#endif
#endif
    return 0;
}


