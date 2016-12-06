/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	//"strings"
	"time"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

// SimpleChaincode example simple Chaincode implementation
type SimpleChaincode struct {
}

// ============================================================================================================================
// Definitions
// ============================================================================================================================

// ----- Marbles ----- //
var marbleIndexStr = "_marbleindex" //name for the key/value that will store a list of all known marbles
type MarblesIndex struct {
	ObjectType string   `json:"docType"` //docType is used to distinguish the various types of objects in state database
	Marbles    []string `json:"marbles"`
}

type Marble struct {
	ObjectType string `json:"docType"` //docType is used to distinguish the various types of objects in state database
	Name       string `json:"name"`    //the fieldtags are needed to keep case from bouncing around
	Color      string `json:"color"`
	Size       int    `json:"size"`
	Owner      string `json:"owner"`
}

// ----- Trades ----- //
var openTradesStr = "_opentrades"      //name for the key/value that will store all open trades
type Description struct {
	ObjectType string `json:"docType"` //docType is used to distinguish the various types of objects in state database
	Color      string `json:"color"`
	Size       int    `json:"size"`
}

type AnOpenTrade struct {
	User      string        `json:"user"`      //user who created the open trade order
	Timestamp int64         `json:"timestamp"` //utc timestamp of creation
	Want      Description   `json:"want"`      //description of desired marble
	Willing   []Description `json:"willing"`   //array of marbles willing to trade away
}

type AllTrades struct {
	ObjectType string        `json:"docType"` //docType is used to distinguish the various types of top level objects in state database
	OpenTrades []AnOpenTrade `json:"open_trades"`
}

// ----- Owners ----- //
var ownerIndexStr = "_ownerindex"       //name for the key/value that will store a list of all known owners
type Owner struct {
	ObjectType string `json:"docType"`  //docType is used to distinguish the various types of objects in state database
	Username   string `json:"username"`
	Company    string `json:"company"`
	Timestamp  int64   `json:"timestamp"` //utc timestamp of registration
}
type OwnersIndex struct {
	ObjectType string   `json:"docType"` //docType is used to distinguish the various types of objects in state database
	Owners    []string  `json:"owners"`
}

// ============================================================================================================================
// Main
// ============================================================================================================================
func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}


// ============================================================================================================================
// Init - reset all the things
// ============================================================================================================================
func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface) ([]byte, error) {
	_, args := stub.GetFunctionAndParameters()
	var Aval int
	var err error

	if len(args) != 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting 1")
	}

	// Initialize the chaincode
	Aval, err = strconv.Atoi(args[0])
	if err != nil {
		return nil, errors.New("Expecting integer value for asset holding")
	}

	// Write the state to the ledger
	err = stub.PutState("abc", []byte(strconv.Itoa(Aval))) //making a test var "abc", I find it handy to read/write to it right away to test the network
	if err != nil {
		return nil, err
	}

	var marbles MarblesIndex
	marbles.ObjectType = "MarbleIndex"
	jsonAsBytes, _ := json.Marshal(marbles) //marshal a marbles index struct with emtpy array of strings to clear the index
	err = stub.PutState(marbleIndexStr, jsonAsBytes)
	if err != nil {
		return nil, err
	}

	var trades AllTrades
	trades.ObjectType = "Trades"
	jsonAsBytes, _ = json.Marshal(trades) 		//trades is empty, this clear the open trade index
	err = stub.PutState(openTradesStr, jsonAsBytes)
	if err != nil {
		return nil, err
	}

	var owner OwnersIndex
	owner.ObjectType = "OwnerIndex"
	jsonAsBytes, _ = json.Marshal(owner)		//owner is empty, this clears the owner index
	err = stub.PutState(ownerIndexStr, jsonAsBytes)
	if err != nil {
		return nil, err
	}

	return nil, nil
}


// ============================================================================================================================
// Invoke - Our entry point for Invocations
// ============================================================================================================================
func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface) ([]byte, error) {
	function, args := stub.GetFunctionAndParameters()
	fmt.Println("starting invoke, for: " + function)

	// Handle different functions
	if function == "init" { 				//initialize the chaincode state, used as reset
		return t.Init(stub)
	} else if function == "delete_marble" { //deletes a marble from state
		res, err := delete_marble(stub, args)
		cleanTrades(stub) 					//lets make sure all open trades are still valid
		return res, err
	} else if function == "write" { 		//writes a value to the chaincode state
		return write(stub, args)
	} else if function == "init_marble" { 	//create a new marble
		return init_marble(stub, args)
	} else if function == "set_owner" { 	//change owner of a marble
		res, err := set_owner(stub, args)
		cleanTrades(stub) 					//lets make sure all open trades are still valid
		return res, err
	} else if function == "open_trade" { 	//create a new trade order
		return open_trade(stub, args)
	} else if function == "perform_trade" { //forfill an open trade order
		res, err := perform_trade(stub, args)
		cleanTrades(stub) 					//lets clean just in case
		return res, err
	} else if function == "remove_trade" { 	//cancel an open trade order
		return remove_trade(stub, args)
	} else if function == "read" {
		return read(stub, args)
	}else if function == "init_owner"{
		return init_owner(stub, args)
	}

	fmt.Println("Received unknown invoke function name: " + function) //should not get here, its an error
	return nil, errors.New("Received unknown invoke function name: '" + function + "'")
}

// ============================================================================================================================
// Read - read a variable from chaincode state
// ============================================================================================================================
func read(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var name, jsonResp string
	var err error
	fmt.Println("starting read")

	if len(args) != 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting name of the var to query")
	}

	name = args[0]
	valAsbytes, err := stub.GetState(name) //get the var from chaincode state
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get state for " + name + "\"}"
		return nil, errors.New(jsonResp)
	}

	fmt.Println("- end read")
	return valAsbytes, nil //send it onward
}

// ============================================================================================================================
// Write - write variable into chaincode state
// ============================================================================================================================
func write(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var name, value string // Entities
	var err error
	fmt.Println("starting write")

	if len(args) != 2 {
		return nil, errors.New("Incorrect number of arguments. Expecting 2. name of the variable and value to set")
	}

	name = args[0] //rename for funsies
	value = args[1]
	err = stub.PutState(name, []byte(value)) //write the variable into the chaincode state
	if err != nil {
		return nil, err
	}

	fmt.Println("- end write")
	return nil, nil
}

// ============================================================================================================================
// Make Timestamp - create a timestamp in ms
// ============================================================================================================================
func makeTimestamp() int64 {
	return time.Now().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}
