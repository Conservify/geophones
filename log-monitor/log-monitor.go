package main

import (
	"github.com/hpcloud/tail"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
)

type NetworkStatus struct {
	Lock     sync.Mutex
	Machines map[string]*Machine
}

type AdcStatus struct {
	LastUpdatedAt time.Time
	GpsLock       bool
}

type UploaderStatus struct {
	LastUpdatedAt time.Time
	Failed        bool
}

type MountPoint struct {
	MountPoint string
	Size       string
	Available  string
	Used       string
}

type MountPoints struct {
	LastUpdatedAt time.Time
	Mounts        map[string]*MountPoint
}

type MachineStatus struct {
	LastUpdatedAt time.Time
	Users         string
	Uptime        string
	LoadAverage   string
}

type BackupStatus struct {
	LastUpdatedAt time.Time
	Failed        bool
}

type Machine struct {
	Hostname      string
	LastMessageAt time.Time
	Status        MachineStatus
	Mounts        MountPoints
	LocalBackup   BackupStatus
	OffsiteBackup BackupStatus
	Adc           AdcStatus
	Uploader      UploaderStatus
}

type SysLogLine struct {
	StampRaw string
	Hostname string
	Facility string
	Message  string
}

type LogFileParser struct {
}

func (l *LogFileParser) Parse(line string) *SysLogLine {
	timeLength := len("Aug  4 21:46:55")
	parts := strings.SplitN(line[timeLength+1:], " ", 3)
	return &SysLogLine{
		StampRaw: line[0:timeLength],
		Hostname: parts[0],
		Facility: strings.TrimSpace(strings.Replace(parts[1], ":", "", -1)),
		Message:  strings.TrimSpace(parts[2]),
	}
}

// Aug  4 21:45:01 lodge local-backup: Local backup done (LOCAL_BACKUP:GOOD)

// Aug  4 21:48:59 lodge status:  21:48:59 up 18 min,  2 users,  load average: 0.00, 0.01, 0.00

var uptimeRe = regexp.MustCompile("([\\d:]+) up (.+),\\s+(\\d+) users,\\s+load average: (.+)")

func (l *LogFileParser) TryParseUptime(sl *SysLogLine, m *Machine) {
	if sl.Facility != "status" {
		return
	}

	v := uptimeRe.FindAllStringSubmatch(sl.Message, -1)
	if v == nil {
		return
	}

	m.Status = MachineStatus{
		LastUpdatedAt: time.Now(),
		Uptime:        v[0][1],
		Users:         v[0][2],
		LoadAverage:   v[0][3],
	}

	log.Printf("Uptime: %v", m.Status)
}

// Aug  5 00:13:03 glacier disk-space: Filesystem                Size      Used Available Use% Mounted on
// Aug  5 00:13:03 glacier disk-space: tmpfs                   831.0M     53.8M    777.2M   6% /
// Aug  5 00:13:03 glacier disk-space: tmpfs                   461.7M         0    461.7M   0% /dev/shm
// Aug  5 00:13:03 glacier disk-space: /dev/mmcblk0p2          234.5M     87.7M    130.4M  40% /mnt/mmcblk0p2
// Aug  5 00:13:03 glacier disk-space: /dev/sda1               343.7G     66.7M    326.1G   0% /backup

// Aug  5 00:13:03 lodge disk-space: Filesystem                Size      Used Available Use% Mounted on
// Aug  5 00:13:03 lodge disk-space: tmpfs                   831.0M     53.9M    777.1M   6% /
// Aug  5 00:13:03 lodge disk-space: tmpfs                   461.7M         0    461.7M   0% /dev/shm
// Aug  5 00:13:03 lodge disk-space: /dev/mmcblk0p2          234.5M     87.9M    130.3M  40% /mnt/mmcblk0p2
// Aug  5 00:13:03 lodge disk-space: /dev/sda1               343.7G      3.7G    322.5G   1% /backup

var spacesRe = regexp.MustCompile("\\s+")

func (l *LogFileParser) TryParseDisk(sl *SysLogLine, m *Machine) {
	if sl.Facility != "disk-space" {
		return
	}

	parts := spacesRe.Split(sl.Message, -1)

	mp := &MountPoint{
		MountPoint: parts[0],
		Size:       parts[1],
		Available:  parts[3],
		Used:       parts[4],
	}

	m.Mounts.LastUpdatedAt = time.Now()
	m.Mounts.Mounts[mp.MountPoint] = mp

	log.Printf("MountPoint: %v", mp)
}

var statusRe = regexp.MustCompile("\\((\\S+):(\\S+)\\)")

type SimpleInlineStatus struct {
	Which  string
	Status string
}

func ParseSimpleInlineStatus(text string) *SimpleInlineStatus {
	v := statusRe.FindAllStringSubmatch(text, -1)
	if v == nil {
		return nil
	}

	ss := SimpleInlineStatus{
		Which:  v[0][1],
		Status: v[0][2],
	}

	log.Printf("Flag: %v", ss)

	return &ss
}

func (l *LogFileParser) TryParseLocalBackup(sl *SysLogLine, m *Machine) {
	s := ParseSimpleInlineStatus(sl.Message)
	if s != nil {
		if s.Which == "LOCAL_BACKUP" {
			m.LocalBackup = BackupStatus{
				LastUpdatedAt: time.Now(),
				Failed:        s.Status == "FATAL",
			}
		}
	}
}

func (l *LogFileParser) TryParseOffsiteBackup(sl *SysLogLine, m *Machine) {
	s := ParseSimpleInlineStatus(sl.Message)
	if s != nil {
		if s.Which == "OFFSITE_BACKUP" {
			m.OffsiteBackup = BackupStatus{
				LastUpdatedAt: time.Now(),
				Failed:        s.Status == "FATAL",
			}
		}
	}
}

func (l *LogFileParser) TryParseAdc(sl *SysLogLine, m *Machine) {
	if sl.Facility == "adc" {
		m.Adc.LastUpdatedAt = time.Now()
	}
}

func sendStatus(ns *NetworkStatus) {
	for {
		ns.Lock.Lock()

		log.Printf("Hello")

		ns.Lock.Unlock()

		time.Sleep(5 * time.Second)
	}

}

func main() {
	t, err := tail.TailFile("/var/log/syslog", tail.Config{Follow: true})
	if err == nil {
		ns := NetworkStatus{
			Machines: make(map[string]*Machine),
		}

		go sendStatus(&ns)

		parser := &LogFileParser{}
		for line := range t.Lines {
			sl := parser.Parse(line.Text)

			ns.Lock.Lock()
			if ns.Machines[sl.Hostname] == nil {
				ns.Machines[sl.Hostname] = &Machine{
					Hostname: sl.Hostname,
					Mounts: MountPoints{
						Mounts: make(map[string]*MountPoint),
					},
					Status: MachineStatus{},
				}
			}

			m := ns.Machines[sl.Hostname]
			m.LastMessageAt = time.Now()

			parser.TryParseUptime(sl, m)
			parser.TryParseDisk(sl, m)
			parser.TryParseLocalBackup(sl, m)
			parser.TryParseOffsiteBackup(sl, m)
			parser.TryParseAdc(sl, m)

			ns.Lock.Unlock()
		}
	}
}
