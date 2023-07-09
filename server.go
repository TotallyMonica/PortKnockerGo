package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

var help = flag.Bool("help", false, "Show help")
var masterPort int
var inter string
var protocol string
var timeout int
var knownGood string
var verbose bool
var passCompile = flag.Bool("pass-compilation", false, "Forces script to pass compilation")

// Communication script
// TODO: Build the framework
func communicate(startPort int, endPort int, address string, master int, proto string, timeout int, verbose bool, knownGood []int) {
	fullConnection := inter + ":" + strconv.Itoa(master)
	masterConnection, err := net.Listen(proto, fullConnection)

	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}

	defer masterConnection.Close()

	fmt.Println("Listening on " + fullConnection)
	fmt.Println("Waiting for a client...")
	for {
		masterClient, err := masterConnection.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		fmt.Printf("Client connected on %v\n", masterClient.RemoteAddr())

		for {
			data, err := bufio.NewReader(masterClient).ReadString('\n')

			if err != nil {
				panic(err)
				return
			}

			// TODO: begin parsing data
		}
	}
}

func main() {
	flag.IntVar(&masterPort, "master-port", 4312, "Master port to use")
	flag.StringVar(&inter, "interface", "0.0.0.0", "Interface to listen on")
	flag.StringVar(&protocol, "protocol", "tcp", "Protocol to use")
	flag.IntVar(&timeout, "timeout", 10, "Timeout period (Default: 10 seconds)")
	flag.StringVar(&knownGood, "known-good", "", "Known good ports to use, comma delimited")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	//fmt.Printf("%v\n%v\n%v\n%v\n%v\n", masterPort, inter, protocol, timeout, knownGood)

	var startPort = 1024

	// Define the parameters
	if os.Getuid() == 0 {
		startPort = 0
	}
	var endPort = 65536

	// Check to see if known good ports were provided.
	var ignorePorts []int

	if len(knownGood) > 0 && strings.Contains(knownGood, ",") {
		knownGoodList := strings.Split(knownGood, ",")
		for i := 0; i < len(knownGoodList); i++ {
			//fmt.Println(knownGoodPorts[i])
			numeric, err := strconv.Atoi(knownGoodList[i])

			// An error was given
			if err != nil {
				panic(err)
			}

			if numeric >= startPort && numeric <= endPort {
				ignorePorts = append(ignorePorts, numeric)
			}
		}
		// Check if a singular known port was given
	} else if len(knownGood) > 0 {
		numeric, err := strconv.Atoi(knownGood)

		// Ensure it is an integer, for now just throw an error on invalid argument.
		// TODO: Give a proper warning and continue as is.
		if err != nil {
			panic(err)
		}

		if numeric >= startPort && numeric <= endPort {
			ignorePorts = append(ignorePorts, numeric)
		}
	}

	communicate(startPort, endPort, inter, masterPort, protocol, timeout, verbose, ignorePorts)
}
