//
//Copyright [2016] [SnapRoute Inc]
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//	 Unless required by applicable law or agreed to in writing, software
//	 distributed under the License is distributed on an "AS IS" BASIS,
//	 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	 See the License for the specific language governing permissions and
//	 limitations under the License.
//
// _______  __       __________   ___      _______.____    __    ____  __  .___________.  ______  __    __
// |   ____||  |     |   ____\  \ /  /     /       |\   \  /  \  /   / |  | |           | /      ||  |  |  |
// |  |__   |  |     |  |__   \  V  /     |   (----` \   \/    \/   /  |  | `---|  |----`|  ,----'|  |__|  |
// |   __|  |  |     |   __|   >   <       \   \      \            /   |  |     |  |     |  |     |   __   |
// |  |     |  `----.|  |____ /  .  \  .----)   |      \    /\    /    |  |     |  |     |  `----.|  |  |  |
// |__|     |_______||_______/__/ \__\ |_______/        \__/  \__/     |__|     |__|      \______||__|  |__|
//

package pluginManager

import (
	"errors"
	"strconv"
	"strings"
)

//Note: Caller validates that str is a valid port range string
func parseRangeAndAppendToList(str string) ([]string, error) {
	var strList []string
	subStr := strings.Split(str, "-")
	subStr[0] = strings.Trim(subStr[0], " ")
	start, err := strconv.Atoi(subStr[0])
	if err != nil {
		return nil, errors.New("Range specifications are only supported with ifIndex parameters")
	}
	subStr[1] = strings.Trim(subStr[1], " ")
	end, err := strconv.Atoi(subStr[1])
	if err != nil {
		return nil, errors.New("Range specifications are only supported with ifIndex parameters")
	}
	for idx := start; idx <= end; idx++ {
		strList = append(strList, strconv.Itoa(idx))
	}
	return strList, nil
}

/*
 * Utility function to parse from a user specified string to a list of strings mapping to ifname or string ifindex.
 * Supported formats for user string shown below:
 * - 1,2,3,10 (comma separated list)
 * - 1-10,15-18 (hyphen separated ranges)
 * - 1,2,6-9 (combination of comma and hyphen separated strings)
 */
func ParseUsrStrToList(usrStr string) ([]string, error) {
	var subStrList []string
	if len(usrStr) == 0 {
		return nil, nil
	}
	//Handle ',' separated strings
	if strings.Contains(usrStr, ",") {
		commaSepList := strings.Split(usrStr, ",")
		for _, subStr := range commaSepList {
			//Substr contains '-' separated range
			if strings.Contains(subStr, "-") {
				list, err := parseRangeAndAppendToList(subStr)
				if err != nil {
					return nil, err
				}
				subStrList = append(subStrList, list...)
			} else {
				subStr = strings.Trim(subStr, " ")
				subStrList = append(subStrList, subStr)
			}
		}
	} else if strings.Contains(usrStr, "-") {
		//Handle '-' separated range
		list, err := parseRangeAndAppendToList(usrStr)
		if err != nil {
			return nil, err
		}
		subStrList = append(subStrList, list...)
	} else {
		usrStr = strings.Trim(usrStr, " ")
		subStrList = append(subStrList, usrStr)
	}
	return subStrList, nil
}

func arePortListsMutuallyExclusive(list1, list2 []int32) bool {
	var list1Map map[int32]bool = make(map[int32]bool, len(list1))

	for idx := 0; idx < len(list1); idx++ {
		list1Map[list1[idx]] = true
	}
	for idx := 0; idx < len(list2); idx++ {
		if _, ok := list1Map[list2[idx]]; ok {
			return false
		}
	}
	return true
}
