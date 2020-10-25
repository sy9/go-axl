package main

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/sy9/axl/axl"
)

const Version = "0.1.1"

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
	pp      = flag.Bool("pp", false, "pretty print successful XML response body")
	sqlfile = flag.String("savesql", "", "save executeSQLQueryResponse as CSV into file")
)

func readCSV(filename string) [][]string {
	var records [][]string

	f, err := os.Open(filename)
	if err != nil {
		log.Fatalf("error opening CSV file: %v", err)
	}

	defer f.Close()

	reader := csv.NewReader(f)
	reader.Comment = '#'
	records, err = reader.ReadAll()
	if err != nil {
		log.Fatalf("error reading CSV file: %v", err)
	}

	return records
}

func decodeRow(body []byte, getHeader bool) []string {
	dec := xml.NewDecoder(bytes.NewReader(body))

	var (
		cd        string
		output    []string
		startSeen bool
	)

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return output
			}
			log.Fatalf("error decoding token: %v", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			startSeen = true
			if getHeader {
				output = append(output, tok.Name.Local)
			}
		case xml.CharData:
			if startSeen {
				cd = string(tok.Copy())
			}
		case xml.EndElement:
			if startSeen && !getHeader {
				output = append(output, cd)
				cd = ""
				startSeen = false
			}
		default:
			startSeen = false
		}
	}
}

func saveSQLResponse(filename string, body []byte) {
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal("unable to create file: %v", err)
	}
	enc := csv.NewWriter(f)
	type (
		Row struct {
			Data []byte `xml:",innerxml"`
		}

		Env struct {
			Return []Row `xml:"return>row"`
		}
	)
	var (
		env           Env
		headerWritten bool
	)

	if err := xml.NewDecoder(bytes.NewReader(body)).Decode(&env); err != nil {
		log.Fatalf("error decoding: %v", err)
	}
	var records [][]string
	for _, row := range env.Return {
		if !headerWritten {
			records = append(records, decodeRow(row.Data, true))
			headerWritten = true
		}
		records = append(records, decodeRow(row.Data, false))
	}
	// Comment out header line
	if records != nil && records[0] != nil && len(records[0][0]) > 0 {
		records[0][0] = strings.Join([]string{"#", records[0][0]}, "")
	}
	enc.WriteAll(records)
	enc.Flush()
	f.Close()
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

		resp, err := client.AXLRequest(r)
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
		if len(*sqlfile) > 0 {
			saveSQLResponse(*sqlfile, resp)
		}

		if *pp { // pretty-print XML output
			enc := xml.NewEncoder(os.Stdout)
			enc.Indent("", "  ")
			dec := xml.NewDecoder(bytes.NewReader(resp))
			for {
				tok, err := dec.Token()
				if err != nil {
					if err == io.EOF {
						break
					}
					log.Fatalf("error decoding XML response: %v", err)
				}
				if err := enc.EncodeToken(tok); err != nil {
					log.Fatalf("error encoding XML token: %v", err)
				}
			}
			if err := enc.Flush(); err != nil {
				log.Fatalf("error flushing: %v", err)
			}
			fmt.Println()
		}
		log.Printf("%s Result: %s", csvHandler.LogIndexAndItem(), result)
	}
}
