# Go-AXL Tool

This tool is a low-level vehicle to transmit arbitrary AXL requests to Cisco Unified Communications Manager (CUCM). The user needs to understand AXL and write the XML requests themselves. Understanding the [AXL documentation](https://developer.cisco.com/docs/axl/) and translating it into valid XML is necessary. The tool is 'self-contained' and has no external runtime dependencies. It can be compiled for Linux/Windows/Mac OS etc. The tool can be used in different operating modes.

## Usage and Operating Modes

A few things to point out:
* if you want to ignore the TLS certificate, use the `-k` option
* default AXL schema is 12.5, it can be changed with `-s 10.0` for example
* the AXL method is derived from the top-level XML element (`addRoutePartition`) in this example

There are three operating modes available:

1. **Single AXL request** - write an XML file containing the AXL request and send it to CUCM:

```xml
addRP.xml:

<addRoutePartition>
  <routePartition>
    <name>AXL_PT</name>
  </routePartition>
</addRoutePartition>
```

```bash
./axl -cucm 10.10.20.1 -u axladmin -p cisco123 -xml addRP.xml
```

As a variation, you can also issue a get request and print the formatted XML output to the console:
```xml
smart.xml:

<getSmartLicenseStatus></getSmartLicenseStatus>
```

```bash
./axl -cucm 10.10.20.1 -u axladmin -p cisco123 -xml smart.xml -pp
```

2. **Bulk requests** - use a CSV file to send multiple AXL requests. Inside the XML file you refer to individual CSV columns using the `{{var n}} syntax` (n refers to the CSV column, starting at 0):

```xml
addRPs.xml:

<addRoutePartition>
  <routePartition>
    <name>{{var 0 }}</name>
    <description>{{var 1}}</description>
  </routePartition>
</addRoutePartition>
```

```csv
routepartitions.csv:

PT_One,First Partition
PT_Two,Second Partition
PT_Three,Third Partition
```

```bash
./axl -cucm 10.10.20.1 -u axladmin -p cisco123 -xml addRPs.xml -csv routepartitions.csv
```

3. **SQL Query export** - you can save SQL query results as a CSV file, which can be used as input for additional requests:

```xml
axlsql.xml:

<executeSQLQuery>
  <sql>SELECT n.dnorpattern, n.description, rp.name AS routepartition FROM numplan n LEFT JOIN routepartition rp ON rp.pkid=n.fkroutepartition</sql>
</executeSQLQuery>
```

```bash
./axl -cucm 10.10.20.1 -u axladmin -p cisco123 -xml axlsql.xml -savesql result.csv
```

## Build instructions

In order to successfully build the tool yourself you need to have the [Go tool](https://golang.org/dl/) installed. Note that Go does not need to be installed to just run the compiled binary. After installing Go (and git), you can follow these steps on the command line / in the terminal:

1. Clone the repo

This will download the project into a new subdirectory.

```bash
git clone https://github.com/sy9/go-axl.git
```

2. Build

```bash
cd go-axl
go build ./cmd/axl
```

You should have the compiled binary in your current directory which can be used directly (try `./axl -v`).

## Required parameters

* `-u` username of user with AXL permissions
* `-p` password for this user
* `-cucm` IP address or FQDN of CUCM publisher / first node
* `-xml` filename of XML file which includes AXL request

## Optional parameters

* `-pp` pretty-print successful XML response
* `-dump` dump HTTP request and response (incl. full body)
* `-csv <filename>` run in bulk mode using specified CSV file
* `-savesql <filename>` save executeSQLQueryResponse data as CSV in specified file
* `-k` ignore TLS certificate
* `-s <schema-version>` AXL schema version (default is 12.5)

## Save SQL response as CSV

If you execute an `executeSQLQuery` AXL request you can use the `-savesql myfile.csv` command to save the response in CSV format. The first line will be a comment (starting with `#`) documenting the column names. You can use this CSV file for later AXL requests if needed.

## Common error codes

If something goes wrong you should see an error code explaining the reason. Here are some common ones sent by CUCM:

* 401 - Username/Password incorrect
* 403 - Username/Password okay, however insufficient permissions (make sure user has AXL permissions)
* 599 - Schema not supported (default schema used is `12.5`, change with `-s` option)

These are HTTP error codes. If one of these errors is seen, or any network error in general, bulk operation stops. The tools continues operation in case of any AXL errors (e.g. "Duplicate value in UNIQUE INDEX" etc.).

## Logging (Bulk mode only)

In bulk mode (using a CSV file), the tool logs one line per CSV line to stdout per default. The content of the first column is included in the log line per default. To include other CSV columns, use the `{{varlog n}}` syntax, where n refers to the CSV column (again starting as 0). Each CSV column mentioned by `varlog` will be included in the log output. 

## XML & Encoding

The rules are as follows:

* all files (XML & CSV) should be UTF-8 encoded, no BOM
* XML file should use correct XML escaping (e.g. replace `&` with `&amp;`)

CSV file specifics:
* CSV file **does not need** any XML escaping (tool takes care of this)
* delimiter is comma (`,`)
* lines with hash (`#`) at the very beginning are ignored
* line ending in Windows or Unix format is supported
* if you happen to import an element with `#` at the beginning (e.g. route pattern in first column) - escape the value with `"` - example: `"#00345599!"`
* escape values with `"` if they happen to contain the delimiter character
* use two double quotes in escaped values (e.g. John "Hero" Smith --> `"John ""Hero"" Smith"`)

## Limitations

* `-savesql` assumes that CUCM returns each row with every column (even empty ones) and in the same order for every record. Our testing so far confirmed this behavior.
* the tool does not act upon rate limit errors from CUCM (e.g. no retry strategy)
* large SQL dumps are not automatically broken down into multiple sub-queries

## Debugging

To dump the requests and responses to stdout, use `-dump`. Note that the complete body will also be printed.
