#include "saiAcl.h"

sai_acl_api_t *sai_acl_api_tbl = NULL;
extern sai_port_api_t* sai_port_api_tbl;

extern sai_object_id_t port_list[MAX_NUM_PORTS];
AclTableData aclTableIdList[ACL_MAX_TABLE];
int aclTableCount = 0;


/* Initialise ACL structs */
int SaiInitAcl()
{
#ifdef SAI_BUILD
    memset(aclTableIdList, 0, sizeof(aclTableIdList));
#endif
    return 0;
}

int SaiProcessAcl(char* aclName, int aclType, aclRule acl,
        int length, int* portList, int direction)
{
#ifdef SAI_BUILD
    sai_object_id_t acl_table_id = 0;
    sai_object_id_t acl_entry_id = 0;
    sai_object_id_t acl_range_id = 0;
    int rv;
    AclTableData* curAclTablePtr = NULL;

    if (aclTableCount == ACL_MAX_TABLE) {
        syslog(LOG_ERR, "Acl: Can't create acl table. "
                "Reach the maximum limitation in acl %s.", aclName);
        return -1;
    }

    rv = SaiSearchAclTableId(aclName, direction, &acl_table_id,
            &curAclTablePtr);
    if (rv == -1) {
        rv = SaiCreateAclTable(aclName, direction, aclType, &acl_table_id);
        if (rv != 0) {
            syslog(LOG_ERR, "Acl: Fail to create acl table in acl %s.",
                    aclName);
            return -1;
        }

        rv = SaiPushAclTableId(aclName, acl_table_id, direction,
                &curAclTablePtr);
        if (rv != 0) {
            syslog(LOG_ERR, "Acl: Fail to store acl table data in acl %s.",
                    aclName);
            return -1;
        }
    }

    rv = SaiCreateAclEntry(acl, acl_table_id, &acl_entry_id, &acl_range_id);
    if (rv != 0) {
        syslog(LOG_ERR, "Acl: Fail to create acl entry in acl %s and "
                "acl rule %s.", aclName, acl.ruleName);
        return -1;
    }

    rv = SaiPushAclEntryId(acl.ruleName, curAclTablePtr, acl_entry_id,
            acl_range_id);
    if (rv != 0) {
         syslog(LOG_ERR, "Acl: Fail to store acl entry data in acl %s and "
                 "acl rule %s.", aclName, acl.ruleName);
        return -1;
    }

    rv = SaiSetAclToInterface(length, portList, direction, acl_table_id);
    if (rv != 0) {
        syslog(LOG_ERR, "Acl: Fail to bind acl table %s to ports.", aclName);
        return -1;
    }

    curAclTablePtr->portList = (int *) malloc(sizeof(int) * length);
    if (curAclTablePtr->portList == NULL) {
        syslog(LOG_CRIT, "Acl: Not enough system memory to use.");
        return -1;
    }
    memcpy(curAclTablePtr->portList, portList, sizeof(int) * length);
    curAclTablePtr->length = length;
#endif
    return 0;
}

