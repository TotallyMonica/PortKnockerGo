package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var help = flag.Bool("help", false, "Show help")
var masterPort int
var inter string
var protocol string
var timeout int
var knownGood string
var verbose bool
var passCompile = flag.Bool("pass-compilation", false, "Forces script to pass compilation")

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

func contains(ports []int, port int) bool {
	for _, val := range ports {
		if val == port {
			return true
		}
	}
	return false
}

func runningAsAdmin() bool {
	if runtime.GOOS == "windows" {
		_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
		return err == nil
	} else {
		return os.Getuid() == 0
	}
}

func testTCP(c chan int, ready chan bool, port int, inter string, timeout int, verbose bool) {
	data := "The cat is out of the bag\n"
	networkInterface := inter + ":" + strconv.Itoa(port)

	server, err := net.Listen("tcp", networkInterface)
	if err != nil {
		ready <- false
		if !strings.Contains(err.Error(), "address already in use") {
			log.Fatalf("74: Error while trying to listen on %s\n%s\n", networkInterface, err)
		} else {
			c <- -2
			return
		}
	}
	fmt.Println("Listening")
	ready <- true

	srvConn, err := server.Accept()
	if err != nil {
		log.Fatal("79: Error while accepting connection\n", err)
	}
	fmt.Println("Listening on " + networkInterface)

	deadline := time.Duration(timeout) * time.Second
	err = srvConn.SetDeadline(time.Now().Add(deadline))
	if err != nil {
		log.Fatal("86: Error while setting timeout\n", err)
	}

	defer func(server net.Listener) {
		err := server.Close()
		if err != nil {
			log.Fatalf("92: Error while closing connection %s\n%s\n", networkInterface, err)
		}
	}(server)

	_, err = srvConn.Write([]byte(data))
	if err != nil {
		log.Fatalf("98: Error while sending data to %s\n%s\n", networkInterface, err)
	}

	received, err := bufio.NewReader(srvConn).ReadString('\n')

	if err != nil {
		log.Fatal("104: Timeout period exceeded\n", err)
	}
	if data == received {
		c <- 0
	} else {
		c <- 1
	}

	deadline = time.Duration(1) * time.Millisecond
	err = srvConn.SetDeadline(time.Now().Add(deadline))
	if err != nil {
		log.Fatal("127: Error while setting timeout\n", err)
	}
}

