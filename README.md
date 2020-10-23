# AXL Tool

Issue AXL requests to Cisco Unified Communications Manager (CUCM).

Usage example:

```xml
addRP.xml:
<addRoutePartition>
  <routePartition>
    <name>AXL_PT</name>
  </routePartition>
</addRoutePartition>
```

`axl -cucm 10.10.20.1 -u axladmin -p cisco123 -xml addRP.xml`

A few things to point out:
* if you want to ignore the TLS certificate, use the `-k` option
* default AXL schema is 12.5, it can be changed with `-s 10.0` for example
* the AXL method is derived from the top-level XML element (`addRoutePartition`) in this example

By specifying a CSV file, you can run AXL requests in bulk. Usage example:

```xml
addRPbulk.xml:
<addRoutePartition>
  <routePartition>
    <name>{{var 0 }}</name>
    <description>{{var 1}}</description>
  </routePartition>
</addRoutePartition>
```

```csv
rp.csv:
PT_One,First Partition
PT_Two,Second Partition
PT_Three,Third Partition
```

`axl -cucm 10.10.20.1 -u axladmin -p cisco123 -xml addRPbulk.xml -csv rp.csv`

In bulk mode, one XML request is sent for each CSV line. The individual values from your CSV are inserted using the `{{var n}}` syntax, where n refers to the CSV column, starting at 0 for the first column. 

## Required parameters

* `-u` username of user with AXL permissions
* `-p` password for this user
* `-cucm` IP address or FQDN of CUCM publisher / first node
* `-xml` filename of XML file which includes AXL request

## Common error codes

If something goes wrong you should see an error code explaining the reason. Here are some common ones sent by CUCM:

* 401 - Username/Password incorrect
* 403 - Username/Password okay, however insufficient permissions (make sure user has AXL permissions)
* 599 - Schema not supported (default schema used is `12.5`, change with `-s` option)

These are HTTP error codes. If one of these errors is seen, bulk operation stops. The tools continues operation in case of any AXL errors (e.g. "Duplicate value in UNIQUE INDEX" etc.).

## Logging

In bulk mode (using a CSV file), the tool logs one line per CSV line to stdout per default. The content of the first column is included in the log line per default. To include other CSV columns, use the `{{varlog n}}` syntax, where n refers to the CSV column (again starting as 0). Each CSV column mentioned by `varlog` will be included in the log output. 
If you add the `log` parameter (e.g. `{{var 0 log}}`) the corresponding value will be added in the output log.

## Debugging

To dump the requests and responses to stdout, use `-dump`. Note that the complete body will also be printed.
