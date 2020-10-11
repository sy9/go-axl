package main

import (
	"log"
	"os"
	"strings"

	"github.com/sy9/axl/axl"
)

const axlMessage = `
<addRoutePartition>
  <name>PT_Internal</name>
  <description>Whatever</description>
  <empty/>
</addRoutePartition>
`
func main() {
	client := axl.NewClient("192.168.1.100:8443")
	    .SetAuthentication("axladmin", "cisco12345")
	    .SetSchemaVersion("12.5")
	    .SetInsecureSkipVerify(true)

	resp, err := client.AXLRequest(strings.NewReader(axlMessage))
	if err != nil {
		log.Fatalf("error from AXLRequest: %v", err)
	}
	io.Copy(os.Stdout, resp.Body())

	enc := axl.NewEncoder(os.Stdout)
	enc.SchemaVersion = "12.5"
	if err := enc.Encode(strings.NewReader(axlMessage)); err != nil {
		log.Fatalf("error encoding AXL: %v", err)
	}
}

