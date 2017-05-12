package main

import (
	"flag"
	"fmt"
	fastping "github.com/tatsushid/go-fastping"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

type response struct {
	addr *net.IPAddr
	rtt  time.Duration
}

func testNetworking(hostname string) (anySuccess bool, stopped bool) {
	p := fastping.NewPinger()
	ra, err := net.ResolveIPAddr("ip4:icmp", hostname)
	if err != nil {
		log.Fatal(err)
	}

	responses := make(map[string]*response)
	responses[ra.String()] = nil

	p.AddIPAddr(ra)

	onRecv, onIdle := make(chan *response), make(chan bool)
	p.OnRecv = func(addr *net.IPAddr, t time.Duration) {
		onRecv <- &response{addr: addr, rtt: t}
	}
	p.OnIdle = func() {
		onIdle <- true
	}

	p.MaxRTT = time.Second
	p.RunLoop()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	startAt := time.Now()
	running := true
	stopped = false
	for running && time.Now().Sub(startAt).Seconds() < 10.0 {
		select {
		case <-c:
			running = false
			stopped = true
		case res := <-onRecv:
			if _, ok := responses[res.addr.String()]; ok {
				responses[res.addr.String()] = res
			}
		case <-onIdle:
			for host, r := range responses {
				if r == nil {
					log.Printf("%s : unreachable %v\n", host, time.Now())
				} else {
					log.Printf("%s : %v %v\n", host, r.rtt, time.Now())
					anySuccess = true
					running = false
				}
				responses[host] = nil
			}
		case <-p.Done():
			if err = p.Err(); err != nil {
				fmt.Println("Ping failed:", err)
				os.Exit(2)
			}
		}
	}

	signal.Stop(c)

	// This hangs for me for some reason?
	// p.Stop()

	return
}

func main() {
	var dryRun bool
	flag.BoolVar(&dryRun, "dry", false, "dry run")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  %s [options] hostname [source]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	hostname := flag.Arg(0)
	if len(hostname) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	if dryRun {
		log.Printf("Dry run enabled.")
	}

	good, stopped := testNetworking(hostname)
	if !stopped && !good {
		log.Printf("Unreachable, restarting networking...")

		restartNetworking := "/etc/init.d/networking restart"
		log.Printf(restartNetworking)
		if !dryRun {
			exec.Command(restartNetworking)
		}

		good, stopped := testNetworking(hostname)
		if !stopped && !good {
			log.Printf("Unreachable, restarting computer...")

			restartComputer := "/sbin/reboot"
			log.Printf(restartComputer)
			if !dryRun {
				exec.Command(restartComputer)
			}
		}
	} else if good {
		log.Printf("Network is good.")
	}

}
