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

	"github.com/logrusorgru/aurora"
	"github.com/miekg/dns"
)

var (
	inConfig      = flag.String("configfile", "crapdns.conf", "Use configuration file ( default: crapdns.conf)")
	inDomains     = flag.String("domains", "", "A comma-separated list of domains to answer for. (Disables config file).")
	parsedDomains []string
)

const (
	targetDir = "/etc/resolver/"
	fileSig   = "###CRAPDNS###"
)

type panicExit struct{ Code int }

func main() {
	defer handleExit()
	defer cleanupDomains()

	if runtime.GOOS != "darwin" {
		fmt.Println(aurora.Red("This utility is for Mac OS only."))
		panic(panicExit{2})
	}

	flag.Usage = func() {
		flag.PrintDefaults()
	}

	flag.Parse()

	parsedDomains = setupDomains()

	server := &dns.Server{Addr: "127.0.0.1:53", Net: "udp"}

	go func() {

		if err := server.ListenAndServe(); err != nil {
			fmt.Printf("Failed to setup the server: %s\n", aurora.Red(err.Error()))
			fmt.Println(aurora.Red("This command should be run as sudo."))
			os.Exit(1)
		}

	}()

	fmt.Println("\nStarting CrapDNS. ", aurora.Green("Listening on 127.0.0.1:53"))
	dns.HandleFunc(".", handleRequest)

	// Wait for the apocalypse
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	fmt.Println(aurora.Sprintf(aurora.Green("Signal (%s) received, exiting"), aurora.Red(s)))
}

func handleExit() {
	if e := recover(); e != nil {
		if exit, ok := e.(panicExit); ok == true {
			os.Exit(exit.Code)
		}
		panic(e) // not an Exit, bubble up
	}
}

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
			fmt.Println(aurora.Sprintf(aurora.Red("Unable to read config file (%s)\n and no domains supplied on command-line "), *inConfig))
			panic(panicExit{1})
		}
		myDomains = strings.Split(string(content), "\n")
	}

	// Setup each domain in the resolver directory.
	for i := range myDomains {
		fmt.Println("Creating resolver for ", aurora.Green(myDomains[i]))
		err := ioutil.WriteFile(targetDir+myDomains[i], nsTemplate, 0644)
		if err != nil {
			panic(err)
		}
	}
	return myDomains
}

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
				fmt.Println(aurora.Green("Removing file: "), aurora.Green(targetDir+f.Name()))
				err := os.Remove(targetDir + f.Name())
				if err != nil {
					panic(err)
				}
			} else {
				fmt.Println(aurora.Magenta("Skipping file "), aurora.Green(f.Name()))
			}
		}
	}
}
