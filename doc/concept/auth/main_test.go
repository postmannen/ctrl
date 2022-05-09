package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"testing"
)

// Set the default logging functionality of the package to false.
var logging = flag.Bool("logging", true, "set to true to enable the normal logger of the package")

func TestACLSingleNode(t *testing.T) {
	if !*logging {
		log.SetOutput(io.Discard)
	}

	c := newCentralAuth()
	c.authorization.authSchema.aclCommandAdd("ship101", "admin", "HORSE")
	c.authorization.authSchema.aclCommandAdd("ship101", "admin", "PIG")

	// --- TESTS ---

	mapOfFromNodeCommands := make(map[node]map[command]struct{})
	err := json.Unmarshal(c.authorization.authSchema.schemaGenerated.NodeMap["ship101"].Data, &mapOfFromNodeCommands)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf(" DEBUG : mapOfFromNodeCommands: %v\n", mapOfFromNodeCommands)

	if _, ok := mapOfFromNodeCommands["admin"]["HORSE"]; !ok {
		t.Fatal(" \U0001F631  [FAILED]: missing map entry")
	}

	if _, ok := mapOfFromNodeCommands["admin"]["PIG"]; !ok {
		t.Fatal(" \U0001F631  [FAILED]: missing map entry")
	}
}

func TestACLWithGroups(t *testing.T) {
	if !*logging {
		log.SetOutput(io.Discard)
	}

	c := newCentralAuth()

	const (
		grp_nodes_operators  = "grp_nodes_operators"
		grp_nodes_ships      = "grp_nodes_ships"
		grp_cmds_commandset1 = "grp_cmds_commandset1"
	)

	c.authorization.authSchema.groupNodesAddNode(grp_nodes_operators, "operator1")
	c.authorization.authSchema.groupNodesAddNode(grp_nodes_operators, "operator2")

	c.authorization.authSchema.groupNodesAddNode(grp_nodes_ships, "ship100")
	c.authorization.authSchema.groupNodesAddNode(grp_nodes_ships, "ship101")

	c.authorization.authSchema.groupCommandsAddCommand(grp_cmds_commandset1, "dmesg")
	c.authorization.authSchema.groupCommandsAddCommand(grp_cmds_commandset1, "date")

	c.authorization.authSchema.aclCommandAdd(grp_nodes_ships, "admin", "useradd -m kongen")
	c.authorization.authSchema.aclCommandAdd("ship101", "admin", "HORSE")

	c.authorization.authSchema.aclCommandAdd(grp_nodes_ships, grp_nodes_operators, grp_cmds_commandset1)

	// --- Tests ---

	//if _, ok := c.authorization.authSchema.schemaMain.ACLMap[grp_nodes_ships][grp_nodes_operators][grp_cmds_commandset1]; !ok {
	//	t.Fatal(" \U0001F631  [FAILED]: missing map entry")
	//}

	// Also check the generated data for the nodes.

	// if _, ok := c.authorization.authSchema.schemaMain.ACLMap[grp_nodes_ships]["admin"]["useradd -m kongen"]; !ok {
	// 	t.Fatal(" \U0001F631  [FAILED]: missing map entry")
	// }

	mapOfFromNodeCommands := make(map[node]map[command]struct{})
	err := json.Unmarshal(c.authorization.authSchema.schemaGenerated.NodeMap["ship101"].Data, &mapOfFromNodeCommands)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf(" DEBUG : mapOfFromNodeCommands: %v\n", mapOfFromNodeCommands)

	if _, ok := mapOfFromNodeCommands["admin"]["useradd -m kongen"]; !ok {
		t.Fatal(" \U0001F631  [FAILED]: missing map entry")
	}

	if _, ok := mapOfFromNodeCommands["operator1"]["dmesg"]; !ok {
		t.Fatal(" \U0001F631  [FAILED]: missing map entry")
	}

	if _, ok := mapOfFromNodeCommands["operator1"]["date"]; !ok {
		t.Fatal(" \U0001F631  [FAILED]: missing map entry")
	}

	if _, ok := mapOfFromNodeCommands["operator2"]["dmesg"]; !ok {
		t.Fatal(" \U0001F631  [FAILED]: missing map entry")
	}

	if _, ok := mapOfFromNodeCommands["operator2"]["date"]; !ok {
		t.Fatal(" \U0001F631  [FAILED]: missing map entry")
	}

	if _, ok := mapOfFromNodeCommands["admin"]["HORSE"]; !ok {
		t.Fatal(" \U0001F631  [FAILED]: missing map entry")
	}

}

func TestACLSingleNodeAndNodeGroup(t *testing.T) {
	if !*logging {
		log.SetOutput(io.Discard)
	}

	c := newCentralAuth()

	c.authorization.authSchema.aclCommandAdd("ship101", "admin", "HORSE")

	c.authorization.authSchema.groupNodesAddNode("grp_nodes_ships", "ship101")
	c.authorization.authSchema.aclCommandAdd("grp_nodes_ships", "admin", "HEN")

	// --- TESTS ---

	mapOfFromNodeCommands := make(map[node]map[command]struct{})
	err := json.Unmarshal(c.authorization.authSchema.schemaGenerated.NodeMap["ship101"].Data, &mapOfFromNodeCommands)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf(" DEBUG : mapOfFromNodeCommands: %+v\n", mapOfFromNodeCommands)

	//if _, ok := mapOfFromNodeCommands["admin"]["PIG"]; !ok {
	//	t.Fatalf(" \U0001F631  [FAILED]: missing map entry: PIG: Content of Map: %v", mapOfFromNodeCommands)
	//}

	if _, ok := mapOfFromNodeCommands["admin"]["HORSE"]; !ok {
		t.Fatalf(" \U0001F631  [FAILED]: missing map entry: HORSE: Content of Map: %v", mapOfFromNodeCommands)
	}

	if _, ok := mapOfFromNodeCommands["admin"]["HEN"]; !ok {
		t.Fatalf(" \U0001F631  [FAILED]: missing map entry: HEN: Content of Map: %v", mapOfFromNodeCommands)

	}
}