int SaiDeleteAcl(char* aclName, int direction)
{
#ifdef SAI_BUILD
    int rv = 0, i = 0;
    AclTableData* curAclTablePtr = NULL;
    sai_status_t sai_rv;
    sai_object_id_t acl_table_id = 0;
    sai_object_id_t acl_entry_id = 0;
    sai_object_id_t acl_range_id = 0;

    rv = SaiSearchAclTableId(aclName, direction, &acl_table_id,
            &curAclTablePtr);
    if (rv != 0) {
        syslog(LOG_ERR, "ACL: Can't find the ACL %s", aclName);
        return -1;
    }

    for(i = 0; i < ACL_TABLE_SIZE; i++) {
        if (curAclTablePtr->entryIdList[i].used == 0) {
            continue;
        }
        acl_range_id = curAclTablePtr->entryIdList[i].acl_range_id;
        if (acl_range_id != 0) {
            sai_rv = sai_acl_api_tbl->remove_acl_range(acl_range_id);
            if (sai_rv != SAI_STATUS_SUCCESS) {
                syslog(LOG_ERR, "ACL: Fail to remove ACL %s ACL range",
                        curAclTablePtr->entryIdList[i].aclRuleName);
                return -1;
            }
        }

        acl_entry_id = curAclTablePtr->entryIdList[i].acl_entry_id;
        sai_rv = sai_acl_api_tbl->delete_acl_entry(acl_entry_id);
        if (sai_rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "ACL: Fail to remove ACL %s ACL entry",
                    curAclTablePtr->entryIdList[i].aclRuleName);
            return -1;
        }

        free(curAclTablePtr->entryIdList[i].aclRuleName);
        curAclTablePtr->entryIdList[i].aclRuleName = NULL;
        curAclTablePtr->entryIdList[i].acl_entry_id = 0;
        curAclTablePtr->entryIdList[i].acl_range_id = 0;
        curAclTablePtr->entryIdList[i].used = 0;
    }

    sai_rv = sai_acl_api_tbl->delete_acl_table(curAclTablePtr->acl_table_id);
    if (sai_rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "ACL: Fail to remove ACL %s ACL table",
                curAclTablePtr->aclName);
        return -1;
    }
    free(curAclTablePtr->aclName);
    curAclTablePtr->aclName = NULL;
    free(curAclTablePtr->portList);
    curAclTablePtr->portList = NULL;
    curAclTablePtr->length = 0;
    curAclTablePtr->acl_table_id = 0;
    curAclTablePtr->used = 0;
#endif
    return 0;
}

int SaiDeleteAclRuleFromAcl(char* aclName, aclRule acl, int length,
        int* portList, int direction)
{
#ifdef SAI_BUILD
    int rv = 0, i = 0;
    int strLen = 0;
    AclTableData* curAclTablePtr = NULL;
    sai_status_t sai_rv;
    sai_object_id_t acl_table_id = 0;
    sai_object_id_t acl_entry_id = 0;
    sai_object_id_t acl_range_id = 0;

    rv = SaiSearchAclTableId(aclName, direction, &acl_table_id,
            &curAclTablePtr);
    if (rv != 0) {
        syslog(LOG_ERR, "ACL: Can't find the ACL %s", aclName);
        return -1;
    }

    for (i = 0 ; i < ACL_TABLE_SIZE; i++) {
        if (curAclTablePtr->entryIdList[i].used == 0) {
            continue;
        }
        strLen = strlen(acl.ruleName);
        if (strncmp(curAclTablePtr->entryIdList[i].aclRuleName, acl.ruleName,
                    strLen) != 0) {
            continue;
        }

        acl_range_id = curAclTablePtr->entryIdList[i].acl_range_id;
        if (acl_range_id != 0) {
            sai_rv = sai_acl_api_tbl->remove_acl_range(acl_range_id);
            if (sai_rv != SAI_STATUS_SUCCESS) {
                syslog(LOG_ERR, "ACL: Fail to remove ACL %s ACL range",
                        curAclTablePtr->entryIdList[i].aclRuleName);
                return -1;
            }
        }

        acl_entry_id = curAclTablePtr->entryIdList[i].acl_entry_id;
        sai_rv = sai_acl_api_tbl->delete_acl_entry(acl_entry_id);
        if (sai_rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "ACL: Fail to remove ACL %s ACL entry",
                    curAclTablePtr->entryIdList[i].aclRuleName);
            return -1;
        }

        free(curAclTablePtr->entryIdList[i].aclRuleName);
        curAclTablePtr->entryIdList[i].aclRuleName = NULL;
        curAclTablePtr->entryIdList[i].acl_entry_id = 0;
        curAclTablePtr->entryIdList[i].acl_range_id = 0;
        curAclTablePtr->entryIdList[i].used = 0;
    }
#endif
    return 0;
}

