package testThrift

import (
	"asicdInt"
	"asicdServices"
	"fmt"
)

func TestLagApis(asicdclnt *asicdServices.ASICDServicesClient) bool {
	//Create LAG
	_, err := asicdclnt.CreateLag("LAG-A", int32(0), "2,3")
	if err != nil {
		fmt.Println("Failed to create lag LAG-A")
		return false
	} else {
		fmt.Println("Successfully created LAG-A")
	}
	//GetBulk LAG
	info, err := asicdclnt.GetBulkLag(asicdInt.Int(0), asicdInt.Int(10))
	if err != nil {
		fmt.Println("Failed to get bulk lag")
		return false
	} else {
		fmt.Println("GetBulk Lag returned - ", info)
	}
	//Delete LAG
	_, err = asicdclnt.DeleteLag(info.LagList[0].LagIfIndex)
	if err != nil {
		fmt.Println("Failed to create lag LAG-A")
		return false
	} else {
		fmt.Println("Successfully created LAG-A")
	}
	return true
}
