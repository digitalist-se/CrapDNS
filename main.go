package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/miekg/dns"
)

var (
	// inConfig is a flag to pass a configuration file with a list of domains.
	inConfig = flag.String("configfile", "crapdns.conf", "Use configuration file ( default: crapdns.conf)")

	// inDomains is a flag for passing domains on the commanf-line
	inDomains = flag.String("domains", "", "A comma-separated list of domains to answer for. (Disables config file).")

	// parsedDomains stores an array of domain strings.
	parsedDomains []string
)

const (
	// targetDir is the path to the MacOS resolver directory.
	targetDir = "/etc/resolver/"

	// fileSig is added toe resolver files we generate
	// so that we only delete our own files.
	fileSig = "###CRAPDNS###"
)

// panicExit is for passing an exit code up through panic()
// instead of calling os.Exit() directly. This means we can
// use a deferred fumction to cleanup everything.
type panicExit struct{ Code int }

func main() {
	defer handleExit()
	defer cleanupDomains()

	if runtime.GOOS != "darwin" {
		fmt.Println("This utility is for Mac OS only.")
		panic(panicExit{2})
	}

	flag.Usage = func() {
		flag.PrintDefaults()
	}

	flag.Parse()

	parsedDomains = setupDomains()

	// server listens only on loopback port 53 UDP
	server := &dns.Server{Addr: "127.0.0.1:53", Net: "udp"}

	// Run our server in a goroutine.
	go func() {

		if err := server.ListenAndServe(); err != nil {
			fmt.Printf("Failed to setup the server: %s\n", err.Error())
			fmt.Println("This command should be run as sudo.")
			os.Exit(1)
		}

	}()

	fmt.Println("\nStarting CrapDNS. Listening on 127.0.0.1:53")
	dns.HandleFunc(".", handleRequest)

	// Wait for the apocalypse
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	fmt.Printf("\nSignal (%s) received, exiting\n", s)
}

func handleExit() {
	if e := recover(); e != nil {
		if exit, ok := e.(panicExit); ok == true {
			os.Exit(exit.Code)
		}
		panic(e) // not an Exit, bubble up
	}
}

// handleRequest(w dns.ResponseWriter, r *dns.Msg) handles incoming DNS
// queries and returns an "A" record pointing to the loopback address.
// If the domain is not listed in the configuration, it returns NXDOMAIN.
func handleRequest(w dns.ResponseWriter, r *dns.Msg) {

	var found = false
	m := new(dns.Msg)
	m.SetReply(r)

	m.RecursionAvailable = r.RecursionDesired

	if r.Question[0].Qtype == dns.TypeA {
		for i := range parsedDomains {
			if dns.IsSubDomain(dns.Fqdn(parsedDomains[i]), dns.Fqdn(m.Question[0].Name)) {
				m.Answer = make([]dns.RR, 1)
				m.Authoritative = true

				m.Answer[0] = &dns.A{
					Hdr: dns.RR_Header{Name: m.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 120},
					A:   net.ParseIP("127.0.0.1"),
				}
				found = true
				break
			}
		}

	}
	if found == false {
		m.Rcode = dns.RcodeNameError
	}
	w.WriteMsg(m)
}

// setupDomains sets up the resolvers for each domain
// passed either on the command-line or from a config file.
func setupDomains() []string {

	var myDomains []string

	nsTemplate := []byte(fileSig + "\nnameserver 127.0.0.1\n")

	// Don't care if it already exists.
	// If root can't make the directory, we have much bigger problems.
	_ = os.Mkdir(targetDir, 0755)

	// Check for domains supplied on the command-line.
	if *inDomains != "" {
		myDomains = strings.Split(*inDomains, ",")
	} else {
		// Or try to read from the config file.
		content, err := ioutil.ReadFile(*inConfig)
		if err != nil {
			fmt.Printf("\nUnable to read config file (%s)\n and no domains supplied on command-line ", *inConfig)
			panic(panicExit{1})
		}
		myDomains = strings.Split(string(content), "\n")
	}

	// Setup each domain in the resolver directory.
	for i := range myDomains {
		fmt.Printf("Creating resolver for (%s)\n", myDomains[i])
		err := ioutil.WriteFile(targetDir+myDomains[i], nsTemplate, 0644)
		if err != nil {
			panic(err)
		}
	}
	return myDomains
}

// cleanupDomains iterates through the resover directry, looking for files
// with our signature (fileSig) and removing them.
func cleanupDomains() {
	// Look for our files in the resolver directory
	fmt.Println("Cleaning up")
	files, err := ioutil.ReadDir(targetDir)
	if err != nil {
		panic(err)
	}

	for _, f := range files {
		if f.IsDir() == false {
			content, err := ioutil.ReadFile(targetDir + f.Name())
			if err != nil {
				panic(err)
			}
			// Check if it's one of ours
			if strings.HasPrefix(string(content), fileSig) {
				fmt.Printf("Removing file: (%s)\n", targetDir+f.Name())
				err := os.Remove(targetDir + f.Name())
				if err != nil {
					panic(err)
				}
			} else {
				fmt.Printf("Skipping file: (%s)", f.Name())
			}
		}
	}
}