int UpdateAclRule(char* aclName, aclRule acl)
{
#ifdef SAI_BUILD
    // IN, OUT
    sai_object_id_t acl_table_id[2] = {0, 0};
    sai_object_id_t acl_entry_id = 0;
    sai_object_id_t acl_range_id = 0;
    sai_status_t sai_rv;
    int rv = 0, i = 0, j = 0;
    int strLen = 0;
    // IN, OUT
    AclTableData* curAclTablePtr[2] = {NULL, NULL};

    SaiSearchAclTableId(aclName, AclIn, &acl_table_id[0], &curAclTablePtr[0]);
    SaiSearchAclTableId(aclName, AclOut, &acl_table_id[1], &curAclTablePtr[1]);
    if (curAclTablePtr[0] == NULL && curAclTablePtr[1] == NULL) {
        syslog(LOG_ERR, "ACL: Can't find the ACL %s", aclName);
        return -1;
    }

    for (j = 0; j < 2; j++) {
        if (curAclTablePtr[j] == NULL) {
            continue;
        }

        for (i = 0 ; i < ACL_TABLE_SIZE; i++) {
            if (curAclTablePtr[j]->entryIdList[i].used == 0) {
                continue;
            }
            strLen = strlen(acl.ruleName);
            if (strncmp(curAclTablePtr[j]->entryIdList[i].aclRuleName,
                        acl.ruleName, strLen) != 0) {
                continue;
            }

            acl_range_id = curAclTablePtr[j]->entryIdList[i].acl_range_id;
            if (acl_range_id != 0) {
                sai_rv = sai_acl_api_tbl->remove_acl_range(acl_range_id);
                if (sai_rv != SAI_STATUS_SUCCESS) {
                    syslog(LOG_ERR, "ACL: Fail to remove ACL %s ACL range",
                            curAclTablePtr[j]->entryIdList[i].aclRuleName);
                    return -1;
                }
            }

            acl_entry_id = curAclTablePtr[j]->entryIdList[i].acl_entry_id;
            sai_rv = sai_acl_api_tbl->delete_acl_entry(acl_entry_id);
            if (sai_rv != SAI_STATUS_SUCCESS) {
                syslog(LOG_ERR, "ACL: Fail to remove ACL %s ACL entry",
                        curAclTablePtr[j]->entryIdList[i].aclRuleName);
                return -1;
            }

            free(curAclTablePtr[j]->entryIdList[i].aclRuleName);
            curAclTablePtr[j]->entryIdList[i].aclRuleName = NULL;
            curAclTablePtr[j]->entryIdList[i].acl_entry_id = 0;
            curAclTablePtr[j]->entryIdList[i].acl_range_id = 0;
            curAclTablePtr[j]->entryIdList[i].used = 0;
        }

        acl_entry_id = 0;
        acl_range_id = 0;
        rv = SaiCreateAclEntry(acl, acl_table_id[j], &acl_entry_id,
                &acl_range_id);
        if (rv != 0) {
            return -1;
        }

        rv = SaiPushAclEntryId(acl.ruleName, curAclTablePtr[j], acl_entry_id,
                acl_range_id);
        if (rv != 0) {
            return -1;
        }
    }
#endif
    return 0;
}

int SaiCreateAclTable(char* aclName, int direction, int aclType,
        sai_object_id_t* table_id)
{
#ifdef SAI_BUILD
    const int MAX_ACL_ATTR_NUM = ACL_TABLE_ATTR + ACL_DEFAULT_TABLE_ATTR;
    int status, index, i;
    sai_status_t rv;

    sai_attribute_t attr[ACL_TABLE_ATTR + ACL_DEFAULT_TABLE_ATTR];

    attr[0].id = SAI_ACL_TABLE_ATTR_STAGE;
    if (direction == AclIn) {
        attr[0].value.s32 = SAI_ACL_STAGE_INGRESS;
    } else if (direction == AclOut) {
        attr[0].value.s32 = SAI_ACL_STAGE_EGRESS;
    }
    attr[1].id = SAI_ACL_TABLE_ATTR_PRIORITY;
    attr[1].value.s32 = SAI_SWITCH_ATTR_ACL_TABLE_MINIMUM_PRIORITY;
    /* Now only support bind ACL on port */
    attr[2].id = SAI_ACL_TABLE_ATTR_BIND_POINT;
    attr[2].value.s32 = SAI_ACL_BIND_POINT_PORT;

    /**
     * Brcm SAI not yet support set ACL table size
     */
    /*
     * attr[2].id = SAI_ACL_TABLE_ATTR_SIZE;
     * attr[2].value.s32 = ACL_TABLE_SIZE;
    */

    for (i = ACL_DEFAULT_TABLE_ATTR; i < MAX_ACL_ATTR_NUM; i++) {
        index = i - ACL_DEFAULT_TABLE_ATTR;
        attr[i].id = AclIngressTblAttr[index].id;
        attr[i].value.booldata = true;
    }

    /* create acl table */
    rv = sai_acl_api_tbl->create_acl_table(table_id, MAX_ACL_ATTR_NUM,
            attr);
    if(rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "ACL: Failed to create ACL table. err %x", rv);
        return SAI_STATUS_FAILURE;
    }
    syslog(LOG_DEBUG, "ACL: acl table created with id %lld ",
            (long long int)table_id);

    return SAI_STATUS_SUCCESS;