// Communication script
// TODO: Build the framework
func communicate(master int, proto string, verbose bool, inter string) masterResults {
	fullConnection := inter + ":" + strconv.Itoa(master)
	masterConnection, err := net.Listen(proto, fullConnection)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}

	defer func(masterConnection net.Listener) {
		err := masterConnection.Close()
		if err != nil {
			log.Fatalf("149: Error while closing connection %s\n%s\n", fullConnection, err)
		}
	}(masterConnection)

	// Begin listening
	if verbose {
		fmt.Println("Master server listening on " + fullConnection)
		fmt.Println("Waiting for a client...")
	}
	masterClient, err := masterConnection.Accept()
	if err != nil {
		log.Fatal("160: Error accepting: ", err)
	}

	//err = masterClient.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
	//if err != nil {
	//	log.Fatalf("143: Error while setting timeout\n%s\n", err)
	//}

	if verbose {
		fmt.Printf("Client connected on %v\n", masterClient.RemoteAddr())
	}

	// We've accepted a client, begin getting test parameters
	data, err := bufio.NewReader(masterClient).ReadString('\n')

	if err != nil {
		log.Fatalf("176: Error while reading data from client%s\n%s\n", masterClient.RemoteAddr(), err)
	}

	var serverParameters serverInfo
	dataBytes := []byte(data)
	err = json.Unmarshal(dataBytes, &serverParameters)

	if err != nil {
		log.Fatalf("184: Error while parsing data from client%s\n%s\n", masterClient.RemoteAddr(), err)
	}

	testedPort := serverParameters.StartPort
	if runningAsAdmin() && testedPort < 1024 {
		panic("Server is not running as root but client requested root only ports")
	}

	var masterTestResults masterResults
	masterTestResults.Results = make([]testResults, 0)

	// Test each port
	for testedPort <= serverParameters.EndPort {
		if contains(serverParameters.KnownGood, testedPort) || testedPort == master {
			testedPort += 1
			continue
		}

		// Build out the test data for that port
		testParams := testPort{
			ContinueTesting: true,
			TestedPort:      testedPort,
			Timeout:         serverParameters.Timeout,
			Protocol:        serverParameters.Protocol,
		}

		// Send test params to client
		formattedTestParams, err := json.Marshal(&testParams)
		if err != nil {
			log.Fatalf("192: Error while sending test paramters to client%s\n%s\n", masterClient.RemoteAddr(), err)
			continue
		} else {
			fmt.Printf("%s", string(formattedTestParams)+"\n")
		}

		result := -16
		c := make(chan int)
		ready := make(chan bool)

		if proto == "tcp" {
			go testTCP(c, ready, testedPort, inter, testParams.Timeout, verbose)
			status := <-ready
			if status {
				fmt.Printf("Ready for client\n")
				_, err := masterClient.Write([]byte(string(formattedTestParams) + "\n"))
				if err != nil {
					log.Fatal("209: Error while writing ready message to client\n", err)
				}
			}
			result = <-c
		}

		// Format results
		var testResult testResults
		testResult.Protocol = proto
		testResult.Port = testedPort
		testResult.Result = result == 0

		switch result = result; result {
		case -2:
			testResult.Reason = "In Use"
		case -1:
			testResult.Reason = "Timed out"
		case 0:
			testResult.Reason = "Success"
		case 1:
			testResult.Reason = "Malformed"
		}
		//time.Sleep(100 * time.Millisecond)
		//formatted, err := json.Marshal(testResult)
		//if err != nil {
		//	log.Fatalf("233: Error while encoding data for client%s\n%s\n", masterClient.RemoteAddr(), err)
		//} else {
		//	_, err = masterClient.Write([]byte(string(formatted) + "\n"))
		//	if err != nil {
		//		log.Fatal("237: Error while sending results to client")
		//	}
		//	results, err := json.Marshal(formatted)
		//	if err == nil {
		//		fmt.Printf("%s", string(results)+"\n")
		//	}
		//}
		masterTestResults.Results = append(masterTestResults.Results, testResult)
		testedPort += 1
	}

	fmt.Println("Finished all ports")
	// Tell the client to stop testing
	testParams, err := json.Marshal(testPort{
		ContinueTesting: false,
	})
	if err != nil {
		log.Fatalf("265: Error while building test params to send to client\n%s\n", err)
	}
	_, err = masterClient.Write([]byte(string(testParams) + "\n"))
	if err != nil {
		log.Fatalf("270: Error while building test params to send to client\n%s\n", err)
	}

	masterTestResults.Count = len(masterTestResults.Results)
	marshalled, err := json.Marshal(&masterTestResults)
	if err == nil && string(marshalled) == "{}" {
		fmt.Printf("Fyi, marshalled didn't go {} for once\n")
	}

	// Send everything to client
	fmt.Println("Formatting results")
	results := ""
	for i, result := range masterTestResults.Results {
		results = results + fmt.Sprintf("%d,%s", result.Port, result.Reason)
		if i != masterTestResults.Count-1 {
			results = results + "|"
		}
	}

	byteResults := []byte(results + "\n")
	fmt.Println("Sending to client")
	_, err = masterClient.Write(byteResults)
	if err != nil {
		log.Fatalf("280: Error while sending test results to client%s\n%s\n", masterClient.RemoteAddr(), err)
	}
	return masterTestResults
}

func main() {
	flag.IntVar(&masterPort, "master-port", 4312, "Master port to use")
	flag.StringVar(&inter, "interface", "0.0.0.0", "Interface to listen on")
	flag.StringVar(&protocol, "protocol", "tcp", "Protocol to use")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	communicate(masterPort, protocol, verbose, inter)
}
