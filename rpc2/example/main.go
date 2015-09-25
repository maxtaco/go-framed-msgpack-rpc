package main

import (
	"flag"
	"log"
	"os"
)

var server bool
var port int

func init() {
	flag.BoolVar(&server, "s", false, "server (client by default)")
	flag.IntVar(&port, "p", 8022, "specify a port (8022 by default)")
}

func main() {
	flag.Parse()
	var err error
	if server {
		err = (&Server{port: port}).Run(make(chan struct{}))
	} else {
		err = (&Client{port: port}).Run()
	}
	if err != nil {
		log.Printf("Error: %s", err.Error())
		os.Exit(-2)
	} else {
		os.Exit(0)
	}
}
