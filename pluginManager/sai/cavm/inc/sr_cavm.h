#ifndef SR_CAVM_H
#define SR_CAVM_H

#define CAVM_CNX880091_PORTCOUNT 32

/*
 *  CNX88091 PortMap, front panel port number used as index into array
 *  provides chip local port number
 */
const int cavm_cnx_880091_portmap[CAVM_CNX880091_PORTCOUNT+1] = {
    0, // frontpanel port numbering starts at 1
    5,
    6,
    7,
    8,
    9,
    10,
    11,
    12,
    1,
    2,
    4,
    3,
    13,
    14,
    15,
    16,
    32,
    31,
    29,
    30,
    20,
    19,
    17,
    18,
    28,
    27,
    26,
    25,
    24,
    23,
    22,
    21
};

/*
 *  CNX88091 PortMap, front panel port number used as index into array
 *  provides chip local port number
 */
const int cavm_cnx_880091_reverse_portmap[CAVM_CNX880091_PORTCOUNT+1] = {
    0, // frontpanel port numbering starts at 1
    5,
    6,
    7,
    8,
    9,
    10,
    11,
    12,
    1,
    2,
    4,
    3,
    13,
    14,
    15,
    16,
    23,
    24,
    22,
    21,
    32,
    31,
    30,
    29,
    28,
    27,
    26,
    25,
    19,
    20,
    18,
    17
};

#endif
