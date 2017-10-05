#ifndef SAI_COPP_H
#define SAI_COPP_H

#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <syslog.h>

#include <sai.h>
#include <saiqueue.h>
#include <saistatus.h>
#include <saitypes.h>

#include "pluginCommon.h"

/* TC based on 8 queues for
 * the CPU.
 */
#define SAI_QOS_PRIORITY_0 0
#define SAI_QOS_PRIORITY_1 1
#define SAI_QOS_PRIORITY_2 2
#define SAI_QOS_PRIORITY_3 3
#define SAI_QOS_PRIORITY_4 4
#define SAI_QOS_PRIORITY_5 5
#define SAI_QOS_PRIORITY_6 6
#define SAI_QOS_PRIORITY_7 7
#define COPP_TABLE_ATTR 5
#define ACL_TABLE_ATTR 12
// size(not yet supp), priority, acl stage, Bind point
#define ACL_DEFAULT_TABLE_ATTR 3
#define COPP_ACL_TABLE_SIZE 128
#define ACL_TABLE_SIZE 1024  //CHECK LIMIT FOR SAI
#define ACL_MAX_TABLE 256
#define DEFAULT_TABLE_PRIORITY 5

typedef struct SaiTblAttr {
	int id;
	int enable;
} SaiTblAttr_t;

/************* COPP structs *******/
/*
 * Match fields to be enabled/disabled
 * in CoPP
 */
static const SaiTblAttr_t CoppAclTblAttr[] = {
{SAI_ACL_TABLE_ATTR_FIELD_IP_PROTOCOL,1},
{SAI_ACL_TABLE_ATTR_FIELD_L4_DST_PORT, 1},
{SAI_ACL_TABLE_ATTR_FIELD_ETHER_TYPE, 1},
{SAI_ACL_TABLE_ATTR_FIELD_DST_IP, 1},
{SAI_ACL_TABLE_ATTR_FIELD_DST_MAC, 1},
//{SAI_ACL_TABLE_ATTR_FIELD_SRC_IPv6, 1}, Not supported
//{SAI_ACL_TABLE_ATTR_FIELD_DST_IPv6, 1}  Not supported
};

typedef struct CoppData_s {
	int CpuQueueNum;
	int priority;
	sai_object_id_t policer_id;
	uint64_t policer_cir;
	uint64_t policer_cbs;
	uint64_t policer_pbs;
} CoppData_t;

typedef struct CoppGlobalData_s {
	sai_object_id_t  table_id;
	sai_object_id_t  cpu_queue_count;
	sai_object_id_t  cpu_port;
        sai_object_list_t sai_cpu_queue_list;
} CoppGlobalData;

/********** ACL structs **********/
// Update ACL_TABLE_ATTR for every new field added.
static const SaiTblAttr_t AclIngressTblAttr[] = {
    {SAI_ACL_TABLE_ATTR_FIELD_DST_MAC, 1},
    {SAI_ACL_TABLE_ATTR_FIELD_SRC_MAC, 1},
    {SAI_ACL_TABLE_ATTR_FIELD_ETHER_TYPE, 1},
    {SAI_ACL_TABLE_ATTR_FIELD_IP_PROTOCOL, 1},
    {SAI_ACL_TABLE_ATTR_FIELD_DST_IP, 1},
    {SAI_ACL_TABLE_ATTR_FIELD_SRC_IP, 1},
    {SAI_ACL_TABLE_ATTR_FIELD_L4_DST_PORT, 1},
    {SAI_ACL_TABLE_ATTR_FIELD_L4_SRC_PORT, 1},
    {SAI_ACL_TABLE_ATTR_FIELD_RANGE, 1},
    {SAI_ACL_TABLE_ATTR_FIELD_IN_PORTS, 1},
    {SAI_ACL_TABLE_ATTR_FIELD_IN_PORT, 1},
    {SAI_ACL_TABLE_ATTR_FIELD_OUT_PORT, 1}
};

typedef enum AclType_s {
	Acl_ingress,
	Acl_egress,
	COPP,
} AclType;

typedef struct AclGlobalData_s {
	sai_object_id_t ingress_table_id;
	sai_object_id_t egress_table_id;
	AclType 	aclType;
} AclGlobalData;

typedef struct AclEntryData_s {
    char* aclRuleName;
    sai_object_id_t acl_entry_id;
    sai_object_id_t acl_range_id;
    int used;
} AclEntryData;

typedef struct AclTableData_s {
    char* aclName;
    sai_object_id_t acl_table_id;
    AclEntryData entryIdList[ACL_TABLE_SIZE];
    int direction;
    int* portList;
    int length;
    int used;
} AclTableData;

int SaiCoPPInit();
int SaiCoPPConfig();
#endif
