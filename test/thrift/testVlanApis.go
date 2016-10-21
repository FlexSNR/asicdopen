package testThrift

import (
	"asicdServices"
	"fmt"
)

func TestVlanApis(asicdclnt *asicdServices.ASICDServicesClient) bool {
	//Create : vlan 2, port 1 as untagged
	var vlanObj = new(asicdServices.Vlan)
	vlanObj.VlanId = int32(2)
	vlanObj.IntfList = []string{""}
	vlanObj.UntagIntfList = []string{"1"}
	_, err := asicdclnt.CreateVlan(vlanObj)
	if err != nil {
		fmt.Println("Failed to create vlan. Aborting test suite.", err)
		return false
	} else {
		fmt.Println("Successfully created vlan 2")
	}
	//Verify vlan was created
	var vlanStateObj = new(asicdServices.VlanState)
	vlanStateObj, err = asicdclnt.GetVlanState(vlanObj.VlanId)
	if err != nil {
		fmt.Println("Failed to get vlan state for vlan - ", vlanObj.VlanId)
		return false
	} else {
		fmt.Println("Retrieved vlan 2 state - ", vlanStateObj)
	}
	//Update vlan object
	var oldVlanObj = new(asicdServices.Vlan)
	*oldVlanObj = *vlanObj
	vlanObj.VlanId = int32(2)
	vlanObj.IntfList = []string{""}
	vlanObj.UntagIntfList = []string{"1", "2", "3"}
	_, err = asicdclnt.UpdateVlan(oldVlanObj, vlanObj, []bool{false, false, true}, nil)
	if err != nil {
		fmt.Println("Failed to update vlan. Aborting test suite.", err)
		return false
	} else {
		fmt.Println("Successfully updated vlan 2")
	}
	//Delete vlan
	_, err = asicdclnt.DeleteVlan(vlanObj)
	if err != nil {
		fmt.Println("Failed to delete vlan. Aborting test suite.", err)
		return false
	} else {
		fmt.Println("Successfully deleted vlan 2")
	}
	return true
}
