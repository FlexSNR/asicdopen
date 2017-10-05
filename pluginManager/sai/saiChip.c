#include <arpa/inet.h>
#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include <syslog.h>

#include <sai.h>
#include <saiacl.h>
#include <saihostintf.h>
#include <saineighbor.h>
#include <sainexthop.h>
#include <sainexthopgroup.h>
#include <saipolicer.h>
#include <sairoute.h>
#include <sairouter.h>
#include <sairouterintf.h>
#include <saistatus.h>
#include <saiswitch.h>
#include <saitypes.h>

#include "_cgo_export.h"
#include "pluginCommon.h"

/* Definition for API tables */
sai_switch_api_t* sai_switch_api_tbl = NULL;
extern sai_port_api_t* sai_port_api_tbl;
extern sai_hostif_api_t* sai_hostif_api_tbl;
extern sai_vlan_api_t* sai_vlan_api_tbl;
extern sai_fdb_api_t* sai_fdb_api_tbl;
extern sai_virtual_router_api_t* sai_vr_api_tbl;
extern sai_route_api_t* sai_route_api_tbl;
extern sai_router_interface_api_t* sai_rif_api_tbl;
extern sai_neighbor_api_t* sai_nbr_api_tbl;
extern sai_next_hop_api_t* sai_nh_api_tbl;
extern sai_next_hop_group_api_t* sai_nh_grp_api_tbl;
extern sai_buffer_api_t* sai_buffer_api_tbl;
extern sai_acl_api_t* sai_acl_api_tbl;
extern sai_policer_api_t* sai_policer_api_tbl;
extern sai_stp_api_t* sai_stp_api_tbl;
sai_lag_api_t* sai_lag_api_tbl = NULL;

#ifdef SAI_BUILD
sai_mac_t switchMacAddr;
extern sai_object_id_t port_list[MAX_NUM_PORTS];
extern sai_object_id_t globalVrId;
extern int maxSysPorts;

typedef struct LagMember {
    sai_object_id_t m_lag_id;
    sai_object_id_t m_lag_member_id;
} LagMember_t;
/*
 * One port should belong to one LAG.
 * Using MAX port number as MAX LAG number.
 */
sai_object_id_t lag_list[MAX_NUM_PORTS];
/* Store lag information for each port */
LagMember_t port_lag_list[MAX_NUM_PORTS];
#endif

//Forward declarations
int SaiCreateLinuxIfForAsicPorts(int ifMapCount, ifMapInfo_t *ifMap);
int SaiCreateLinuxIf(int port, char *ifName, sai_object_id_t *hostIfId);
int SaiDeleteLinuxIfForAsicPorts(int ifMapCount, ifMapInfo_t *ifMap);

/* List of call back functions for various events. Needs to be implemented */
extern void sai_port_state_cb (uint32_t count,
        sai_port_oper_status_notification_t *data);

static void sai_port_cb (uint32_t count,
        sai_port_event_notification_t *data)
{
}

extern void sai_fdb_cb (uint32_t count,
        sai_fdb_event_notification_data_t *data);

static void sai_switch_state_cb (sai_switch_oper_status_t switchstate)
{
}

static void sai_packet_cb (const void *buffer,
        sai_size_t buffer_size,
        uint32_t attr_count,
        const sai_attribute_t *attr_list)
{

}

static void sai_switch_shutdown_cb (void)
{
}


/* Adapter host provided services, currently un-implemented */
const char* profile_get_value(sai_switch_profile_id_t profile_id,
        const char* variable)
{
    return NULL;
}

int profile_get_next_value(sai_switch_profile_id_t profile_id,
        const char** variable,
        const char** value)
{
    return -1;
}

const service_method_table_t services =
{
    profile_get_value,
    profile_get_next_value
};

