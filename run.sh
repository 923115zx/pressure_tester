#
# File            : run.sh
# Author          : ZhaoXin
# CreateTime      : 2018-08-08 17:52:33
# LastModified    : 2018-08-15 11:39:18
# Vim             : ts=4, sw=4
#

#!/usr/bin/env bash

#./build.sh
go build -o pressure_tester

./pressure_tester -dest_addr=10.109.0.91:3001 -path_list=retail/tracker/:version/add_human,retail/tracker/:version/get_track
