#ifndef SR_MLNX_H
#define SR_MLNX_H

#define MLNX_SN2700_PORTCOUNT 32
/* SN2700 Portmap, front panel port number used as index into
 * array provides chip local port number */
const int mlnx_sn2700_portmap[MLNX_SN2700_PORTCOUNT+1] = {
    0, //frontpanel port numbering starts at 1
    31,
    32,
    29,
    30,
    27,
    28,
    25,
    26,
    23,
    24,
    21,
    22,
    19,
    20,
    17,
    18,
    1,
    2,
    3,
    4,
    5,
    6,
    7,
    8,
    9,
    10,
    11,
    12,
    13,
    14,
    15,
    16
};
/* SN2700 Portmap, front panel port number used as index into
 * array provides chip local port number */
const int mlnx_sn2700_reverse_portmap[MLNX_SN2700_PORTCOUNT+1] = {
    0,
    17,
    18,
    19,
    20,
    21,
    22,
    23,
    24,
    25,
    26,
    27,
    28,
    29,
    30,
    31,
    32,
    15,
    16,
    13,
    14,
    11,
    12,
    9,
    10,
    7,
    8,
    5,
    6,
    3,
    4,
    1,
    2
};
#endif //SR_MLNX_H