int SaiQueryAndPopulateAPITables()
{
#ifdef SAI_BUILD
    sai_status_t rv;
    //Initialize switch API
    rv = sai_api_query(SAI_API_SWITCH, (void**)&sai_switch_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_switch_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving switch api table. Aborting init.");
        return -1;
    }
    //Initialize host intf API
    rv = sai_api_query(SAI_API_HOST_INTERFACE, (void**)&sai_hostif_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_hostif_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving switch api table. Aborting init.");
        return -1;
    }
    //Initialize vlan API
    rv = sai_api_query(SAI_API_VLAN, (void**)&sai_vlan_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_vlan_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving vlan api table. Aborting init.");
        return -1;
    }
    //Initialize fdb API
    rv = sai_api_query(SAI_API_FDB, (void**)&sai_fdb_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_fdb_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving fdb api table. Aborting init.");
        return -1;
    }
    //Initialize port API
    rv = sai_api_query(SAI_API_PORT, (void**)&sai_port_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_port_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving port api table. Aborting init.");
        return -1;
    }
    //Initialize VR API
    rv = sai_api_query(SAI_API_VIRTUAL_ROUTER, (void**)&sai_vr_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_vr_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving virtual router api table. "
                "Aborting init.");
        return -1;
    }
    //Initialize route API
    rv = sai_api_query(SAI_API_ROUTE, (void**)&sai_route_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_route_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving route api table. Aborting init.");
        return -1;
    }
    //Initialize neighbor API
    rv = sai_api_query(SAI_API_NEIGHBOR, (void**)&sai_nbr_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_nbr_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving neighbor api table. "
                "Aborting init.");
        return -1;
    }
    //Initialize next hop API
    rv = sai_api_query(SAI_API_NEXT_HOP, (void**)&sai_nh_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_nh_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving next hop api table. "
                "Aborting init.");
        return -1;
    }
    //Initialize next hop group API
    rv = sai_api_query(SAI_API_NEXT_HOP_GROUP, (void**)&sai_nh_grp_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_nh_grp_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving next hop group api table. "
                "Aborting init.");
        return -1;
    }
    //Initialize router interface API
    rv = sai_api_query(SAI_API_ROUTER_INTERFACE, (void**)&sai_rif_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_rif_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving router intf api table. "
                "Aborting init.");
        return -1;
    }
    //Initialize buffer interface API
    rv = sai_api_query(SAI_API_BUFFERS, (void**)&sai_buffer_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_buffer_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving buffer api table. Aborting init.");
        return -1;
    }
    //Initialize acl API
    rv = sai_api_query(SAI_API_ACL, (void**)&sai_acl_api_tbl);
    if((rv != SAI_STATUS_SUCCESS) || (sai_acl_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving acl api table. Aborting init.");
        return -1;
    }
    //Initialize policer API
    rv = sai_api_query(SAI_API_POLICER, (void**)&sai_policer_api_tbl);
    if((rv != SAI_STATUS_SUCCESS) || (sai_policer_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving policer api table. Aborting init.");
        return -1;
    }
    //Initialize lag API
    rv = sai_api_query(SAI_API_LAG, (void **)&sai_lag_api_tbl);
    if((rv != SAI_STATUS_SUCCESS) || (sai_lag_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving lag api table. Aborting init.");
        return -1;
    }
    //Initialize stp API
    rv = sai_api_query(SAI_API_STP, (void **)&sai_stp_api_tbl);
    if((rv != SAI_STATUS_SUCCESS) || (sai_stp_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving stp api table. Aborting init.");
        return -1;
    }
#endif
    return 0;
}

void SaiInitAPILog()
{
#ifdef SAI_BUILD
    sai_log_set(SAI_API_SWITCH,                 SAI_LOG_NOTICE);
    sai_log_set(SAI_API_VIRTUAL_ROUTER,         SAI_LOG_NOTICE);
    sai_log_set(SAI_API_PORT,                   SAI_LOG_NOTICE);
    sai_log_set(SAI_API_FDB,                    SAI_LOG_NOTICE);
    sai_log_set(SAI_API_VLAN,                   SAI_LOG_NOTICE);
    sai_log_set(SAI_API_HOST_INTERFACE,         SAI_LOG_NOTICE);
    sai_log_set(SAI_API_MIRROR,                 SAI_LOG_NOTICE);
    sai_log_set(SAI_API_ROUTER_INTERFACE,       SAI_LOG_NOTICE);
    sai_log_set(SAI_API_NEIGHBOR,               SAI_LOG_NOTICE);
    sai_log_set(SAI_API_NEXT_HOP,               SAI_LOG_NOTICE);
    sai_log_set(SAI_API_NEXT_HOP_GROUP,         SAI_LOG_NOTICE);
    sai_log_set(SAI_API_ROUTE,                  SAI_LOG_NOTICE);
    sai_log_set(SAI_API_LAG,                    SAI_LOG_NOTICE);
    sai_log_set(SAI_API_POLICER,                SAI_LOG_NOTICE);
    sai_log_set(SAI_API_TUNNEL,                 SAI_LOG_NOTICE);
    sai_log_set(SAI_API_QUEUE,                  SAI_LOG_NOTICE);
    sai_log_set(SAI_API_SCHEDULER,              SAI_LOG_NOTICE);
    sai_log_set(SAI_API_WRED,                   SAI_LOG_NOTICE);
    sai_log_set(SAI_API_QOS_MAPS,               SAI_LOG_NOTICE);
    sai_log_set(SAI_API_BUFFERS,                SAI_LOG_NOTICE);
    sai_log_set(SAI_API_SCHEDULER_GROUP,        SAI_LOG_NOTICE);
    sai_log_set(SAI_API_ACL,                    SAI_LOG_NOTICE);
    sai_log_set(SAI_API_STP,                    SAI_LOG_NOTICE);
#endif
}

int SaiInit(int bootMode, int ifMapCount, ifMapInfo_t *ifMap, uint8_t *macAddr)
{
#ifdef SAI_BUILD
    int i, netDevType;
    sai_status_t rv;
    sai_attribute_t attr;
    sai_attribute_t attrs[3];
    sai_object_id_t cpuPort;
    sai_switch_notification_t cb_list;

    //Save switch mac addr locally
    memcpy(switchMacAddr, macAddr, MAC_ADDR_LEN);
    rv = sai_api_initialize(0, &services);
    if (rv !=  SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failure initializing SAI switch APIs. Aborting init.");
        return -1;
    }

    //Populate API tables
    rv = SaiQueryAndPopulateAPITables();
    if (rv < 0) {
        syslog(LOG_ERR, "Failed to query SAI API tables. Aborting init.");
        return -1;
    }

    SaiInitAPILog();

    //Initialize switch
    memset (&cb_list, 0, sizeof(sai_switch_notification_t));
    cb_list.on_switch_state_change = sai_switch_state_cb;
    cb_list.on_fdb_event = sai_fdb_cb;
    cb_list.on_port_state_change = sai_port_state_cb;
    cb_list.on_switch_shutdown_request = sai_switch_shutdown_cb;
    cb_list.on_port_event = sai_port_cb;
    cb_list.on_packet_event = sai_packet_cb;
    if (sai_switch_api_tbl->initialize_switch != NULL) {
        rv = sai_switch_api_tbl->initialize_switch (0, "", "", &cb_list);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failure initalizing switch. Aborting init");
        }
    }

    //Connect to SDK
    rv = sai_switch_api_tbl->connect_switch(0, "", &cb_list);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failure connecting to SDK. Aborting init");
    }

    // Added following attribute during debugging session
    attr.id = SAI_SWITCH_ATTR_SRC_MAC_ADDRESS;
    memcpy(attr.value.mac, switchMacAddr, 6);
    rv = sai_switch_api_tbl->set_switch_attribute(&attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to set MAC address to switch 0x%x\n", rv);
        return -1;
    }
    // end of attribute

    //Cache max number of ports
    attr.id = SAI_SWITCH_ATTR_PORT_NUMBER;
    rv = sai_switch_api_tbl->get_switch_attribute(1, &attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failure retrieving port id's. Aborting init");
        return -1;
    }
    //Cache max number of ports
    maxSysPorts = attr.value.u32;
    syslog(LOG_INFO, "Retrieved port id's after doing portNum. "
            "Total ports count = %d\n", maxSysPorts);

    //Retrieve port object ids
    attr.id = SAI_SWITCH_ATTR_PORT_LIST;
    attr.value.objlist.count = MAX_NUM_PORTS;
    /*
     * port_list + 1 used below so that
     * port numbers can be used to directly index array
     */
    attr.value.objlist.list = port_list + 1;
    rv = sai_switch_api_tbl->get_switch_attribute(1, &attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failure retrieving port id's after doing portList. "
                "Aborting init");
        return -1;
    }

    // Map front panel ports to linux interfaces
    rv = SaiCreateLinuxIfForAsicPorts(ifMapCount, ifMap);
    if (rv < 0) {
        syslog(LOG_ERR, "Failed to map asic ports to linux interfaces");
        return -1;
    }

    // Disable all ports
    for (i = 1; i <= maxSysPorts; i++) {
        attr.id = SAI_PORT_ATTR_ADMIN_STATE;
        attr.value.booldata = false;
        rv = sai_port_api_tbl->set_port_attribute(port_list[i], &attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set port admin state for port %d", i);
            return -1;
        }
    }

    /* Retrieve global vr id */
    attr.id = SAI_SWITCH_ATTR_DEFAULT_VIRTUAL_ROUTER_ID;
    rv = sai_switch_api_tbl->get_switch_attribute(1, &attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to retrieve global VR instance. "
                "Aborting init");
        return -1;
    }
    globalVrId = attr.value.oid;
    //Initialize port data structure
    SaiPortInit(ifMapCount, ifMap);

    //Initialize L3 data
    SaiL3Init();

    /* init CoPP feature */
    if (SaiCoPPConfig() == SAI_STATUS_SUCCESS) {
        syslog(LOG_INFO, "Asicd CoPP config done.!");
    } else {
        syslog(LOG_ERR, "CoPP SAI config failed.");
    }
    syslog(LOG_INFO, "Asicd SAI initialization complete !");

    /* Init ACL struct */
    SaiInitAcl();