#endif
	return 0;
}

int SaiCreateAclEntry(aclRule acl, sai_object_id_t acl_table_id,
        sai_object_id_t* acl_entry_id, sai_object_id_t* acl_range_id)
{
#ifdef SAI_BUILD
    int attrCount = 0, max = 0, min = 0;
    sai_status_t rv;
    sai_object_id_t entry_id;
    sai_object_id_t range_id;
    sai_attribute_t entry_attrs[15];
    sai_attribute_t range_attrs[2];
    sai_mac_t macMask = {0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF};

    /* ACL table */
    entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_TABLE_ID;
    entry_attrs[attrCount].value.oid = acl_table_id;
    attrCount++;

    /* Entry priority */
    entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_PRIORITY;
    entry_attrs[attrCount].value.u32 = 1;
    attrCount++;

    /* ACL action: Deny or permit */
    if (acl.action == AclDeny) {
        entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_PACKET_ACTION;
        entry_attrs[attrCount].value.aclaction.parameter.s32 =
            SAI_PACKET_ACTION_DROP;
        entry_attrs[attrCount].value.aclaction.enable = true;
    } else if (acl.action == AclAllow) {
        entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_PACKET_ACTION;
        entry_attrs[attrCount].value.aclaction.parameter.s32 =
            SAI_PACKET_ACTION_FORWARD;
        entry_attrs[attrCount].value.aclaction.enable = true;
    }
    attrCount++;

    /* Src and Dst Port */
    if (acl.srcport != 0) {
        entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_FIELD_IN_PORT;
        entry_attrs[attrCount].value.aclfield.data.oid = port_list[acl.srcport];
        attrCount++;
    }

    if (acl.dstport != 0) {
        entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_FIELD_OUT_PORT;
        entry_attrs[attrCount].value.aclfield.data.oid = port_list[acl.dstport];
        attrCount++;
    }

    /* Src and dst MAC */
    if (acl.sourceMac != NULL) {
        entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_FIELD_SRC_MAC;
        memcpy(entry_attrs[attrCount].value.aclfield.data.mac, acl.sourceMac,
                sizeof(sai_mac_t));
        memcpy(entry_attrs[attrCount].value.aclfield.mask.mac, macMask,
                sizeof(sai_mac_t));
        attrCount++;
    }

    if (acl.destMac != NULL) {
        entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_FIELD_DST_MAC;
        memcpy(entry_attrs[attrCount].value.aclfield.data.mac, acl.destMac,
                sizeof(sai_mac_t));
        memcpy(entry_attrs[attrCount].value.aclfield.mask.mac, macMask,
                sizeof(sai_mac_t));
        attrCount++;
    }

    /* Protocol */
    if (acl.proto != -1) {
        entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_FIELD_IP_PROTOCOL;
        entry_attrs[attrCount].value.aclfield.data.u8 = acl.proto;
        entry_attrs[attrCount].value.aclfield.mask.u8 = 0xFF;
        attrCount++;
    }

    /* Src and Dst IP */
    if (acl.sourceIp != NULL && acl.sourceMask != NULL) {
        entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_FIELD_SRC_IP;
        entry_attrs[attrCount].value.aclfield.data.ip4 =
            htonl((uint32_t)(acl.sourceIp[3]) |
                  (uint32_t)(acl.sourceIp[2]) << 8 |
                  (uint32_t)(acl.sourceIp[1]) << 16 |
                  (uint32_t)(acl.sourceIp[0]) << 24);
        entry_attrs[attrCount].value.aclfield.mask.ip4 =
            htonl((uint32_t)(acl.sourceMask[3]) |
                  (uint32_t)(acl.sourceMask[2]) << 8 |
                  (uint32_t)(acl.sourceMask[1]) << 16 |
                  (uint32_t)(acl.sourceMask[0]) << 24);
        attrCount++;
    }

    if (acl.destIp != NULL && acl.destMask != NULL) {
        entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_FIELD_DST_IP;
        entry_attrs[attrCount].value.aclfield.data.ip4 =
            htonl((uint32_t)(acl.destIp[3]) |
                  (uint32_t)(acl.destIp[2]) << 8 |
                  (uint32_t)(acl.destIp[1]) << 16 |
                  (uint32_t)(acl.destIp[0]) << 24);
        entry_attrs[attrCount].value.aclfield.mask.ip4 =
            htonl((uint32_t)(acl.destMask[3]) |
                  (uint32_t)(acl.destMask[2]) << 8 |
                  (uint32_t)(acl.destMask[1]) << 16 |
                  (uint32_t)(acl.destMask[0]) << 24);
        attrCount++;
    }

    /* L4Src and L4Dst Port (or range Port) */
    *acl_range_id = 0;
    if (acl.l4PortMatch == AclEq) {
        if (acl.l4SrcPort != 0) {
            entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_FIELD_L4_SRC_PORT;
            entry_attrs[attrCount].value.aclfield.data.u16 =
                acl.l4SrcPort;
            entry_attrs[attrCount].value.aclfield.mask.u16 = 0xFFFF;
            attrCount++;
        }

        if (acl.l4DstPort != 0) {
            entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_FIELD_L4_DST_PORT;
            entry_attrs[attrCount].value.aclfield.data.u16 =
                acl.l4DstPort;
            entry_attrs[attrCount].value.aclfield.mask.u16 = 0xFFFF;
            attrCount++;
        }
    } else if (acl.l4PortMatch == AclRange) {
        if (acl.l4SrcPort != 0) {
            if (acl.l4MinPort != 0) {
                min = acl.l4MinPort;
                max = acl.l4SrcPort;
            }

            if (acl.l4MaxPort != 0) {
                min = acl.l4SrcPort;
                max = acl.l4MaxPort;
            }

            range_attrs[0].id = SAI_ACL_RANGE_ATTR_TYPE;
            range_attrs[0].value.s32 = SAI_ACL_RANGE_L4_SRC_PORT_RANGE;
            range_attrs[1].id = SAI_ACL_RANGE_ATTR_LIMIT;
            range_attrs[1].value.u32range.min = min;
            range_attrs[1].value.u32range.max = max;
            rv = sai_acl_api_tbl->create_acl_range(&range_id, 2, range_attrs);
            if (rv != SAI_STATUS_SUCCESS) {
                syslog(LOG_ERR, "Acl: Fail to create L4 port range acl in "
                        "acl rule %s.", acl.ruleName);
                return -1;
            }
            *acl_range_id = range_id;
            entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_FIELD_RANGE;
            entry_attrs[attrCount].value.aclfield.data.objlist.list = &range_id;
            entry_attrs[attrCount].value.aclfield.data.objlist.count = 1;
            attrCount++;
        }

        if (acl.l4DstPort != 0) {
            if (acl.l4MinPort != 0) {
                min = acl.l4MinPort;
                max = acl.l4DstPort;
            }

            if (acl.l4MaxPort != 0) {
                min = acl.l4DstPort;
                max = acl.l4MaxPort;
            }

            range_attrs[0].id = SAI_ACL_RANGE_ATTR_TYPE;
            range_attrs[0].value.s32 = SAI_ACL_RANGE_L4_DST_PORT_RANGE;
            range_attrs[1].id = SAI_ACL_RANGE_ATTR_LIMIT;
            range_attrs[1].value.u32range.min = min;
            range_attrs[1].value.u32range.max = max;
            rv = sai_acl_api_tbl->create_acl_range(&range_id, 2, range_attrs);
            if (rv != SAI_STATUS_SUCCESS) {
                syslog(LOG_ERR, "Acl: Fail to create L4 port range acl in "
                        "acl rule %s.", acl.ruleName);
                return -1;
            }
            *acl_range_id = range_id;
            entry_attrs[attrCount].id = SAI_ACL_ENTRY_ATTR_FIELD_RANGE;
            entry_attrs[attrCount].value.aclfield.data.objlist.list = &range_id;
            entry_attrs[attrCount].value.aclfield.data.objlist.count = 1;
            attrCount++;
        }
    }

    rv = sai_acl_api_tbl->create_acl_entry(&entry_id, attrCount, entry_attrs);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Acl: Fail to create acl entry in acl rule %s.",
                acl.ruleName);
        return -1;
    }
    *acl_entry_id = entry_id;
