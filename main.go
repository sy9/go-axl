package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/sy9/axl/axl"
)

const Version = "0.1.0"

var (
	cucm    = flag.String("cucm", "", "FQDN or IP of CUCM Publisher / First Node (required)")
	user    = flag.String("u", "", "AXL username (required)")
	pass    = flag.String("p", "", "AXL password (required)")
	insec   = flag.Bool("k", false, "Skip certificate validation")
	schema  = flag.String("s", "12.5", "AXL schema version")
	xmlfile = flag.String("xml", "", "XML filename for request input (required)")
	csvfile = flag.String("csv", "", "CSV filename for bulk requests")
	dump    = flag.Bool("dump", false, "dump request and response headers and body")
	ver     = flag.Bool("v", false, "show version and exit")
)

func readCSV(filename string) [][]string {
	var records [][]string

	f, err := os.Open(filename)
	if err != nil {
		log.Fatalf("error opening CSV file: %v", err)
	}

	defer f.Close()

	records, err = csv.NewReader(f).ReadAll()
	if err != nil {
		log.Fatalf("error reading CSV file: %v", err)
	}

	return records
}

func main() {
	flag.Parse()
	if *ver {
		fmt.Printf("AXL version: %s\n", Version)
		os.Exit(1)
	}

	if len(*cucm) == 0 || len(*xmlfile) == 0 || len(*user) == 0 || len(*pass) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	axlBody, err := ioutil.ReadFile(*xmlfile)
	if err != nil {
		log.Fatalf("error reading XML file: %v", err)
	}

	csvHandler, err := NewCSVHandler(axlBody)
	if err != nil {
		log.Fatalf("error initializing CSV handler: %v", err)
	}

	if len(*csvfile) > 0 {
		csvHandler.SetRecords(readCSV(*csvfile))
	}

	client := axl.NewClient(*cucm).
		SetAuthentication(*user, *pass).
		SetSchemaVersion(*schema).
		SetInsecureSkipVerify(*insec).
		SetRequestResponseDump(*dump)

	for csvHandler.Next() {
		r, w := io.Pipe()
		go func() {
			err := csvHandler.Do(w)
			if err != nil {
				log.Fatalf("error executing template: %v", err)
			}
			w.Close()
		}()

		_, err := client.AXLRequest(r)
		result := "success"
		if err != nil {
			//if err, ok := err.(*axl.AXLError); ok{
			var e *axl.AXLError
			if errors.As(err, &e) {
				result = fmt.Sprintf("%s (%d)", e.AXLErrorMessage, e.AXLErrorCode)
			} else {
				log.Fatalf("error from AXLRequest: %v", err)
			}
		}
		log.Printf("%s Result: %s", csvHandler.LogIndexAndItem(), result)
	}
}
