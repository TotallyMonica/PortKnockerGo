package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var masterPort int
var address string
var protocol string
var knownGood string
var verbose bool
var unstable bool
var timeout int

type testPort struct {
	Protocol        string `json:"protocol"`
	Timeout         int    `json:"timeout"`
	TestedPort      int    `json:"tested_port"`
	ContinueTesting bool   `json:"continue_testing"`
}

type masterResults struct {
	Count   int           `json:"count"`
	Results []testResults `json:"results"`
}

type testResults struct {
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	Result   bool   `json:"results"`
	Reason   string `json:"reason"`
}

type serverInfo struct {
	Protocol  string `json:"protocol"`
	Timeout   int    `json:"timeout"`
	KnownGood []int  `json:"known_good"`
	StartPort int    `json:"start_port"`
	EndPort   int    `json:"end_port"`
}

func testTCP(ready chan bool, port int, addr string, timeout int, verbose bool) {
	networkInterface := addr + ":" + strconv.Itoa(port)

	//if !unstable {
	//	fmt.Println("Sleeping 5 seconds")
	//	time.Sleep(5 * time.Second)
	//}

	fmt.Printf("Testing port %d\n", port)
	server, err := net.Dial("tcp", networkInterface)
	if err != nil {
		log.Fatalf("58: Error while dialing server %s\n%s\n", networkInterface, err)
	}
	fmt.Println("Dialing")
	deadline := time.Duration(timeout) * time.Second
	fmt.Println("Set duration")
	err = server.SetDeadline(time.Now().Add(deadline))
	if err != nil {
		log.Fatal("70: Couldn't set timeout")
	}
	fmt.Println("Set Timeout")

	defer func(server net.Conn) {
		err := server.Close()
		if err != nil {
			log.Fatalf("77: Error while closing connection to %s\n%s\n", networkInterface, err)
		}
	}(server)

	received, err := bufio.NewReader(server).ReadString('\n')
	if err != nil {
		log.Fatalf("83: Error while reading data from server %s\n%s\n", networkInterface, err)
	}
	if verbose {
		fmt.Println("Got data")
	}
	_, err = server.Write([]byte(received))
	if err != nil {
		log.Fatalf("90: Error while sending data to server %s\n%s\n", networkInterface, err)
	}
	if verbose {
		fmt.Println("Sent data")
	}
	deadline = 1 * time.Millisecond
	fmt.Println("Set duration")
	err = server.SetDeadline(time.Now().Add(deadline))
	if err != nil {
		log.Fatal("99: Couldn't set timeout")
	}
	fmt.Println("Set Timeout")
	return
}