#endif
    return 0;
}

int SaiSetAclToInterface(int length, int* portList, int direction,
        sai_object_id_t acl_table_id)
{
#ifdef SAI_BUILD
    int i = 0;
    sai_status_t rv;
    sai_attribute_t attr;
    sai_attribute_t clear_attr;

    if (length == 0) {
        return 0;
    }

    for (i = 0; i < length; i++) {
        if (direction == AclIn) {
            attr.id = SAI_PORT_ATTR_INGRESS_ACL_LIST;
        } else {
            attr.id = SAI_PORT_ATTR_EGRESS_ACL_LIST;
        }
        attr.value.objlist.list = &acl_table_id;
        attr.value.objlist.count = 1;
        rv = sai_port_api_tbl->set_port_attribute(
                port_list[portList[i]], &attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Acl: Fail to apply ACL table to %d port.",
                    (int)portList[i]);
            return -1;
        }
    }
#endif
    return 0;
}

int SaiPushAclEntryId(char* aclRuleName, AclTableData* aclTableData,
        sai_object_id_t entry_id, sai_object_id_t range_id)
{
#ifdef SAI_BUILD
    int i = 0, full = 1;
    int strLen = 0;

    for(i = 0; i < ACL_TABLE_SIZE; i++) {
        if (aclTableData->entryIdList[i].used == 0) {
            full = 0;
            strLen = strlen(aclRuleName);
            aclTableData->entryIdList[i].aclRuleName =
                (char *)malloc(strLen + 1);
            if (aclTableData->entryIdList[i].aclRuleName == NULL) {
                return -1;
            }
            memcpy(aclTableData->entryIdList[i].aclRuleName, aclRuleName,
                    strLen);
            aclTableData->entryIdList[i].aclRuleName[strLen] = '\0';
            aclTableData->entryIdList[i].acl_entry_id = entry_id;
            aclTableData->entryIdList[i].acl_range_id = range_id;
            aclTableData->entryIdList[i].used = 1;
            break;
        }
    }

    if (full == 1) {
        return -1;
    }
#endif
    return 0;
}