#endif
    return 0;
}

int SaiDeinit(int cacheSwState)
{
#ifdef SAI_BUILD
    int rv;

    if (cacheSwState) {
        //Code to enable warmboot state sync
    }
    rv = SaiDeleteLinuxIfForAsicPorts(ALL_INTERFACES, NULL);
    if (rv < 0) {
        syslog(LOG_ERR, "Failed to delete mapping of asic ports to "
                "linux interfaces");
        return -1;
    }

    // Release all data release
    sai_switch_api_tbl->shutdown_switch(false);
#endif
    return 0;
}

uint64_t SaiCreateLag(int hashType, int portCount, int *ports)
{
#ifdef SAI_BUILD
    sai_object_id_t lagId;
    sai_status_t rv;
    int lag_id;
    int i;

    syslog(LOG_INFO, "SaiCreateLag=> hashType: %d, portCount: %d", hashType,
            portCount);
    rv = sai_lag_api_tbl->create_lag(&lagId, 0, NULL);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to create LAG, lid:%lx", lagId);
        return -1;
    }

    lag_id = (int)lagId;
    syslog(LOG_ERR, "Create an empty LAG, lid: %d", lag_id);

    for (i = 0; i < portCount; i++) {
        SaiAddLagMember(lag_id, ports[i]);
    }

    lag_list[lag_id] = lagId;

    return lagId;
