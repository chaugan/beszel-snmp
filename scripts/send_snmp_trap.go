package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	gosnmp "github.com/gosnmp/gosnmp"
)

func main() {
	target := flag.String("target", "127.0.0.1:9162", "trap receiver host:port")
	community := flag.String("community", "public", "SNMP community")
	sysname := flag.String("sysname", "lab-switch", "sysName.0 value")
	temp := flag.Int("temp", 26, "temperature value (integer)")
	trapOID := flag.String("trapOID", ".1.3.6.1.6.3.1.1.5.1", "snmpTrapOID value")
	valueOID := flag.String("oid", ".1.3.6.1.4.1.9.9.13.1.3.1.3.0", "value OID to send")
	flag.Parse()

	host, portStr, err := net.SplitHostPort(*target)
	if err != nil {
		log.Fatalf("invalid target %q: %v", *target, err)
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("invalid port in target %q: %v", *target, err)
	}

	params := &gosnmp.GoSNMP{
		Target:    host,
		Port:      uint16(p),
		Community: *community,
		Version:   gosnmp.Version2c,
		Timeout:   2 * time.Second,
		Retries:   0,
	}
	if err := params.Connect(); err != nil {
		log.Fatalf("connect failed: %v", err)
	}
	defer params.Conn.Close()

	vars := []gosnmp.SnmpPDU{
		// sysUpTime.0
		{Name: ".1.3.6.1.2.1.1.3.0", Type: gosnmp.TimeTicks, Value: uint32(1)},
		// snmpTrapOID.0
		{Name: ".1.3.6.1.6.3.1.1.4.1.0", Type: gosnmp.ObjectIdentifier, Value: *trapOID},
		// sysName.0
		{Name: ".1.3.6.1.2.1.1.5.0", Type: gosnmp.OctetString, Value: *sysname},
		// value OID (e.g., Cisco temperature)
		{Name: *valueOID, Type: gosnmp.Integer, Value: *temp},
	}

	trap := gosnmp.SnmpTrap{Variables: vars}
	if _, err := params.SendTrap(trap); err != nil {
		log.Fatalf("send trap failed: %v", err)
	}
	fmt.Println("Trap sent to", *target)
}
