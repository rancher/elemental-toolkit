package main

import (
	"log"
	"os"
	"os/exec"
	"strings"
)

var (
	bootSequence = []string{
		"/bbin/date",
		"/bbin/dhclient -ipv6=false",
		"/bbin/ip a",
		"/loader",
		"/bbin/elvish",
		"/bbin/shutdown halt",
	}
)


func runCmd(s string) {
	log.Printf("Executing Command: %v", s)

	cmdSplit := strings.Split(s, " ")
	cmd := exec.Command(cmdSplit[0], cmdSplit[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		log.Print(err)
	}
}

func main() {
	for _, s := range bootSequence {
		runCmd(s)
	}
}