#else
    return 0;
#endif
}

int SaiDeleteLag(int lagId)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    sai_object_id_t lag_id;

    syslog(LOG_INFO, "SaiDeleteLag=> lagId: %d", lagId);

    lag_id = lag_list[lagId];

    rv = sai_lag_api_tbl->remove_lag(lag_id);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to remove LAG, lid:%d", lagId);
        return -1;
    }

    syslog(LOG_INFO, "Remove LAG, lid: %d", lagId);

    lag_list[lagId] = 0;
#endif
    return 0;
}

int SaiUpdateLag(int lagId, int hashType, int oldPortCount, int *oldPorts,
        int portCount, int *ports)
{
#ifdef SAI_BUILD
    int i, j, is_found;
    sai_object_id_t lag_id;

    syslog(LOG_INFO, "SaiUpdateLag=> lagId: %d, old: %d, new: %d", lagId,
            oldPortCount, portCount);

    /* Check added port */
    for (i = 0; i < portCount; i++) {
        is_found = 0;
        for (j = 0; j < oldPortCount; j++) {
            if (ports[i] == oldPorts[j]) {
                is_found = 1;
                break;
            }
        }
        if (is_found == 0) {
            SaiAddLagMember(lag_list[lagId], ports[i]);
        }
    }

    /* Check removed port */
    for (i = 0; i < oldPortCount; i++) {
        is_found = 0;
        for (j = 0; j < portCount; j++) {
            if (oldPorts[i] == ports[j]) {
                is_found = 1;
                break;
            }
        }
        if (is_found == 0) {
            SaiRemoveLagMember(lag_list[lagId], oldPorts[i]);
        }
    }
#endif
    return 0;
}

