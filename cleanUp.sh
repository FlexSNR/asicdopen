#!/bin/bash
#CleanUp links MLNX
cd pluginManager/sai/mlnx/sn2700/lib/;
find . -type l -exec rm -f {} \;
cd -
#Cleanup Hash library
if [ -L ./pluginManager/pluginCommon/utils/libhash.so ]; then
    cd pluginManager/pluginCommon/utils/;
    rm libhash.so 2>/dev/null
    cd -
fi
echo "Cleanup completed !"