func TestSchemaMainACLMap(t *testing.T) {
	if !*logging {
		log.SetOutput(io.Discard)
	}

	c := newCentralAuth()

	//c.authorization.authSchema.aclNodeFromnodeCommandAdd("ship101", "admin", "PIG")
	fmt.Printf("---------------ADDING COMMAND-------------\n")
	c.authorization.authSchema.aclCommandAdd("ship0", "admin", "systemctl")
	c.authorization.authSchema.aclCommandAdd("ship1", "admin", "tcpdump")

	if _, ok := c.authorization.authSchema.schemaMain.ACLMap["ship0"]["admin"]["systemctl"]; !ok {
		t.Fatalf(" \U0001F631  [FAILED]: missing map entry: ship0, admin, systemctl")
	}
	if _, ok := c.authorization.authSchema.schemaMain.ACLMap["ship1"]["admin"]["tcpdump"]; !ok {
		t.Fatalf(" \U0001F631  [FAILED]: missing map entry: ship1, admin, tcpdump")
	}

	fmt.Printf("---------------ADDING COMMAND-------------\n")
	c.authorization.authSchema.groupNodesAddNode("grp_nodes_ships", "ship1")
	c.authorization.authSchema.groupNodesAddNode("grp_nodes_ships", "ship2")
	c.authorization.authSchema.aclCommandAdd("grp_nodes_ships", "admin", "dmesg")

	if _, ok := c.authorization.authSchema.schemaMain.ACLMap["grp_nodes_ships"]["admin"]["dmesg"]; !ok {
		t.Fatalf(" \U0001F631  [FAILED]: missing map entry: ship1, admin, tcpdump")
	}

	fmt.Printf("---------------ADDING COMMAND-------------\n")
	c.authorization.authSchema.aclCommandAdd("ship2", "admin", "echo")

	if _, ok := c.authorization.authSchema.schemaMain.ACLMap["ship2"]["admin"]["echo"]; !ok {
		t.Fatalf(" \U0001F631  [FAILED]: missing map entry: ship1, admin, tcpdump")
	}

	fmt.Printf("---------------DELETING COMMAND grp_nodes_ships, admin, dmesg-------------\n")
	c.authorization.authSchema.aclCommandDelete("grp_nodes_ships", "admin", "dmesg")

	if _, ok := c.authorization.authSchema.schemaMain.ACLMap["grp_nodes_ships"]["admin"]["dmesg"]; ok {
		t.Fatalf(" \U0001F631  [FAILED]: found map entry: grp_nodes_ships, admin, dmesg")
	}
	// Check that the remaining are still ok.
	if _, ok := c.authorization.authSchema.schemaMain.ACLMap["ship0"]["admin"]["systemctl"]; !ok {
		t.Fatalf(" \U0001F631  [FAILED]: missing map entry: ship0, admin, systemctl")
	}
	if _, ok := c.authorization.authSchema.schemaMain.ACLMap["ship1"]["admin"]["tcpdump"]; !ok {
		t.Fatalf(" \U0001F631  [FAILED]: missing map entry: ship1, admin, tcpdump")
	}
	if _, ok := c.authorization.authSchema.schemaMain.ACLMap["ship2"]["admin"]["echo"]; !ok {
		t.Fatalf(" \U0001F631  [FAILED]: missing map entry: ship1, admin, tcpdump")
	}

	fmt.Printf("---------------DELETING COMMAND ship0, admin, systemctl-------------\n")
	c.authorization.authSchema.aclCommandDelete("ship0", "admin", "systemctl")

	if _, ok := c.authorization.authSchema.schemaMain.ACLMap["ship0"]["admin"]["systemctl"]; ok {
		t.Fatalf(" \U0001F631  [FAILED]: missing map entry: ship0, admin, systemctl")
	}
	// Check that the remaining are ok.
	if _, ok := c.authorization.authSchema.schemaMain.ACLMap["ship1"]["admin"]["tcpdump"]; !ok {
		t.Fatalf(" \U0001F631  [FAILED]: missing map entry: ship1, admin, tcpdump")
	}
	if _, ok := c.authorization.authSchema.schemaMain.ACLMap["ship2"]["admin"]["echo"]; !ok {
		t.Fatalf(" \U0001F631  [FAILED]: missing map entry: ship1, admin, tcpdump")
	}

	fmt.Printf("---------------DELETING SOURCE ship1, admin-------------\n")
	c.authorization.authSchema.aclSourceDelete("ship1", "admin")

	if _, ok := c.authorization.authSchema.schemaMain.ACLMap["ship1"]["admin"]; ok {
		t.Fatalf(" \U0001F631  [FAILED]: missing map entry: ship1, admin, tcpdump")
	}
	// Check that the remaining are ok.
	if _, ok := c.authorization.authSchema.schemaMain.ACLMap["ship2"]["admin"]["echo"]; !ok {
		t.Fatalf(" \U0001F631  [FAILED]: missing map entry: ship1, admin, tcpdump")
	}
	// --- TESTS ---
}