int SaiAddLagMember(sai_object_id_t lagId, int portId)
{
#ifdef SAI_BUILD
    sai_attribute_t attrs[2];
    sai_object_id_t lag_member_id;
    sai_status_t rv;

    syslog(LOG_INFO, "Add port to LAG: %lx => %d", lagId, portId);

    attrs[0].id = SAI_LAG_MEMBER_ATTR_LAG_ID;
    attrs[0].value.oid = lagId;

    attrs[1].id = SAI_LAG_MEMBER_ATTR_PORT_ID;
    attrs[1].value.oid = port_list[portId];

    rv = sai_lag_api_tbl->create_lag_member(&lag_member_id, 2, attrs);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to add member %lx (%d) to LAG %lx",
                port_list[portId], portId, lagId);
        return -1;
    }

    syslog(LOG_INFO, "Add member %lx (%d) to LAG %lx", lag_member_id,
            portId, lagId);

    port_lag_list[portId].m_lag_id = lagId;
    port_lag_list[portId].m_lag_member_id = lag_member_id;
#endif
    return 0;
}

int SaiRemoveLagMember(sai_object_id_t lagId, int portId)
{
#ifdef SAI_BUILD
    sai_status_t rv;

    syslog(LOG_INFO, "Remove port from LAG: %lx => %d", lagId, portId);
    rv = sai_lag_api_tbl->remove_lag_member(port_lag_list[portId].m_lag_member_id);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to remove member %lx (%d) from LAG %lx",
                port_list[portId], portId, lagId);
        return -1;
    }

    syslog(LOG_INFO, "Remove member %lx (%d) from LAG %lx", port_list[portId],
            portId, lagId);

    port_lag_list[portId].m_lag_id = 0;
    port_lag_list[portId].m_lag_member_id = 0;
#endif
    return 0;
}

int SaiRestoreLagDB()
{
    return 0;
}

void SaiDevShell(void)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    sai_attribute_t attr;

    // Enable driver shell
    attr.id = SAI_SWITCH_ATTR_SWITCH_SHELL_ENABLE;
    attr.value.booldata = 1;  // TRUE
    rv = sai_switch_api_tbl->set_switch_attribute(&attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to enable driver shell");
    }
    // end of attribute
#endif
}
