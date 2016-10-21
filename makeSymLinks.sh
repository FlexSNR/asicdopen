#!/bin/bash
if [ "$1" == "mlnx" ]; then
    cd pluginManager/sai/mlnx/sn2700/lib
    ln -sf libsai.so.1.0.0 libsai.so.1
    ln -sf libsai.so.1.0.0 libsai.so
    ln -sf libsw_rm_int.so.1.0.0 libsw_rm_int.so.1
    ln -sf libsw_rm_int.so.1.0.0 libsw_rm_int.so
    ln -sf libsw_rm.so.1.0.0 libsw_rm.so.1
    ln -sf libsw_rm.so.1.0.0 libsw_rm.so
    ln -sf libsxapi.so.1.0.0 libsxapi.so.1
    ln -sf libsxapi.so.1.0.0 libsxapi.so
    ln -sf libsxcomp.so.1.0.0 libsxcomp.so.1
    ln -sf libsxcomp.so.1.0.0 libsxcomp.so
    ln -sf libsxcom.so.1.0.0 libsxcom.so.1
    ln -sf libsxcom.so.1.0.0 libsxcom.so
    ln -sf libsxdemadparser.so.1.0.0 libsxdemadparser.so.1
    ln -sf libsxdemadparser.so.1.0.0 libsxdemadparser.so
    ln -sf libsxdemad.so.1.0.0 libsxdemad.so.1
    ln -sf libsxdemad.so.1.0.0 libsxdemad.so
    ln -sf libsxdev.so.1.0.0 libsxdev.so.1
    ln -sf libsxdev.so.1.0.0 libsxdev.so
    ln -sf libsxdreg_access.so.1.0.0 libsxdreg_access.so.1
    ln -sf libsxdreg_access.so.1.0.0 libsxdreg_access.so
    ln -sf libsxgenutils.so.1.0.0 libsxgenutils.so.1
    ln -sf libsxgenutils.so.1.0.0 libsxgenutils.so
    ln -sf libsxlog.so.1.0.0 libsxlog.so.1
    ln -sf libsxlog.so.1.0.0 libsxlog.so
    ln -sf libsxnet.so.1.0.0 libsxnet.so.1
    ln -sf libsxnet.so.1.0.0 libsxnet.so
    ln -sf libsxutils.so.1.0.0 libsxutils.so.1
    ln -sf libsxutils.so.1.0.0 libsxutils.so
    cd -
fi

#this is for utils library linking
cd pluginManager/pluginCommon/utils/
    ln -sf libhash.so.1 libhash.so
cd -