func communicate(address string, masterPort int, protocol string, knownGood []int, timeout int, verbose bool) []string {
	masterAddr := fmt.Sprintf("%s:%d", address, masterPort)

	//deadline := time.Duration(timeout) * time.Second
	masterServer, err := net.Dial(protocol, masterAddr)
	if err != nil {
		log.Fatalf("111: Error while connecting to server %s\n%s\n", masterAddr, err)
	}
	//err = masterServer.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
	//if err != nil {
	//	log.Fatalf("99: Error while setting timeout\n%s\n", err)
	//}

	// Close the listener if the timeout period gets exceeded
	//timer := time.NewTimer(deadline)
	//go func(listener net.Conn) {
	//	<-timer.C
	//	fmt.Println("Listener timeout reached")
	//	listener.Close()
	//}(masterServer)
	defer func(masterServer net.Conn) {
		err := masterServer.Close()
		if err != nil {
			log.Fatalf("128: Error while closing connection to server %s\n%s\n", masterAddr, err)
		}
	}(masterServer)

	if verbose {
		fmt.Println("Connecting to " + masterAddr)
	}

	serverParameters := serverInfo{
		Protocol:  protocol,
		Timeout:   timeout,
		KnownGood: knownGood,
		StartPort: 1024,
		EndPort:   65535,
	}

	data, err := json.Marshal(serverParameters)
	if err != nil {
		log.Fatalf("130: Error while sending test parameters to server: %s\n", err)
	}

	_, err = masterServer.Write([]byte(string(data) + "\n"))
	if err != nil {
		log.Fatalf("135: Error while sending data to server %s\n%s\n", masterAddr, err)
	}

	//masterTestResults := make([]testResults, 0)
	continueTesting := true

	for continueTesting {
		var testedPort testPort
		params, err := bufio.NewReader(masterServer).ReadString('\n')
		if err != nil {
			log.Fatalf("145: Error while receiving data from server %s\n%s\n", masterAddr, err)
		}

		err = json.Unmarshal([]byte(params), &testedPort)

		if err != nil {
			log.Fatalf("153: Error while parsing data sent from %s\n%s\n", masterAddr, err)
		}

		// Check if the server wants us to stop testing
		if !testedPort.ContinueTesting {
			continueTesting = testedPort.ContinueTesting
			continue
		}

		c := make(chan bool)
		// Run test on that port
		if protocol == "tcp" {
			testTCP(c, testedPort.TestedPort, address, timeout, verbose)
		}
	}
	fmt.Println("Finished all ports")
	fmt.Println("Receiving results from server")
	overallTestResults, err := bufio.NewReader(masterServer).ReadString('\n')
	if err != nil {
		log.Fatalf("176: Error while receiving data from %s\n%s\n", masterAddr, err)
	}

	results := strings.Split(overallTestResults, "|")
	return results
}

func main() {
	flag.IntVar(&masterPort, "master-port", 4312, "Master port to use")
	flag.StringVar(&address, "address", "", "Interface to listen on")
	flag.StringVar(&protocol, "protocol", "tcp", "Protocol to use")
	flag.StringVar(&knownGood, "known-good", "", "Known good ports, comma delimited")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&unstable, "unstable", false, "Skip the delay. Setting this may cause race conditions.")
	flag.IntVar(&timeout, "timeout", 10, "Set timeout period")

	flag.Parse()

	ignoredPorts := make([]int, len(strings.Split(knownGood, ",")))
	for _, port := range strings.Split(knownGood, ",") {
		parsedPort, err := strconv.Atoi(port)
		if err != nil {
			log.Fatalf("197: Error while parsing port %s\n%s\n", port, err)
		}
		ignoredPorts = append(ignoredPorts, parsedPort)
	}

	results := communicate(address, masterPort, protocol, ignoredPorts, timeout, verbose)
	filenameFriendlyAddr := strings.ReplaceAll(address, ":", "_")
	filenameFriendlyAddr = strings.ReplaceAll(filenameFriendlyAddr, "&", "_")
	filenameFriendlyAddr = strings.ReplaceAll(filenameFriendlyAddr, ".", "_")
	filenameFriendlyAddr = strings.ReplaceAll(filenameFriendlyAddr, "[", "")
	filenameFriendlyAddr = strings.ReplaceAll(filenameFriendlyAddr, "]", "")
	filename := fmt.Sprintf("results_%s.csv", filenameFriendlyAddr)
	file, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Error while creating file %s\n%s\n", filename, err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatalf("Error while closing file %s\n%s\n", filename, err)
		}
	}(file)
	csvWriter := csv.NewWriter(file)
	err = csvWriter.Write([]string{"port", "result"})
	if err != nil {
		fmt.Printf("195: Error while writing headers to %s\n%s\n", filename, err)
	}
	for i, row := range results {
		err = csvWriter.Write(strings.Split(strings.TrimSpace(row), ","))
		if err != nil {
			fmt.Printf("227: Error while writing row	 to %s\n%s\n", filename, err)
		}
		if i%1000 == 0 {
			csvWriter.Flush()
		}
		if i == len(results)-1 {
			fmt.Println("Finishing...")
		}
	}
	csvWriter.Flush()
}
