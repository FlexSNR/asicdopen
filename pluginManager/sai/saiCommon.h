#ifndef SAI_COMMON_H
#define SAI_COMMON_H

typedef struct knetIfInfo_s {
    int valid;
    int port;
    sai_object_id_t ifId;
    char ifName[NETIF_NAME_LEN_MAX];
} knetIfInfo;

#endif