int SaiSearchAclTableId(char* aclName, int direction,
        sai_object_id_t* acl_table_id, AclTableData** aclTableData)
{
#ifdef SAI_BUILD
    int i = 0, find = 1;
    int strLen = 0;

    for (i = 0; i < ACL_MAX_TABLE; i++) {
        if (aclTableIdList[i].used == 1) {
            strLen = strlen(aclName);
            if (strncmp(aclTableIdList[i].aclName, aclName, strLen) != 0
                    || aclTableIdList[i].direction != direction) {
                continue;
            }

            *acl_table_id = aclTableIdList[i].acl_table_id;
            *aclTableData = &aclTableIdList[i];
            find = 0;
            break;
        }
    }

    if (find == 1) {
        return -1;
    }
#endif
    return 0;
}

int SaiPushAclTableId(char* aclName, sai_object_id_t acl_table_id,
        int direction, AclTableData** aclTableData)
{
#ifdef SAI_BUILD
    int i = 0, full = 1;
    int strLen = 0;

    for (i = 0; i < ACL_MAX_TABLE; i++) {
        if (aclTableIdList[i].used == 0) {
            full = 0;
            strLen = strlen(aclName);
            aclTableIdList[i].aclName = (char*)malloc(strLen + 1);
            if (aclTableIdList[i].aclName == NULL) {
                syslog(LOG_CRIT, "Acl: Not enough system memory to use.");
                return -1;
            }
            memcpy(aclTableIdList[i].aclName, aclName, strLen);
            aclTableIdList[i].aclName[strLen] = '\0';
            aclTableIdList[i].acl_table_id = acl_table_id;
            aclTableIdList[i].direction = direction;
            aclTableIdList[i].used = 1;
            *aclTableData = &aclTableIdList[i];
            aclTableCount++;
            break;
        }
    }

    /**
     * Reached the maximum ACL table number
     */
    if (full == 1) {
        return -1;
    }
#endif
    return 0;
}
